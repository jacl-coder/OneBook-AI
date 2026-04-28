package domain

import (
	"encoding/json"
	"time"
)

type BookStatus string

const (
	StatusQueued     BookStatus = "queued"
	StatusProcessing BookStatus = "processing"
	StatusReady      BookStatus = "ready"
	StatusFailed     BookStatus = "failed"
)

type BookCleanupStatus string

const (
	BookCleanupStatusQueued  BookCleanupStatus = "queued"
	BookCleanupStatusRunning BookCleanupStatus = "running"
	BookCleanupStatusFailed  BookCleanupStatus = "failed"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
)

type IdentityType string

const (
	IdentityEmail IdentityType = "email"
	IdentityPhone IdentityType = "phone"
	IdentityOAuth IdentityType = "oauth"
)

type Book struct {
	ID                   string     `json:"id"`
	OwnerID              string     `json:"ownerId"`
	Title                string     `json:"title"`
	OriginalFilename     string     `json:"originalFilename"`
	PrimaryCategory      string     `json:"primaryCategory"`
	Tags                 []string   `json:"tags"`
	Format               string     `json:"format"`
	Language             string     `json:"language"`
	DocumentType         string     `json:"documentType,omitempty"`
	DocumentSummary      string     `json:"documentSummary,omitempty"`
	FirstPageText        string     `json:"firstPageText,omitempty"`
	Keywords             []string   `json:"keywords,omitempty"`
	StorageKey           string     `json:"-"`
	Status               BookStatus `json:"status"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	SizeBytes            int64      `json:"sizeBytes"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
	CleanupStatus        string     `json:"cleanupStatus,omitempty"`
	CleanupError         string     `json:"cleanupError,omitempty"`
	CleanupAttempts      int        `json:"cleanupAttempts,omitempty"`
	CleanupUpdatedAt     *time.Time `json:"cleanupUpdatedAt,omitempty"`
	ProcessingGeneration int64      `json:"-"`
}

