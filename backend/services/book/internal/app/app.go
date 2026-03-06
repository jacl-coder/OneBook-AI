package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
	"onebookai/pkg/storage"
	"onebookai/pkg/store"
)

// Config holds runtime configuration for the core application.
type Config struct {
	DatabaseURL               string
	Store                     store.Store
	MinioEndpoint             string
	MinioAccessKey            string
	MinioSecretKey            string
	MinioBucket               string
	MinioUseSSL               bool
	IngestURL                 string
	InternalJWTKeyID          string
	InternalJWTPrivateKeyPath string
	MaxUploadBytes            int64
	AllowedExtensions         []string
	QdrantURL                 string
	QdrantAPIKey              string
	QdrantCollection          string
}

// App is the core application service wiring together storage and domain logic.
type App struct {
	store             store.Store
	objects           storage.ObjectStore
	ingest            ingestClient
	search            *retrieval.Client
	presignExpiry     time.Duration
	maxUploadBytes    int64
	allowedExtensions map[string]struct{}
}

// New constructs the application with database-backed metadata storage and filesystem file storage.
func New(cfg Config) (*App, error) {
	objStore, err := storage.NewMinioStore(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
	if err != nil {
		return nil, err
	}
	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required")
		}
		dataStore, err = store.NewGormStore(cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("init postgres store: %w", err)
		}
	}

	if cfg.IngestURL == "" {
		return nil, fmt.Errorf("ingest URL required")
	}
	signer, err := servicetoken.NewSignerWithOptions(servicetoken.SignerOptions{
		PrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		KeyID:          cfg.InternalJWTKeyID,
		Issuer:         "book-service",
		TTL:            servicetoken.DefaultTokenTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("init internal signer: %w", err)
	}

	ingestClient, err := newIngestClient(cfg.IngestURL, signer)
	if err != nil {
		return nil, fmt.Errorf("init ingest client: %w", err)
	}
	searchClient, err := retrieval.NewQdrantClient(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, 1)
	if err != nil {
		return nil, fmt.Errorf("init qdrant client: %w", err)
	}

	app := &App{
		store:             dataStore,
		objects:           objStore,
		ingest:            ingestClient,
		search:            searchClient,
		presignExpiry:     15 * time.Minute,
		maxUploadBytes:    normalizeMaxBytes(cfg.MaxUploadBytes),
		allowedExtensions: normalizeExtensions(cfg.AllowedExtensions),
	}
	app.startCleanupWorker()
	return app, nil
}

// UploadBook stores a new book file and enqueues simulated processing.
func (a *App) UploadBook(owner domain.User, filename string, r io.Reader, size int64, primaryCategory string, tags []string, idempotencyKey string) (domain.Book, bool, error) {
	if filename == "" {
		return domain.Book{}, false, errors.New("filename required")
	}
	if !a.isExtensionAllowed(filename) {
		return domain.Book{}, false, fmt.Errorf("unsupported file type")
	}
	if a.maxUploadBytes > 0 && size > 0 && size > a.maxUploadBytes {
		return domain.Book{}, false, fmt.Errorf("file too large")
	}
	normalizedCategory, err := normalizePrimaryCategory(primaryCategory)
	if err != nil {
		return domain.Book{}, false, err
	}
	normalizedTags, err := normalizeBookTags(tags)
	if err != nil {
		return domain.Book{}, false, err
	}
	requestHash := uploadRequestHash(owner.ID, filename, size, normalizedCategory, normalizedTags)
	record, replayBook, replayed, err := a.beginBookIdempotency(idempotencyScopeUpload, owner.ID, idempotencyKey, requestHash)
	if err != nil {
		return domain.Book{}, false, err
	}
	if replayed {
		return replayBook, true, nil
	}
	id := util.NewID()
	storageKey := buildStorageKey(id, filename)
	book := domain.Book{
		ID:               id,
		OwnerID:          owner.ID,
		Title:            titleFromName(filename),
		OriginalFilename: filepath.Base(filename),
		PrimaryCategory:  normalizedCategory,
		Tags:             normalizedTags,
		Format:           detectBookFormat(filename),
		Language:         string(domain.BookLanguageUnknown),
		StorageKey:       storageKey,
		Status:           domain.StatusQueued,
		SizeBytes:        size,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := a.objects.Put(context.Background(), storageKey, r, size, contentType); err != nil {
		_ = a.markBookIdempotencyFailed(record, httpStatusFromErr(err))
		return domain.Book{}, false, fmt.Errorf("save file: %w", err)
	}
	if err := a.store.SaveBook(book); err != nil {
		_ = a.objects.Delete(context.Background(), storageKey)
		_ = a.markBookIdempotencyFailed(record, httpStatusFromErr(err))
		return domain.Book{}, false, fmt.Errorf("save book: %w", err)
	}
	if err := a.ingest.Enqueue(id); err != nil {
		_ = a.store.SetStatus(id, domain.StatusFailed, err.Error())
		_ = a.markBookIdempotencyFailed(record, httpStatusFromErr(err))
		return domain.Book{}, false, fmt.Errorf("enqueue ingest: %w", err)
	}
	_ = a.completeBookIdempotency(record, "book", book.ID, 201)
	return book, false, nil
}

// ListBooks returns all books for the current user scope.
func (a *App) ListBooks(user domain.User, opts store.BookListOptions) ([]domain.Book, error) {
	opts.Query = strings.TrimSpace(opts.Query)
	opts.Status = strings.TrimSpace(strings.ToLower(opts.Status))
	opts.PrimaryCategory = strings.TrimSpace(strings.ToLower(opts.PrimaryCategory))
	opts.Tag = strings.TrimSpace(opts.Tag)
	opts.Format = strings.TrimSpace(strings.ToLower(opts.Format))
	opts.Language = strings.TrimSpace(strings.ToLower(opts.Language))
	if user.Role != domain.RoleAdmin {
		opts.OwnerID = user.ID
	}
	items, _, err := a.store.ListBooksWithOptions(opts)
	return items, err
}

// GetBook retrieves a book by ID.
func (a *App) GetBook(id string) (domain.Book, bool, error) {
	return a.store.GetBook(id)
}

func (a *App) GetBookIncludingDeleted(id string) (domain.Book, bool, error) {
	return a.store.GetBookIncludingDeleted(id)
}

func (a *App) UpdateBook(id string, title, primaryCategory string, tags []string) (domain.Book, error) {
	book, ok, err := a.store.GetBook(id)
	if err != nil {
		return domain.Book{}, err
	}
	if !ok {
		return domain.Book{}, fmt.Errorf("book not found")
	}
	normalizedTitle, err := normalizeBookTitle(title)
	if err != nil {
		return domain.Book{}, err
	}
	normalizedCategory, err := normalizePrimaryCategory(primaryCategory)
	if err != nil {
		return domain.Book{}, err
	}
	normalizedTags, err := normalizeBookTags(tags)
	if err != nil {
		return domain.Book{}, err
	}
	book.Title = normalizedTitle
	book.PrimaryCategory = normalizedCategory
	book.Tags = normalizedTags
	book.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveBook(book); err != nil {
		return domain.Book{}, fmt.Errorf("save book: %w", err)
	}
	return book, nil
}

// GetDownloadURL returns a pre-signed URL and original filename.
func (a *App) GetDownloadURL(id string) (string, string, error) {
	book, ok, err := a.store.GetBook(id)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("book not found")
	}
	if strings.TrimSpace(book.StorageKey) == "" {
		return "", "", fmt.Errorf("storage key missing")
	}
	url, err := a.objects.PresignGet(context.Background(), book.StorageKey, a.presignExpiry, book.OriginalFilename)
	if err != nil {
		return "", "", err
	}
	return url, book.OriginalFilename, nil
}

