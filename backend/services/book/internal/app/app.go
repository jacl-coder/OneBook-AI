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

	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/storage"
	"onebookai/pkg/store"
)

// Config holds runtime configuration for the core application.
type Config struct {
	DatabaseURL    string
	Store          store.Store
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	IngestURL      string
	InternalToken  string
}

// App is the core application service wiring together storage and domain logic.
type App struct {
	store         store.Store
	objects       storage.ObjectStore
	ingest        ingestClient
	presignExpiry time.Duration
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
	if cfg.InternalToken == "" {
		return nil, fmt.Errorf("internal token required")
	}

	return &App{
		store:         dataStore,
		objects:       objStore,
		ingest:        newIngestClient(cfg.IngestURL, cfg.InternalToken),
		presignExpiry: 15 * time.Minute,
	}, nil
}

// UploadBook stores a new book file and enqueues simulated processing.
func (a *App) UploadBook(owner domain.User, filename string, r io.Reader, size int64) (domain.Book, error) {
	if filename == "" {
		return domain.Book{}, errors.New("filename required")
	}
	id := util.NewID()
	storageKey := buildStorageKey(id, filename)
	book := domain.Book{
		ID:               id,
		OwnerID:          owner.ID,
		Title:            titleFromName(filename),
		OriginalFilename: filepath.Base(filename),
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
		return domain.Book{}, fmt.Errorf("save file: %w", err)
	}
	if err := a.store.SaveBook(book); err != nil {
		_ = a.objects.Delete(context.Background(), storageKey)
		return domain.Book{}, fmt.Errorf("save book: %w", err)
	}
	if err := a.ingest.Enqueue(id); err != nil {
		_ = a.store.SetStatus(id, domain.StatusFailed, err.Error())
		return domain.Book{}, fmt.Errorf("enqueue ingest: %w", err)
	}
	return book, nil
}

// ListBooks returns all books for the current user scope.
func (a *App) ListBooks(user domain.User) ([]domain.Book, error) {
	if user.Role == domain.RoleAdmin {
		return a.store.ListBooks()
	}
	return a.store.ListBooksByOwner(user.ID)
}

// GetBook retrieves a book by ID.
func (a *App) GetBook(id string) (domain.Book, bool, error) {
	return a.store.GetBook(id)
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
	url, err := a.objects.PresignGet(context.Background(), book.StorageKey, a.presignExpiry)
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
	book, ok, err := a.store.GetBook(id)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := a.store.DeleteBook(id); err != nil {
		return err
	}
	if err := a.objects.Delete(context.Background(), book.StorageKey); err != nil {
		return err
	}
	return nil
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
