package app

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/auth"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/storage"
	"onebookai/services/gateway/internal/store"
)

// Config holds runtime configuration for the core application.
type Config struct {
	StorageDir    string
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	SessionTTL    time.Duration
	JWTSecret     string
	Store         store.Store
	Sessions      store.SessionStore
}

// App is the core application service wiring together storage and domain logic.
type App struct {
	store    store.Store
	sessions store.SessionStore
	files    *storage.FileStore
}

// New constructs the application with in-memory metadata storage and filesystem file storage.
func New(cfg Config) (*App, error) {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}

	fileStore, err := storage.NewFileStore(cfg.StorageDir)
	if err != nil {
		return nil, err
	}

	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required (no in-memory store allowed)")
		}
		dataStore, err = store.NewGormStore(cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("init postgres store: %w", err)
		}
	}

	sessionStore := cfg.Sessions
	if sessionStore == nil {
		switch {
		case cfg.JWTSecret != "":
			sessionStore = store.NewJWTSessionStore(cfg.JWTSecret, cfg.SessionTTL)
		case cfg.RedisAddr != "":
			sessionStore = store.NewRedisSessionStore(cfg.RedisAddr, cfg.RedisPassword, cfg.SessionTTL)
		default:
			return nil, fmt.Errorf("session store required (jwtSecret or redisAddr)")
		}
	}

	return &App{
		store:    dataStore,
		sessions: sessionStore,
		files:    fileStore,
	}, nil
}

// SignUp registers a new user with default role user.
func (a *App) SignUp(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", errors.New("email and password required")
	}
	exists, err := a.store.HasUserEmail(email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("check email: %w", err)
	}
	if exists {
		return domain.User{}, "", fmt.Errorf("email already exists")
	}
	role := domain.RoleUser
	count, err := a.store.UserCount()
	if err != nil {
		return domain.User{}, "", fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		role = domain.RoleAdmin
	}
	user := domain.User{
		ID:           util.NewID(),
		Email:        email,
		PasswordHash: auth.HashPassword(password),
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}
	if err := a.store.SaveUser(user); err != nil {
		return domain.User{}, "", fmt.Errorf("save user: %w", err)
	}
	token, err := a.sessions.NewSession(user.ID)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue session: %w", err)
	}
	return user, token, nil
}

// Login validates credentials and issues a session token.
func (a *App) Login(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, ok, err := a.store.GetUserByEmail(email)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("fetch user: %w", err)
	}
	if !ok {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	token, err := a.sessions.NewSession(user.ID)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("issue session: %w", err)
	}
	return user, token, nil
}

// UserFromToken resolves a user from a session token.
func (a *App) UserFromToken(token string) (domain.User, bool) {
	uid, ok, err := a.sessions.GetUserIDByToken(token)
	if err != nil || !ok {
		return domain.User{}, false
	}
	user, found, err := a.store.GetUserByID(uid)
	if err != nil || !found {
		return domain.User{}, false
	}
	return user, true
}

// Logout removes a session token.
func (a *App) Logout(token string) error {
	return a.sessions.DeleteSession(token)
}

// ListUsers returns all users (admin use only).
func (a *App) ListUsers() ([]domain.User, error) {
	return a.store.ListUsers()
}

// UploadBook stores a new book file and enqueues simulated processing.
func (a *App) UploadBook(owner domain.User, filename string, r io.Reader, size int64) (domain.Book, error) {
	if filename == "" {
		return domain.Book{}, errors.New("filename required")
	}
	id := util.NewID()
	book := domain.Book{
		ID:               id,
		OwnerID:          owner.ID,
		Title:            titleFromName(filename),
		OriginalFilename: filepath.Base(filename),
		Status:           domain.StatusQueued,
		SizeBytes:        size,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := a.files.Save(id, filename, r); err != nil {
		return domain.Book{}, fmt.Errorf("save file: %w", err)
	}
	if err := a.store.SaveBook(book); err != nil {
		return domain.Book{}, fmt.Errorf("save book: %w", err)
	}
	a.simulateProcessing(id)
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

// DeleteBook removes book metadata and files.
func (a *App) DeleteBook(id string) error {
	if err := a.store.DeleteBook(id); err != nil {
		return err
	}
	return a.files.Delete(id)
}

// AskQuestion performs a placeholder question/answer flow bound to a book.
func (a *App) AskQuestion(user domain.User, bookID, question string) (domain.Answer, error) {
	book, ok, err := a.store.GetBook(bookID)
	if err != nil {
		return domain.Answer{}, fmt.Errorf("get book: %w", err)
	}
	if !ok {
		return domain.Answer{}, fmt.Errorf("book not found")
	}
	if book.OwnerID != user.ID && user.Role != domain.RoleAdmin {
		return domain.Answer{}, fmt.Errorf("forbidden")
	}
	if book.Status != domain.StatusReady {
		return domain.Answer{}, ErrBookNotReady
	}
	if strings.TrimSpace(question) == "" {
		return domain.Answer{}, fmt.Errorf("question required")
	}

	answer := domain.Answer{
		BookID:   bookID,
		Question: question,
		Answer:   fmt.Sprintf("占位回答：基于《%s》的示例回复。后续接入检索与模型生成。", book.Title),
		Sources: []domain.Source{
			{
				Label:    "示例出处",
				Location: "chapter/section",
				Snippet:  "这里展示未来会返回的章节/页码摘要。",
			},
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := a.store.AppendMessage(bookID, domain.Message{
		ID:        util.NewID(),
		BookID:    bookID,
		Role:      "user",
		Content:   question,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return domain.Answer{}, fmt.Errorf("save user message: %w", err)
	}
	if err := a.store.AppendMessage(bookID, domain.Message{
		ID:        util.NewID(),
		BookID:    bookID,
		Role:      "assistant",
		Content:   answer.Answer,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return domain.Answer{}, fmt.Errorf("save answer message: %w", err)
	}
	return answer, nil
}

// simulateProcessing fakes asynchronous processing pipeline.
func (a *App) simulateProcessing(id string) {
	go func() {
		if err := a.store.SetStatus(id, domain.StatusProcessing, ""); err != nil {
			slog.Error("failed to update status to processing", "book_id", id, "err", err)
		}
		time.Sleep(500 * time.Millisecond)
		if err := a.store.SetStatus(id, domain.StatusReady, ""); err != nil {
			slog.Error("failed to update status to ready", "book_id", id, "err", err)
		}
	}()
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
