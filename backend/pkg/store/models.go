package store

import (
	"time"

	"gorm.io/datatypes"
)

// GORM models used for persistence.
type UserModel struct {
	ID           string `gorm:"primaryKey"`
	Email        string `gorm:"not null;default:'';index"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"not null"`
	Status       string
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time
}

type UserIdentityModel struct {
	ID         string     `gorm:"primaryKey"`
	UserID     string     `gorm:"not null;index"`
	Type       string     `gorm:"not null;index"`
	Provider   string     `gorm:"not null;default:'';index"`
	Identifier string     `gorm:"not null;index"`
	VerifiedAt *time.Time `gorm:"index"`
	IsPrimary  bool       `gorm:"not null;default:false;index"`
	CreatedAt  time.Time  `gorm:"not null"`
	UpdatedAt  time.Time  `gorm:"not null"`
}

type UserProfileModel struct {
	UserID             string     `gorm:"primaryKey"`
	DisplayName        string     `gorm:"not null;default:'';index"`
	AvatarURL          string     `gorm:"not null;default:''"`
	AvatarStorageKey   string     `gorm:"not null;default:'';index"`
	AvatarContentType  string     `gorm:"not null;default:''"`
	AdminNote          string     `gorm:"not null;default:''"`
	LastLoginAt        *time.Time `gorm:"index"`
	LoginCount         int        `gorm:"not null;default:0"`
	LastLoginIP        string     `gorm:"not null;default:''"`
	LastLoginUserAgent string     `gorm:"not null;default:''"`
	CreatedAt          time.Time  `gorm:"not null"`
	UpdatedAt          time.Time  `gorm:"not null"`
}

type BookModel struct {
	ID                   string         `gorm:"primaryKey"`
	OwnerID              string         `gorm:"not null;index"`
	Title                string         `gorm:"not null"`
	OriginalFilename     string         `gorm:"not null"`
	PrimaryCategory      string         `gorm:"not null;default:'other';index"`
	Tags                 datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'"`
	Format               string         `gorm:"not null;default:'';index"`
	Language             string         `gorm:"not null;default:'unknown';index"`
	StorageKey           string
	Status               string `gorm:"not null"`
	ErrorMessage         string
	SizeBytes            int64      `gorm:"not null"`
	CreatedAt            time.Time  `gorm:"not null"`
	UpdatedAt            time.Time  `gorm:"not null"`
	DeletedAt            *time.Time `gorm:"index"`
	CleanupStatus        string     `gorm:"index"`
	CleanupError         string
	CleanupAttempts      int        `gorm:"not null;default:0"`
	CleanupUpdatedAt     *time.Time `gorm:"index"`
	ProcessingGeneration int64      `gorm:"not null;default:0"`
}

type ConversationModel struct {
	ID            string     `gorm:"primaryKey"`
	UserID        string     `gorm:"not null;index"`
	BookID        *string    `gorm:"index"`
	Title         string     `gorm:"not null"`
	LastMessageAt *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"not null"`
	UpdatedAt     time.Time  `gorm:"not null"`
}

type MessageModel struct {
	ID             string         `gorm:"primaryKey"`
	ConversationID *string        `gorm:"index"`
	UserID         string         `gorm:"index"`
	BookID         string         `gorm:"not null;index"`
	Role           string         `gorm:"not null"`
	Content        string         `gorm:"not null"`
	Sources        datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt      time.Time      `gorm:"not null;index"`
}

type ChunkModel struct {
	ID        string         `gorm:"primaryKey"`
	BookID    string         `gorm:"not null;index"`
	Content   string         `gorm:"type:text;not null"`
	Metadata  datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt time.Time      `gorm:"not null;index"`
}

type ChunkIndexStatusModel struct {
	ChunkID            string     `gorm:"primaryKey"`
	BookID             string     `gorm:"not null;index"`
	ContentSHA256      string     `gorm:"not null;index"`
	EmbeddingModel     string     `gorm:"not null;default:''"`
	EmbeddingDim       int        `gorm:"not null;default:0"`
	OpenSearchStatus   string     `gorm:"not null;index"`
	OpenSearchSyncedAt *time.Time `gorm:"index"`
	QdrantStatus       string     `gorm:"not null;index"`
	QdrantSyncedAt     *time.Time `gorm:"index"`
	LastError          string
	CreatedAt          time.Time `gorm:"not null;index"`
	UpdatedAt          time.Time `gorm:"not null;index"`
}

type AdminAuditLogModel struct {
	ID         string         `gorm:"primaryKey"`
	ActorID    string         `gorm:"not null;index"`
	Action     string         `gorm:"not null;index"`
	TargetType string         `gorm:"not null;index"`
	TargetID   string         `gorm:"not null;index"`
	Before     datatypes.JSON `gorm:"type:jsonb"`
	After      datatypes.JSON `gorm:"type:jsonb"`
	RequestID  string         `gorm:"index"`
	IP         string
	UserAgent  string
	CreatedAt  time.Time `gorm:"not null;index"`
}

type EvalDatasetModel struct {
	ID          string  `gorm:"primaryKey"`
	Name        string  `gorm:"not null;index"`
	SourceType  string  `gorm:"not null;index"`
	BookID      *string `gorm:"index"`
	Version     int     `gorm:"not null"`
	Status      string  `gorm:"not null;index"`
	Description string
	Files       datatypes.JSON `gorm:"type:jsonb"`
	CreatedBy   string         `gorm:"not null;index"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}

