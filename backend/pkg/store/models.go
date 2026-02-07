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

type MessageModel struct {
	ID        string    `gorm:"primaryKey"`
	BookID    string    `gorm:"not null;index"`
	Role      string    `gorm:"not null"`
	Content   string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;index"`
}

type ChunkModel struct {
	ID        string           `gorm:"primaryKey"`
	BookID    string           `gorm:"not null;index"`
	Content   string           `gorm:"type:text;not null"`
	Metadata  datatypes.JSON   `gorm:"type:jsonb"`
	Embedding *pgvector.Vector `gorm:"type:vector(3072)"`
	CreatedAt time.Time        `gorm:"not null;index"`
}