type BookDocumentProfile struct {
	DocumentType    string
	DocumentSummary string
	FirstPageText   string
	Keywords        []string
}

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName,omitempty"`
	AvatarURL    string     `json:"avatarUrl,omitempty"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	LoginCount   int        `json:"loginCount,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type AdminUser struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Phone          string     `json:"phone"`
	DisplayName    string     `json:"displayName"`
	AvatarURL      string     `json:"avatarUrl"`
	AdminNote      string     `json:"adminNote"`
	LoginMethods   []string   `json:"loginMethods"`
	OAuthProviders []string   `json:"oauthProviders"`
	EmailVerified  bool       `json:"emailVerified"`
	PhoneVerified  bool       `json:"phoneVerified"`
	PasswordSet    bool       `json:"passwordSet"`
	LastLoginAt    *time.Time `json:"lastLoginAt,omitempty"`
	LoginCount     int        `json:"loginCount"`
	Role           UserRole   `json:"role"`
	Status         UserStatus `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type UserProfile struct {
	UserID             string     `json:"userId"`
	DisplayName        string     `json:"displayName"`
	AvatarURL          string     `json:"avatarUrl"`
	AvatarStorageKey   string     `json:"-"`
	AvatarContentType  string     `json:"-"`
	AdminNote          string     `json:"-"`
	LastLoginAt        *time.Time `json:"lastLoginAt,omitempty"`
	LoginCount         int        `json:"loginCount"`
	LastLoginIP        string     `json:"-"`
	LastLoginUserAgent string     `json:"-"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type UserIdentity struct {
	ID         string       `json:"id"`
	UserID     string       `json:"userId"`
	Type       IdentityType `json:"type"`
	Provider   string       `json:"provider,omitempty"`
	Identifier string       `json:"identifier"`
	VerifiedAt *time.Time   `json:"verifiedAt,omitempty"`
	IsPrimary  bool         `json:"isPrimary"`
	CreatedAt  time.Time    `json:"createdAt"`
	UpdatedAt  time.Time    `json:"updatedAt"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	UserID         string    `json:"userId,omitempty"`
	BookID         string    `json:"bookId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Sources        []Source  `json:"sources,omitempty"`
	Abstained      bool      `json:"abstained,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Conversation struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	BookID        string     `json:"bookId,omitempty"`
	Title         string     `json:"title"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type Answer struct {
	Conversation   Conversation    `json:"conversation"`
	Question       string          `json:"question"`
	Answer         string          `json:"answer"`
	Citations      []Source        `json:"citations"`
	Abstained      bool            `json:"abstained"`
	RetrievalDebug *RetrievalDebug `json:"retrievalDebug,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type Source struct {
	Label     string  `json:"label"`
	Location  string  `json:"location"`
	Snippet   string  `json:"snippet"`
	ChunkID   string  `json:"chunkId,omitempty"`
	SourceRef string  `json:"sourceRef,omitempty"`
	Score     float64 `json:"score,omitempty"`
	Language  string  `json:"language,omitempty"`
}

type RetrievalHit struct {
	ChunkID   string  `json:"chunkId"`
	SourceRef string  `json:"sourceRef,omitempty"`
	Score     float64 `json:"score"`
	Stage     string  `json:"stage"`
	Snippet   string  `json:"snippet,omitempty"`
}

type RetrievalDebug struct {
	Language string         `json:"language"`
	Queries  []string       `json:"queries"`
	Dense    []RetrievalHit `json:"dense"`
	Lexical  []RetrievalHit `json:"lexical"`
	Fused    []RetrievalHit `json:"fused"`
	Reranked []RetrievalHit `json:"reranked"`
}

type Chunk struct {
	ID        string            `json:"id"`
	BookID    string            `json:"bookId"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"createdAt"`
}

type ChunkIndexSyncStatus string

const (
	ChunkIndexSyncStatusPending ChunkIndexSyncStatus = "pending"
	ChunkIndexSyncStatusSynced  ChunkIndexSyncStatus = "synced"
	ChunkIndexSyncStatusFailed  ChunkIndexSyncStatus = "failed"
)

type ChunkIndexBackend string

const (
	ChunkIndexBackendOpenSearch ChunkIndexBackend = "opensearch"
	ChunkIndexBackendQdrant     ChunkIndexBackend = "qdrant"
)

type ChunkIndexStatus struct {
	ChunkID            string               `json:"chunkId"`
	BookID             string               `json:"bookId"`
	ContentSHA256      string               `json:"contentSha256"`
	EmbeddingModel     string               `json:"embeddingModel,omitempty"`
	EmbeddingDim       int                  `json:"embeddingDim,omitempty"`
	OpenSearchStatus   ChunkIndexSyncStatus `json:"openSearchStatus"`
	OpenSearchSyncedAt *time.Time           `json:"openSearchSyncedAt,omitempty"`
	QdrantStatus       ChunkIndexSyncStatus `json:"qdrantStatus"`
	QdrantSyncedAt     *time.Time           `json:"qdrantSyncedAt,omitempty"`
	LastError          string               `json:"lastError,omitempty"`
	CreatedAt          time.Time            `json:"createdAt"`
	UpdatedAt          time.Time            `json:"updatedAt"`
}

type BookIndexStatusSummary struct {
	BookID        string             `json:"bookId"`
	TotalChunks   int                `json:"totalChunks"`
	PendingChunks int                `json:"pendingChunks"`
	SyncedChunks  int                `json:"syncedChunks"`
	FailedChunks  int                `json:"failedChunks"`
	Items         []ChunkIndexStatus `json:"items"`
}

type AdminAuditLog struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actorId"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
	RequestID  string         `json:"requestId,omitempty"`
	IP         string         `json:"ip,omitempty"`
	UserAgent  string         `json:"userAgent,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type BookStatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type AdminOverview struct {
	TotalUsers      int               `json:"totalUsers"`
	ActiveUsers     int               `json:"activeUsers"`
	DisabledUsers   int               `json:"disabledUsers"`
	TotalBooks      int               `json:"totalBooks"`
	BooksByStatus   []BookStatusCount `json:"booksByStatus"`
	BooksCreated24h int               `json:"booksCreated24h"`
	BooksFailed24h  int               `json:"booksFailed24h"`
	RefreshedAt     time.Time         `json:"refreshedAt"`
	WindowStart     time.Time         `json:"windowStart"`
	WindowHours     int               `json:"windowHours"`
}

type EvalDatasetSourceType string

const (
	EvalDatasetSourceUpload EvalDatasetSourceType = "upload"
	EvalDatasetSourceBook   EvalDatasetSourceType = "book"
)

type EvalDatasetStatus string

const (
	EvalDatasetStatusActive   EvalDatasetStatus = "active"
	EvalDatasetStatusArchived EvalDatasetStatus = "archived"
)

type EvalRunStatus string

const (
	EvalRunStatusQueued    EvalRunStatus = "queued"
	EvalRunStatusRunning   EvalRunStatus = "running"
	EvalRunStatusSucceeded EvalRunStatus = "succeeded"
	EvalRunStatusFailed    EvalRunStatus = "failed"
	EvalRunStatusCanceled  EvalRunStatus = "canceled"
)

type EvalRunMode string

const (
	EvalRunModeRetrieval     EvalRunMode = "retrieval"
	EvalRunModePostRetrieval EvalRunMode = "post_retrieval"
	EvalRunModeAnswer        EvalRunMode = "answer"
	EvalRunModeAll           EvalRunMode = "all"
)

type EvalRetrievalMode string

const (
	EvalRetrievalModeHybridBest  EvalRetrievalMode = "hybrid_best"
	EvalRetrievalModeHybrid      EvalRetrievalMode = "hybrid_no_rerank"
	EvalRetrievalModeDenseOnly   EvalRetrievalMode = "dense_only"
	EvalRetrievalModeLexicalOnly EvalRetrievalMode = "lexical_only"
	EvalRetrievalModeSparseOnly  EvalRetrievalMode = "sparse_only"
)

type EvalGateStatus string

const (
	EvalGateStatusPassed EvalGateStatus = "passed"
	EvalGateStatusWarn   EvalGateStatus = "warn"
	EvalGateStatusFailed EvalGateStatus = "failed"
)

type EvalDataset struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	SourceType  EvalDatasetSourceType `json:"sourceType"`
	BookID      string                `json:"bookId,omitempty"`
	Version     int                   `json:"version"`
	Status      EvalDatasetStatus     `json:"status"`
	Description string                `json:"description,omitempty"`
	Files       map[string]string     `json:"files"`
	CreatedBy   string                `json:"createdBy"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

type EvalRunArtifact struct {
	Name        string    `json:"name"`
	Path        string    `json:"path,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
}