// UpdateStatus updates book status and error message.
func (a *App) UpdateStatus(id string, status domain.BookStatus, errMsg string) error {
	return a.store.SetStatus(id, status, errMsg)
}

// DeleteBook removes book metadata and files.
func (a *App) DeleteBook(id string) error {
	book, ok, err := a.store.GetBookIncludingDeleted(id)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if book.DeletedAt != nil {
		if domain.BookCleanupStatus(book.CleanupStatus) == domain.BookCleanupStatusFailed {
			return a.store.UpdateBookCleanup(id, domain.BookCleanupStatusQueued, "", false)
		}
		return nil
	}
	return a.store.MarkBookDeleted(id, domain.BookCleanupStatusQueued)
}

// ReprocessBook re-enqueues ingest for an existing book and resets status to queued.
func (a *App) ReprocessBook(actor domain.User, id, idempotencyKey string) (domain.Book, bool, error) {
	_, ok, err := a.store.GetBook(id)
	if err != nil {
		return domain.Book{}, false, err
	}
	if !ok {
		return domain.Book{}, false, fmt.Errorf("book not found")
	}
	requestHash := util.HashStrings(id, "{}")
	record, replayBook, replayed, err := a.beginBookIdempotency(idempotencyScopeReprocess, actor.ID, idempotencyKey, requestHash)
	if err != nil {
		return domain.Book{}, false, err
	}
	if replayed {
		return replayBook, true, nil
	}
	current, ok, err := a.store.GetBook(id)
	if err != nil {
		return domain.Book{}, false, err
	}
	if !ok {
		return domain.Book{}, false, fmt.Errorf("book not found")
	}
	if current.Status == domain.StatusQueued || current.Status == domain.StatusProcessing {
		_ = a.completeBookIdempotency(record, "book", current.ID, 200)
		return current, false, nil
	}
	if err := a.store.SetStatus(id, domain.StatusQueued, ""); err != nil {
		_ = a.markBookIdempotencyFailed(record, httpStatusFromErr(err))
		return domain.Book{}, false, err
	}
	if err := a.ingest.Enqueue(id); err != nil {
		_ = a.store.SetStatus(id, domain.StatusFailed, err.Error())
		_ = a.markBookIdempotencyFailed(record, httpStatusFromErr(err))
		return domain.Book{}, false, fmt.Errorf("enqueue ingest: %w", err)
	}
	updated, ok, err := a.store.GetBook(id)
	if err != nil {
		return domain.Book{}, false, err
	}
	if !ok {
		return domain.Book{}, false, fmt.Errorf("book not found")
	}
	_ = a.completeBookIdempotency(record, "book", updated.ID, 200)
	return updated, false, nil
}

func titleFromName(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	title := strings.TrimSuffix(base, ext)
	title = strings.TrimSpace(title)
	if title == "" {
		return "未命名书籍"
	}
	return title
}

func buildStorageKey(bookID, filename string) string {
	name := sanitizeFilename(filepath.Base(filename))
	if name == "" {
		name = "book"
	}
	return path.Join("books", bookID, name)
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	lastUnderscore := false
	for _, r := range name {
		if r <= 0x7f {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
				b.WriteRune(r)
				lastUnderscore = false
				continue
			}
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func normalizeMaxBytes(value int64) int64 {
	if value <= 0 {
		return 50 * 1024 * 1024
	}
	return value
}

func normalizeExtensions(exts []string) map[string]struct{} {
	if len(exts) == 0 {
		exts = []string{".pdf", ".epub", ".txt"}
	}
	out := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		out[ext] = struct{}{}
	}
	return out
}

func (a *App) isExtensionAllowed(filename string) bool {
	if len(a.allowedExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := a.allowedExtensions[ext]
	return ok
}
