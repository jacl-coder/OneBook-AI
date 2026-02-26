package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"onebookai/pkg/domain"
)

const migrateLockID int64 = 73217321

const (
	defaultEmbeddingDim      = 3072
	canonicalEmbeddingDimEnv = "ONEBOOK_EMBEDDING_DIM"
)

type GormStoreOptions struct {
	EmbeddingDim int
}

type GormStoreOption func(*GormStoreOptions)

// WithEmbeddingDim sets the canonical embedding dimension used by storage.
func WithEmbeddingDim(dim int) GormStoreOption {
	return func(opts *GormStoreOptions) {
		opts.EmbeddingDim = dim
	}
}

// GormStore implements Store using GORM + Postgres.
type GormStore struct {
	db           *gorm.DB
	embeddingDim int
}

// NewGormStore opens the DB and runs auto-migrations.
func NewGormStore(dsn string, options ...GormStoreOption) (*GormStore, error) {
	opts := GormStoreOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	embeddingDim, err := resolveEmbeddingDim(opts.EmbeddingDim)
	if err != nil {
		return nil, err
	}

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
		if err := tx.AutoMigrate(&UserModel{}, &BookModel{}, &ConversationModel{}, &MessageModel{}, &ChunkModel{}); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
		if err := tx.Exec(fmt.Sprintf(`
			DO $$
			BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'chunk_models' AND column_name = 'embedding'
			) THEN
				ALTER TABLE chunk_models ALTER COLUMN embedding TYPE vector(%d);
			END IF;
			END $$;
		`, embeddingDim)).Error; err != nil {
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
				DELETE FROM message_models m
				WHERE m.conversation_id IS NOT NULL
				  AND NOT EXISTS (SELECT 1 FROM conversation_models c WHERE c.id = m.conversation_id);
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
				IF NOT EXISTS (
					SELECT 1 FROM information_schema.table_constraints
					WHERE table_schema = 'public'
					AND table_name = 'message_models'
					AND constraint_name = 'message_models_conversation_id_fkey'
				) THEN
					ALTER TABLE message_models
					ADD CONSTRAINT message_models_conversation_id_fkey
					FOREIGN KEY (conversation_id) REFERENCES conversation_models(id) ON DELETE CASCADE;
				END IF;
			END $$;
		`).Error; err != nil {
			return fmt.Errorf("ensure book foreign keys: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &GormStore{db: db, embeddingDim: embeddingDim}, nil
}

func resolveEmbeddingDim(configValue int) (int, error) {
	if configValue > 0 {
		return configValue, nil
	}
	raw := strings.TrimSpace(os.Getenv(canonicalEmbeddingDimEnv))
	if raw == "" {
		return defaultEmbeddingDim, nil
	}
	dim, err := strconv.Atoi(raw)
	if err != nil || dim <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", canonicalEmbeddingDimEnv, raw)
	}
	return dim, nil
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
		if err := tx.Delete(&ConversationModel{}, "book_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&MessageModel{}, "book_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&BookModel{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

// CreateConversation creates a new conversation record.
func (s *GormStore) CreateConversation(conversation domain.Conversation) error {
	model := conversationToModel(conversation)
	return s.db.Create(&model).Error
}

// GetConversation returns one conversation by ID.
func (s *GormStore) GetConversation(id string) (domain.Conversation, bool, error) {
	var model ConversationModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Conversation{}, false, nil
		}
		return domain.Conversation{}, false, err
	}
	return conversationFromModel(model), true, nil
}

// ListConversationsByUser returns latest conversations of a user.
func (s *GormStore) ListConversationsByUser(userID string, limit int) ([]domain.Conversation, error) {
	if limit <= 0 {
		limit = 100
	}
	var models []ConversationModel
	if err := s.db.Where("user_id = ?", userID).
		Order("last_message_at DESC NULLS LAST").
		Order("updated_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]domain.Conversation, 0, len(models))
	for _, model := range models {
		items = append(items, conversationFromModel(model))
	}
	return items, nil
}

// UpdateConversation refreshes title and last-message timestamp.
func (s *GormStore) UpdateConversation(id string, title string, lastMessageAt time.Time) error {
	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if strings.TrimSpace(title) != "" {
		updates["title"] = strings.TrimSpace(title)
	}
	if !lastMessageAt.IsZero() {
		updates["last_message_at"] = lastMessageAt.UTC()
	}
	return s.db.Model(&ConversationModel{}).Where("id = ?", id).Updates(updates).Error
}

// AppendMessage records a message.
func (s *GormStore) AppendMessage(bookID string, msg domain.Message) error {
	model := messageToModel(msg)
	model.BookID = bookID
	return s.db.Create(&model).Error
}

// AppendConversationMessage records a conversation message.
func (s *GormStore) AppendConversationMessage(conversationID string, msg domain.Message) error {
	model := messageToModel(msg)
	model.ConversationID = &conversationID
	model.BookID = msg.BookID
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

// ListConversationMessages returns recent messages for a conversation.
func (s *GormStore) ListConversationMessages(conversationID string, limit int) ([]domain.Message, error) {
	query := s.db.Where("conversation_id = ?", conversationID).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var models []MessageModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	msgs := make([]domain.Message, 0, len(models))
	for _, model := range models {
		msgs = append(msgs, messageFromModel(model))
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
	if err := s.validateEmbeddingDim(embedding); err != nil {
		return err
	}
	return s.db.Model(&ChunkModel{}).Where("id = ?", id).
		Update("embedding", pgvector.NewVector(embedding)).Error
}

// SearchChunks finds similar chunks by cosine distance.
func (s *GormStore) SearchChunks(bookID string, embedding []float32, limit int) ([]domain.Chunk, error) {
	if limit <= 0 {
		return []domain.Chunk{}, nil
	}
	if err := s.validateEmbeddingDim(embedding); err != nil {
		return nil, err
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

func (s *GormStore) validateEmbeddingDim(embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	if s.embeddingDim > 0 && len(embedding) != s.embeddingDim {
		return fmt.Errorf("embedding dimension mismatch: got %d, want %d", len(embedding), s.embeddingDim)
	}
	return nil
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

func conversationToModel(c domain.Conversation) ConversationModel {
	var bookID *string
	if strings.TrimSpace(c.BookID) != "" {
		value := strings.TrimSpace(c.BookID)
		bookID = &value
	}
	return ConversationModel{
		ID:            c.ID,
		UserID:        c.UserID,
		BookID:        bookID,
		Title:         c.Title,
		LastMessageAt: c.LastMessageAt,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

func conversationFromModel(m ConversationModel) domain.Conversation {
	bookID := ""
	if m.BookID != nil {
		bookID = strings.TrimSpace(*m.BookID)
	}
	return domain.Conversation{
		ID:            m.ID,
		UserID:        m.UserID,
		BookID:        bookID,
		Title:         m.Title,
		LastMessageAt: m.LastMessageAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func messageToModel(msg domain.Message) MessageModel {
	var conversationID *string
	if strings.TrimSpace(msg.ConversationID) != "" {
		value := strings.TrimSpace(msg.ConversationID)
		conversationID = &value
	}
	rawSources, _ := json.Marshal(msg.Sources)
	return MessageModel{
		ID:             msg.ID,
		ConversationID: conversationID,
		UserID:         msg.UserID,
		BookID:         msg.BookID,
		Role:           msg.Role,
		Content:        msg.Content,
		Sources:        rawSources,
		CreatedAt:      msg.CreatedAt,
	}
}

func messageFromModel(m MessageModel) domain.Message {
	conversationID := ""
	if m.ConversationID != nil {
		conversationID = strings.TrimSpace(*m.ConversationID)
	}
	var sources []domain.Source
	if len(m.Sources) > 0 {
		_ = json.Unmarshal(m.Sources, &sources)
	}
	return domain.Message{
		ID:             m.ID,
		ConversationID: conversationID,
		UserID:         m.UserID,
		BookID:         m.BookID,
		Role:           m.Role,
		Content:        m.Content,
		Sources:        sources,
		CreatedAt:      m.CreatedAt,
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
