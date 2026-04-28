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
	Query           string
	OwnerID         string
	Status          string
	PrimaryCategory string
	Tag             string
	Format          string
	Language        string
	SortBy          string
	SortOrder       string
	Page            int
	PageSize        int
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

type EvalDatasetListOptions struct {
	Query      string
	SourceType string
	Status     string
	BookID     string
	Page       int
	PageSize   int
}

type EvalRunListOptions struct {
	DatasetID     string
	Status        string
	Mode          string
	RetrievalMode string
	Page          int
	PageSize      int
}

// Store defines persistence operations for users, books, and messages.
type Store interface {
	// users
	SaveUser(domain.User) error
	SaveUserWithIdentity(domain.User, domain.UserIdentity) error
	SaveUserWithIdentities(domain.User, []domain.UserIdentity) error
	SaveUserIdentity(domain.UserIdentity) error
	HasUserEmail(email string) (bool, error)
	HasUserIdentity(identityType domain.IdentityType, identifier string) (bool, error)
	GetUserByEmail(email string) (domain.User, bool, error)
	GetUserByIdentity(identityType domain.IdentityType, identifier string) (domain.User, bool, error)
	GetUserByProviderIdentity(provider, identifier string) (domain.User, bool, error)
	GetUserByID(id string) (domain.User, bool, error)
	ListUserIdentities(userIDs []string) (map[string][]domain.UserIdentity, error)
	DeleteUserIdentity(userID string, identityType domain.IdentityType) error
	DeleteUser(userID string) error
	ListUsers() ([]domain.User, error)
	ListUsersWithOptions(UserListOptions) ([]domain.User, int, error)
	UserCount() (int, error)

	// books
	SaveBook(domain.Book) error
	SaveBookAndOutbox(domain.Book, *domain.IdempotencyRecord, *domain.OutboxMessage) error
	SetStatus(id string, status domain.BookStatus, errMsg string) error
	SetStatusIfGeneration(id string, generation int64, status domain.BookStatus, errMsg string) (bool, error)
	ListBooks() ([]domain.Book, error)
	ListBooksWithOptions(BookListOptions) ([]domain.Book, int, error)
	ListBooksByOwner(ownerID string) ([]domain.Book, error)
	GetBook(id string) (domain.Book, bool, error)
	GetBookIncludingDeleted(id string) (domain.Book, bool, error)
	ListBooksPendingCleanup(limit int) ([]domain.Book, error)
	ClaimBooksPendingCleanup(limit int) ([]domain.Book, error)
	MarkBookDeleted(id string, cleanupStatus domain.BookCleanupStatus) error
	UpdateBookCleanup(id string, status domain.BookCleanupStatus, errMsg string, incrementAttempts bool) error
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
	SaveConversationExchange(domain.Conversation, bool, domain.Message, domain.Message, *domain.IdempotencyRecord) error

	// chunks
	ReplaceChunks(bookID string, chunks []domain.Chunk) error
	ListChunksByBook(bookID string) ([]domain.Chunk, error)
	GetChunksByIDs(ids []string) ([]domain.Chunk, error)
	ListChunkIndexStatusesByBook(bookID string) ([]domain.ChunkIndexStatus, error)
	UpdateChunkIndexStatus(chunkIDs []string, backend domain.ChunkIndexBackend, status domain.ChunkIndexSyncStatus, embeddingModel string, embeddingDim int, errMsg string) error

	// admin
	SaveAdminAuditLog(domain.AdminAuditLog) error
	ListAdminAuditLogs(AdminAuditLogListOptions) ([]domain.AdminAuditLog, int, error)
	GetAdminOverview(windowStart time.Time, windowHours int) (domain.AdminOverview, error)
	SaveEvalDataset(domain.EvalDataset) error
	GetEvalDataset(id string) (domain.EvalDataset, bool, error)
	ListEvalDatasets(EvalDatasetListOptions) ([]domain.EvalDataset, int, error)
	DeleteEvalDataset(id string) error
	ArchiveEvalDataset(id string) error
	SaveEvalRun(domain.EvalRun) error
	GetEvalRun(id string) (domain.EvalRun, bool, error)
	GetActiveEvalRunByFingerprint(fingerprint string) (domain.EvalRun, bool, error)
	ListEvalRuns(EvalRunListOptions) ([]domain.EvalRun, int, error)
	CountEvalRunsByDataset(datasetID string) (int, error)
	GetAdminEvalOverview(windowStart time.Time) (domain.AdminEvalOverview, error)
	SaveIdempotencyRecord(domain.IdempotencyRecord) error
	GetIdempotencyRecord(scope, actorID, key string) (domain.IdempotencyRecord, bool, error)
	ClaimOutboxMessages(topic string, limit int, lease time.Duration) ([]domain.OutboxMessage, error)
	MarkOutboxDispatched(id string) error
	ReleaseOutboxMessage(id, errMsg string, availableAt time.Time) error
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
