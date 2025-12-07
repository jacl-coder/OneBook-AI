package store

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"onebookai/pkg/domain"
)

// GormStore implements Store using GORM + Postgres.
type GormStore struct {
	db *gorm.DB
}

// NewGormStore opens the DB and runs auto-migrations.
func NewGormStore(dsn string) (*GormStore, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.AutoMigrate(&UserModel{}, &BookModel{}, &MessageModel{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	return &GormStore{db: db}, nil
}

// SaveUser registers or updates a user.
func (s *GormStore) SaveUser(u domain.User) error {
	model := userToModel(u)
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"email", "password_hash", "role"}),
	}).Create(&model).Error
}

// HasUserEmail checks if email exists.
func (s *GormStore) HasUserEmail(email string) (bool, error) {
	var count int64
	if err := s.db.Model(&UserModel{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserByEmail looks up a user by email.
func (s *GormStore) GetUserByEmail(email string) (domain.User, bool, error) {
	var model UserModel
	if err := s.db.Where("email = ?", email).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}
	return userFromModel(model), true, nil
}

// GetUserByID returns a user by ID.
func (s *GormStore) GetUserByID(id string) (domain.User, bool, error) {
	var model UserModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.User{}, false, nil
		}
		return domain.User{}, false, err
	}
	return userFromModel(model), true, nil
}

// ListUsers returns all users ordered by created_at.
func (s *GormStore) ListUsers() ([]domain.User, error) {
	var models []UserModel
	if err := s.db.Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]domain.User, 0, len(models))
	for _, m := range models {
		res = append(res, userFromModel(m))
	}
	return res, nil
}

// UserCount returns number of users.
func (s *GormStore) UserCount() (int, error) {
	var count int64
	if err := s.db.Model(&UserModel{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// SaveBook stores or updates a book.
func (s *GormStore) SaveBook(b domain.Book) error {
	model := bookToModel(b)
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"owner_id", "title", "original_filename", "status", "error_message", "size_bytes", "updated_at"}),
	}).Create(&model).Error
}

// SetStatus updates book status/error.
func (s *GormStore) SetStatus(id string, status domain.BookStatus, errMsg string) error {
	return s.db.Model(&BookModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        string(status),
			"error_message": errMsg,
			"updated_at":    time.Now().UTC(),
		}).Error
}

// ListBooks returns all books ordered by created_at.
func (s *GormStore) ListBooks() ([]domain.Book, error) {
	return s.listBooks("created_at ASC")
}

// ListBooksByOwner returns books filtered by owner.
func (s *GormStore) ListBooksByOwner(ownerID string) ([]domain.Book, error) {
	return s.listBooks("created_at ASC", "owner_id = ?", ownerID)
}

func (s *GormStore) listBooks(order string, conds ...any) ([]domain.Book, error) {
	var models []BookModel
	tx := s.db.Order(order)
	if len(conds) > 0 {
		tx = tx.Where(conds[0], conds[1:]...)
	}
	if err := tx.Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]domain.Book, 0, len(models))
	for _, m := range models {
		res = append(res, bookFromModel(m))
	}
	return res, nil
}

// GetBook retrieves a book.
func (s *GormStore) GetBook(id string) (domain.Book, bool, error) {
	var model BookModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Book{}, false, nil
		}
		return domain.Book{}, false, err
	}
	return bookFromModel(model), true, nil
}

// DeleteBook removes book and messages.
func (s *GormStore) DeleteBook(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&MessageModel{}, "book_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&BookModel{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

// AppendMessage records a message.
func (s *GormStore) AppendMessage(bookID string, msg domain.Message) error {
	model := messageToModel(msg)
	model.BookID = bookID
	return s.db.Create(&model).Error
}

func userToModel(u domain.User) UserModel {
	return UserModel{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         string(u.Role),
		CreatedAt:    u.CreatedAt,
	}
}

func userFromModel(m UserModel) domain.User {
	return domain.User{
		ID:           m.ID,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		Role:         domain.UserRole(m.Role),
		CreatedAt:    m.CreatedAt,
	}
}

func bookToModel(b domain.Book) BookModel {
	return BookModel{
		ID:               b.ID,
		OwnerID:          b.OwnerID,
		Title:            b.Title,
		OriginalFilename: b.OriginalFilename,
		Status:           string(b.Status),
		ErrorMessage:     b.ErrorMessage,
		SizeBytes:        b.SizeBytes,
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
	}
}

func bookFromModel(m BookModel) domain.Book {
	return domain.Book{
		ID:               m.ID,
		OwnerID:          m.OwnerID,
		Title:            m.Title,
		OriginalFilename: m.OriginalFilename,
		Status:           domain.BookStatus(m.Status),
		ErrorMessage:     m.ErrorMessage,
		SizeBytes:        m.SizeBytes,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func messageToModel(msg domain.Message) MessageModel {
	return MessageModel{
		ID:        msg.ID,
		BookID:    msg.BookID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}
}
