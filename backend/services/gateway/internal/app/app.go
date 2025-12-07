package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"onebookai/pkg/auth"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/storage"
	"onebookai/services/gateway/internal/store"
)

// Config holds runtime configuration for the core application.
type Config struct {
	StorageDir string
}

// App is the core application service wiring together storage and domain logic.
type App struct {
	store *store.MemoryStore
	files *storage.FileStore
}

// New constructs the application with in-memory metadata storage and filesystem file storage.
func New(cfg Config) (*App, error) {
	fileStore, err := storage.NewFileStore(cfg.StorageDir)
	if err != nil {
		return nil, err
	}
	return &App{
		store: store.NewMemoryStore(),
		files: fileStore,
	}, nil
}

// SignUp registers a new user with default role user.
func (a *App) SignUp(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return domain.User{}, "", errors.New("email and password required")
	}
	if a.store.HasUserEmail(email) {
		return domain.User{}, "", fmt.Errorf("email already exists")
	}
	role := domain.RoleUser
	if a.store.UserCount() == 0 {
		role = domain.RoleAdmin
	}
	user := domain.User{
		ID:           store.NewID(),
		Email:        email,
		PasswordHash: auth.HashPassword(password),
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}
	a.store.SaveUser(user)
	token := a.store.NewSession(user.ID)
	return user, token, nil
}

// Login validates credentials and issues a session token.
func (a *App) Login(email, password string) (domain.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, ok := a.store.GetUserByEmail(email)
	if !ok {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return domain.User{}, "", fmt.Errorf("invalid credentials")
	}
	token := a.store.NewSession(user.ID)
	return user, token, nil
}

// UserFromToken resolves a user from a session token.
func (a *App) UserFromToken(token string) (domain.User, bool) {
	return a.store.GetUserByToken(token)
}

// ListUsers returns all users (admin use only).
func (a *App) ListUsers() []domain.User {
	return a.store.ListUsers()
}

// UploadBook stores a new book file and enqueues simulated processing.
func (a *App) UploadBook(owner domain.User, filename string, r io.Reader, size int64) (domain.Book, error) {
	if filename == "" {
		return domain.Book{}, errors.New("filename required")
	}
	id := store.NewID()
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
	a.store.SaveBook(book)
	a.simulateProcessing(id)
	return book, nil
}

// ListBooks returns all books for the current user scope.
func (a *App) ListBooks(user domain.User) []domain.Book {
	if user.Role == domain.RoleAdmin {
		return a.store.ListBooks()
	}
	return a.store.ListBooksByOwner(user.ID)
}

// GetBook retrieves a book by ID.
func (a *App) GetBook(id string) (domain.Book, bool) {
	return a.store.GetBook(id)
}

// DeleteBook removes book metadata and files.
func (a *App) DeleteBook(id string) error {
	a.store.DeleteBook(id)
	return a.files.Delete(id)
}

// AskQuestion performs a placeholder question/answer flow bound to a book.
func (a *App) AskQuestion(user domain.User, bookID, question string) (domain.Answer, error) {
	book, ok := a.store.GetBook(bookID)
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
	a.store.AppendMessage(bookID, domain.Message{
		ID:        store.NewID(),
		BookID:    bookID,
		Role:      "user",
		Content:   question,
		CreatedAt: time.Now().UTC(),
	})
	a.store.AppendMessage(bookID, domain.Message{
		ID:        store.NewID(),
		BookID:    bookID,
		Role:      "assistant",
		Content:   answer.Answer,
		CreatedAt: time.Now().UTC(),
	})
	return answer, nil
}

// simulateProcessing fakes asynchronous processing pipeline.
func (a *App) simulateProcessing(id string) {
	go func() {
		a.store.SetStatus(id, domain.StatusProcessing, "")
		time.Sleep(500 * time.Millisecond)
		a.store.SetStatus(id, domain.StatusReady, "")
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
