package store

import "onebookai/pkg/domain"

// Store defines persistence operations for users, books, and messages.
type Store interface {
	// users
	SaveUser(domain.User) error
	HasUserEmail(email string) (bool, error)
	GetUserByEmail(email string) (domain.User, bool, error)
	GetUserByID(id string) (domain.User, bool, error)
	ListUsers() ([]domain.User, error)
	UserCount() (int, error)

	// books
	SaveBook(domain.Book) error
	SetStatus(id string, status domain.BookStatus, errMsg string) error
	ListBooks() ([]domain.Book, error)
	ListBooksByOwner(ownerID string) ([]domain.Book, error)
	GetBook(id string) (domain.Book, bool, error)
	DeleteBook(id string) error

	// chats
	AppendMessage(bookID string, msg domain.Message) error
	ListMessages(bookID string, limit int) ([]domain.Message, error)

	// chunks
	ReplaceChunks(bookID string, chunks []domain.Chunk) error
	ListChunksByBook(bookID string) ([]domain.Chunk, error)
	SetChunkEmbedding(id string, embedding []float32) error
	SearchChunks(bookID string, embedding []float32, limit int) ([]domain.Chunk, error)
}

// SessionStore persists session tokens.
type SessionStore interface {
	NewSession(userID string) (string, error)
	GetUserIDByToken(token string) (string, bool, error)
	DeleteSession(token string) error
}