type EvalRunModel struct {
	ID             string         `gorm:"primaryKey"`
	DatasetID      string         `gorm:"not null;index"`
	Fingerprint    string         `gorm:"not null;default:'';index"`
	Status         string         `gorm:"not null;index"`
	Mode           string         `gorm:"not null;index"`
	RetrievalMode  string         `gorm:"not null;index"`
	Params         datatypes.JSON `gorm:"type:jsonb"`
	GateMode       string         `gorm:"not null"`
	GateStatus     string         `gorm:"not null;index"`
	SummaryMetrics datatypes.JSON `gorm:"type:jsonb"`
	Warnings       datatypes.JSON `gorm:"type:jsonb"`
	Artifacts      datatypes.JSON `gorm:"type:jsonb"`
	StageSummaries datatypes.JSON `gorm:"type:jsonb"`
	Progress       int            `gorm:"not null"`
	ErrorMessage   string
	StartedAt      *time.Time `gorm:"index"`
	FinishedAt     *time.Time `gorm:"index"`
	CreatedBy      string     `gorm:"not null;index"`
	CreatedAt      time.Time  `gorm:"not null;index"`
	UpdatedAt      time.Time  `gorm:"not null;index"`
}

type IdempotencyRecordModel struct {
	ID             string `gorm:"primaryKey"`
	Scope          string `gorm:"not null;uniqueIndex:idx_idempotency_scope_actor_key,priority:1"`
	ActorID        string `gorm:"not null;uniqueIndex:idx_idempotency_scope_actor_key,priority:2"`
	IdempotencyKey string `gorm:"not null;uniqueIndex:idx_idempotency_scope_actor_key,priority:3"`
	RequestHash    string `gorm:"not null"`
	ResourceType   string
	ResourceID     string
	StatusCode     int            `gorm:"not null"`
	State          string         `gorm:"not null;index"`
	ResponseJSON   datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt      time.Time      `gorm:"not null;index"`
	UpdatedAt      time.Time      `gorm:"not null;index"`
}

type OutboxMessageModel struct {
	ID           string         `gorm:"primaryKey"`
	Topic        string         `gorm:"not null;index"`
	ResourceType string         `gorm:"not null;index"`
	ResourceID   string         `gorm:"not null;index"`
	PayloadJSON  datatypes.JSON `gorm:"type:jsonb;not null"`
	Attempts     int            `gorm:"not null;default:0"`
	LastError    string
	AvailableAt  time.Time  `gorm:"not null;index"`
	LockedAt     *time.Time `gorm:"index"`
	DispatchedAt *time.Time `gorm:"index"`
	CreatedAt    time.Time  `gorm:"not null;index"`
	UpdatedAt    time.Time  `gorm:"not null;index"`
}
