package store

import (
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/datatypes"
)

// GORM models used for persistence.
type UserModel struct {
	ID           string `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"not null"`
	Status       string
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time
}

type BookModel struct {
	ID               string `gorm:"primaryKey"`
	OwnerID          string `gorm:"not null;index"`
	Title            string `gorm:"not null"`
	OriginalFilename string `gorm:"not null"`
	StorageKey       string
	Status           string `gorm:"not null"`
	ErrorMessage     string
	SizeBytes        int64     `gorm:"not null"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
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
	ID        string           `gorm:"primaryKey"`
	BookID    string           `gorm:"not null;index"`
	Content   string           `gorm:"type:text;not null"`
	Metadata  datatypes.JSON   `gorm:"type:jsonb"`
	Embedding *pgvector.Vector `gorm:"type:vector(3072)"`
	CreatedAt time.Time        `gorm:"not null;index"`
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