type EvalRunStageSummary struct {
	Stage   string         `json:"stage"`
	Metrics map[string]any `json:"metrics"`
}

type EvalRun struct {
	ID             string                `json:"id"`
	DatasetID      string                `json:"datasetId"`
	Fingerprint    string                `json:"-"`
	Status         EvalRunStatus         `json:"status"`
	Mode           EvalRunMode           `json:"mode"`
	RetrievalMode  EvalRetrievalMode     `json:"retrievalMode"`
	Params         map[string]any        `json:"params,omitempty"`
	GateMode       string                `json:"gateMode"`
	GateStatus     EvalGateStatus        `json:"gateStatus"`
	SummaryMetrics map[string]any        `json:"summaryMetrics,omitempty"`
	Warnings       []string              `json:"warnings,omitempty"`
	Artifacts      []EvalRunArtifact     `json:"artifacts,omitempty"`
	StageSummaries []EvalRunStageSummary `json:"stageSummaries,omitempty"`
	Progress       int                   `json:"progress"`
	ErrorMessage   string                `json:"errorMessage,omitempty"`
	StartedAt      *time.Time            `json:"startedAt,omitempty"`
	FinishedAt     *time.Time            `json:"finishedAt,omitempty"`
	CreatedBy      string                `json:"createdBy"`
	CreatedAt      time.Time             `json:"createdAt"`
	UpdatedAt      time.Time             `json:"updatedAt"`
}

type IdempotencyState string

const (
	IdempotencyStateProcessing IdempotencyState = "processing"
	IdempotencyStateCompleted  IdempotencyState = "completed"
	IdempotencyStateFailed     IdempotencyState = "failed"
)

type IdempotencyRecord struct {
	ID             string           `json:"id"`
	Scope          string           `json:"scope"`
	ActorID        string           `json:"actorId"`
	IdempotencyKey string           `json:"idempotencyKey"`
	RequestHash    string           `json:"requestHash"`
	ResourceType   string           `json:"resourceType,omitempty"`
	ResourceID     string           `json:"resourceId,omitempty"`
	StatusCode     int              `json:"statusCode"`
	State          IdempotencyState `json:"state"`
	ResponseJSON   json.RawMessage  `json:"-"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

type OutboxMessage struct {
	ID           string          `json:"id"`
	Topic        string          `json:"topic"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	PayloadJSON  json.RawMessage `json:"payloadJson"`
	Attempts     int             `json:"attempts"`
	LastError    string          `json:"lastError,omitempty"`
	AvailableAt  time.Time       `json:"availableAt"`
	LockedAt     *time.Time      `json:"lockedAt,omitempty"`
	DispatchedAt *time.Time      `json:"dispatchedAt,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type AdminEvalOverview struct {
	TotalDatasets    int       `json:"totalDatasets"`
	ActiveDatasets   int       `json:"activeDatasets"`
	TotalRuns        int       `json:"totalRuns"`
	QueuedRuns       int       `json:"queuedRuns"`
	RunningRuns      int       `json:"runningRuns"`
	SuccessfulRuns   int       `json:"successfulRuns"`
	FailedRuns       int       `json:"failedRuns"`
	CanceledRuns     int       `json:"canceledRuns"`
	RecentRuns       int       `json:"recentRuns"`
	RecentGateFailed int       `json:"recentGateFailed"`
	SuccessRate      float64   `json:"successRate"`
	RefreshedAt      time.Time `json:"refreshedAt"`
}
