package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"onebookai/pkg/domain"
)

const migrateLockID int64 = 73217321

// GormStore implements Store using GORM + Postgres.
type GormStore struct {
	db *gorm.DB
}

// NewGormStore opens the DB and runs auto-migrations.
func NewGormStore(dsn string) (*GormStore, error) {
	gormLog := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := withMigrationLock(db, func(tx *gorm.DB) error {
		if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
			return fmt.Errorf("create pgvector extension: %w", err)
		}
		if err := tx.AutoMigrate(&UserModel{}, &BookModel{}, &MessageModel{}, &ChunkModel{}); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
		if err := tx.Exec(`
			DO $$
			BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'chunk_models' AND column_name = 'embedding'
			) THEN
				ALTER TABLE chunk_models ALTER COLUMN embedding TYPE vector(3072);
			END IF;
			END $$;
		`).Error; err != nil {
			return fmt.Errorf("alter chunk embedding type: %w", err)
		}
		if err := tx.Exec(`
			UPDATE chunk_models
			SET metadata = jsonb_set(
				jsonb_set(
					COALESCE(metadata, '{}'::jsonb),
					'{source_type}',
					to_jsonb(
						CASE
							WHEN metadata ? 'page' THEN 'pdf'
							WHEN metadata ? 'section' THEN 'epub'
							ELSE 'text'
						END
					),
					true
				),
				'{source_ref}',
				to_jsonb(
					CASE
						WHEN metadata ? 'page' THEN 'page:' || (metadata->>'page')
						WHEN metadata ? 'section' THEN 'section:' || (metadata->>'section')
						ELSE 'text'
					END
				),
				true
			)
			WHERE metadata IS NULL
			   OR NOT (metadata ? 'source_type' AND metadata ? 'source_ref');
		`).Error; err != nil {
			return fmt.Errorf("backfill chunk metadata: %w", err)
		}
		if err := tx.Exec(`
			DO $$
			BEGIN
				DELETE FROM chunk_models c
				WHERE NOT EXISTS (SELECT 1 FROM book_models b WHERE b.id = c.book_id);
				DELETE FROM message_models m
				WHERE NOT EXISTS (SELECT 1 FROM book_models b WHERE b.id = m.book_id);
				IF NOT EXISTS (
					SELECT 1 FROM information_schema.table_constraints
					WHERE table_schema = 'public'
					AND table_name = 'chunk_models'
					AND constraint_name = 'chunk_models_book_id_fkey'
				) THEN
					ALTER TABLE chunk_models
					ADD CONSTRAINT chunk_models_book_id_fkey
					FOREIGN KEY (book_id) REFERENCES book_models(id) ON DELETE CASCADE;
				END IF;
				IF NOT EXISTS (
					SELECT 1 FROM information_schema.table_constraints
					WHERE table_schema = 'public'
					AND table_name = 'message_models'
					AND constraint_name = 'message_models_book_id_fkey'
				) THEN
					ALTER TABLE message_models
					ADD CONSTRAINT message_models_book_id_fkey
					FOREIGN KEY (book_id) REFERENCES book_models(id) ON DELETE CASCADE;
				END IF;
			END $$;
		`).Error; err != nil {
			return fmt.Errorf("ensure book foreign keys: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &GormStore{db: db}, nil
}

func withMigrationLock(db *gorm.DB, fn func(*gorm.DB) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open sql conn: %w", err)
	}
	defer conn.Close()
	if err := execAdvisory(ctx, conn, "SELECT pg_advisory_lock($1)", migrateLockID); err != nil {
		return fmt.Errorf("acquire migrate lock: %w", err)
	}
	defer func() {
		_ = execAdvisory(ctx, conn, "SELECT pg_advisory_unlock($1)", migrateLockID)
	}()
	return fn(db)
}

func execAdvisory(ctx context.Context, conn *sql.Conn, query string, lockID int64) error {
	_, err := conn.ExecContext(ctx, query, lockID)
	return err
}

// SaveUser registers or updates a user.
func (s *GormStore) SaveUser(u domain.User) error {
	model := userToModel(u)
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"email", "password_hash", "role", "status", "updated_at"}),
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
		DoUpdates: clause.AssignmentColumns([]string{"owner_id", "title", "original_filename", "storage_key", "status", "error_message", "size_bytes", "updated_at"}),
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

// DeleteBook removes book and messages (chunks handled by FK cascade).
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

// ListMessages returns recent messages for a book (newest first, then reversed to chronological).
func (s *GormStore) ListMessages(bookID string, limit int) ([]domain.Message, error) {
	if limit <= 0 {
		return []domain.Message{}, nil
	}
	var models []MessageModel
	if err := s.db.Where("book_id = ?", bookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	msgs := make([]domain.Message, 0, len(models))
	for i := len(models) - 1; i >= 0; i-- {
		msgs = append(msgs, messageFromModel(models[i]))
	}
	return msgs, nil
}

// ReplaceChunks replaces all chunks for a book.
func (s *GormStore) ReplaceChunks(bookID string, chunks []domain.Chunk) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&ChunkModel{}, "book_id = ?", bookID).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}
		models := make([]ChunkModel, 0, len(chunks))
		for _, chunk := range chunks {
			model := chunkToModel(chunk)
			model.BookID = bookID
			models = append(models, model)
		}
		return tx.CreateInBatches(&models, 200).Error
	})
}

