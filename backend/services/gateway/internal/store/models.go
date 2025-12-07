package store

import "time"

// GORM models used for persistence.
type UserModel struct {
	ID           string    `gorm:"primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

type BookModel struct {
	ID               string `gorm:"primaryKey"`
	OwnerID          string `gorm:"not null;index"`
	Title            string `gorm:"not null"`
	OriginalFilename string `gorm:"not null"`
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
