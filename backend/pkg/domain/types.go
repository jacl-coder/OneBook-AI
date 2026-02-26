package domain

import "time"

type BookStatus string

const (
	StatusQueued     BookStatus = "queued"
	StatusProcessing BookStatus = "processing"
	StatusReady      BookStatus = "ready"
	StatusFailed     BookStatus = "failed"
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

type Book struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"ownerId"`
	Title            string     `json:"title"`
	OriginalFilename string     `json:"originalFilename"`
	StorageKey       string     `json:"-"`
	Status           BookStatus `json:"status"`
	ErrorMessage     string     `json:"errorMessage,omitempty"`
	SizeBytes        int64      `json:"sizeBytes"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId,omitempty"`
	UserID         string    `json:"userId,omitempty"`
	BookID         string    `json:"bookId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Sources        []Source  `json:"sources,omitempty"`
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
	ConversationID string    `json:"conversationId,omitempty"`
	BookID         string    `json:"bookId"`
	Question       string    `json:"question"`
	Answer         string    `json:"answer"`
	Sources        []Source  `json:"sources"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Source struct {
	Label    string `json:"label"`
	Location string `json:"location"`
	Snippet  string `json:"snippet"`
}

type Chunk struct {
	ID        string            `json:"id"`
	BookID    string            `json:"bookId"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"createdAt"`
}