// ListChunksByBook returns chunks for a book.
func (s *GormStore) ListChunksByBook(bookID string) ([]domain.Chunk, error) {
	var models []ChunkModel
	if err := s.db.Where("book_id = ?", bookID).Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	chunks := make([]domain.Chunk, 0, len(models))
	for _, model := range models {
		chunks = append(chunks, chunkFromModel(model))
	}
	return chunks, nil
}

// SetChunkEmbedding updates the embedding vector for a chunk.
func (s *GormStore) SetChunkEmbedding(id string, embedding []float32) error {
	return s.db.Model(&ChunkModel{}).Where("id = ?", id).
		Update("embedding", pgvector.NewVector(embedding)).Error
}

// SearchChunks finds similar chunks by cosine distance.
func (s *GormStore) SearchChunks(bookID string, embedding []float32, limit int) ([]domain.Chunk, error) {
	if limit <= 0 {
		return []domain.Chunk{}, nil
	}
	vec := pgvector.NewVector(embedding)
	var models []ChunkModel
	if err := s.db.Model(&ChunkModel{}).
		Where("book_id = ? AND embedding IS NOT NULL", bookID).
		Order(clause.Expr{SQL: "embedding <=> ?", Vars: []any{vec}}).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	chunks := make([]domain.Chunk, 0, len(models))
	for _, model := range models {
		chunks = append(chunks, chunkFromModel(model))
	}
	return chunks, nil
}

func userToModel(u domain.User) UserModel {
	return UserModel{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         string(u.Role),
		Status:       string(u.Status),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func userFromModel(m UserModel) domain.User {
	status := domain.UserStatus(m.Status)
	if status == "" {
		status = domain.StatusActive
	}
	return domain.User{
		ID:           m.ID,
		Email:        m.Email,
		PasswordHash: m.PasswordHash,
		Role:         domain.UserRole(m.Role),
		Status:       status,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func bookToModel(b domain.Book) BookModel {
	return BookModel{
		ID:               b.ID,
		OwnerID:          b.OwnerID,
		Title:            b.Title,
		OriginalFilename: b.OriginalFilename,
		StorageKey:       b.StorageKey,
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
		StorageKey:       m.StorageKey,
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

func messageFromModel(m MessageModel) domain.Message {
	return domain.Message{
		ID:        m.ID,
		BookID:    m.BookID,
		Role:      m.Role,
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
}

func chunkToModel(chunk domain.Chunk) ChunkModel {
	meta, _ := json.Marshal(chunk.Metadata)
	return ChunkModel{
		ID:        chunk.ID,
		BookID:    chunk.BookID,
		Content:   chunk.Content,
		Metadata:  meta,
		CreatedAt: chunk.CreatedAt,
	}
}

func chunkFromModel(model ChunkModel) domain.Chunk {
	var meta map[string]string
	if len(model.Metadata) > 0 {
		_ = json.Unmarshal(model.Metadata, &meta)
	}
	return domain.Chunk{
		ID:        model.ID,
		BookID:    model.BookID,
		Content:   model.Content,
		Metadata:  meta,
		CreatedAt: model.CreatedAt,
	}
}
