package store

import (
	"time"

	"onebookai/pkg/domain"
)

type UserListOptions struct {
	Query     string
	Role      string
	Status    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

type BookListOptions struct {
	Query     string
	OwnerID   string
	Status    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

type AdminAuditLogListOptions struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	From       time.Time
	To         time.Time
	Page       int
	PageSize   int
}

// Store defines persistence operations for users, books, and messages.
type Store interface {
	// users
	SaveUser(domain.User) error
	HasUserEmail(email string) (bool, error)
	GetUserByEmail(email string) (domain.User, bool, error)
	GetUserByID(id string) (domain.User, bool, error)
	ListUsers() ([]domain.User, error)
	ListUsersWithOptions(UserListOptions) ([]domain.User, int, error)
	UserCount() (int, error)

	// books
	SaveBook(domain.Book) error
	SetStatus(id string, status domain.BookStatus, errMsg string) error
	ListBooks() ([]domain.Book, error)
	ListBooksWithOptions(BookListOptions) ([]domain.Book, int, error)
	ListBooksByOwner(ownerID string) ([]domain.Book, error)
	GetBook(id string) (domain.Book, bool, error)
	DeleteBook(id string) error

	// chats
	AppendMessage(bookID string, msg domain.Message) error
	ListMessages(bookID string, limit int) ([]domain.Message, error)
	CreateConversation(conversation domain.Conversation) error
	GetConversation(id string) (domain.Conversation, bool, error)
	ListConversationsByUser(userID string, limit int) ([]domain.Conversation, error)
	UpdateConversation(id string, title string, lastMessageAt time.Time) error
	AppendConversationMessage(conversationID string, msg domain.Message) error
	ListConversationMessages(conversationID string, limit int) ([]domain.Message, error)

	// chunks
	ReplaceChunks(bookID string, chunks []domain.Chunk) error
	ListChunksByBook(bookID string) ([]domain.Chunk, error)
	SetChunkEmbedding(id string, embedding []float32) error
	SearchChunks(bookID string, embedding []float32, limit int) ([]domain.Chunk, error)

	// admin
	SaveAdminAuditLog(domain.AdminAuditLog) error
	ListAdminAuditLogs(AdminAuditLogListOptions) ([]domain.AdminAuditLog, int, error)
	GetAdminOverview(windowStart time.Time, windowHours int) (domain.AdminOverview, error)
}

// SessionStore persists session tokens.
type SessionStore interface {
	NewSession(userID string) (string, error)
	GetUserIDByToken(token string) (string, bool, error)
	DeleteSession(token string) error
}

// UserSessionRevoker is an optional capability that revokes all sessions
// issued for a user since a cutoff time.
type UserSessionRevoker interface {
	RevokeUserSessions(userID string, since time.Time) error
}

// UserRefreshTokenRevoker is an optional capability that revokes all refresh
// tokens for a user.
type UserRefreshTokenRevoker interface {
	RevokeUserRefreshTokens(userID string) error
}

// JWK represents a JSON Web Key entry used by JWKS endpoints.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
}

// JWKSProvider is an optional capability exposed by session stores that can
// publish JSON Web Keys.
type JWKSProvider interface {
	JWKS() []JWK
}
