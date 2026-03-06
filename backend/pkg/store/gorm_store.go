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
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
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
		if err := tx.AutoMigrate(&UserModel{}, &BookModel{}, &ConversationModel{}, &MessageModel{}, &ChunkModel{}, &AdminAuditLogModel{}, &EvalDatasetModel{}, &EvalRunModel{}, &IdempotencyRecordModel{}); err != nil {
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
		if err := tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_admin_audit_target
			ON admin_audit_log_models (target_type, target_id, created_at DESC);
		`).Error; err != nil {
			return fmt.Errorf("ensure admin audit target index: %w", err)
		}
		if err := tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_eval_runs_dataset_status
			ON eval_run_models (dataset_id, status, created_at DESC);
		`).Error; err != nil {
			return fmt.Errorf("ensure eval run index: %w", err)
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

// ListUsersWithOptions returns users with filtering and pagination.
func (s *GormStore) ListUsersWithOptions(opts UserListOptions) ([]domain.User, int, error) {
	page, pageSize := normalizePage(opts.Page, opts.PageSize)
	tx := s.db.Model(&UserModel{})
	query := strings.TrimSpace(opts.Query)
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		tx = tx.Where("LOWER(email) LIKE ? OR LOWER(id) LIKE ?", like, like)
	}
	role := strings.TrimSpace(strings.ToLower(opts.Role))
	if role != "" {
		tx = tx.Where("role = ?", role)
	}
	status := strings.TrimSpace(strings.ToLower(opts.Status))
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sortBy := normalizeUserSortBy(opts.SortBy)
	sortOrder := normalizeSortOrder(opts.SortOrder)
	var models []UserModel
	if err := tx.Order(sortBy + " " + sortOrder).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.User, 0, len(models))
	for _, model := range models {
		items = append(items, userFromModel(model))
	}
	return items, int(total), nil
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
		DoUpdates: clause.AssignmentColumns([]string{"owner_id", "title", "original_filename", "primary_category", "tags", "format", "language", "storage_key", "status", "error_message", "size_bytes", "updated_at", "deleted_at", "cleanup_status", "cleanup_error", "cleanup_attempts", "cleanup_updated_at"}),
	}).Create(&model).Error
}

// SetStatus updates book status/error.
func (s *GormStore) SetStatus(id string, status domain.BookStatus, errMsg string) error {
	return s.db.Model(&BookModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
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

// ListBooksWithOptions returns books with filtering and pagination.
func (s *GormStore) ListBooksWithOptions(opts BookListOptions) ([]domain.Book, int, error) {
	tx := s.db.Model(&BookModel{})
	tx = tx.Where("deleted_at IS NULL")
	query := strings.TrimSpace(opts.Query)
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(original_filename) LIKE ? OR LOWER(id) LIKE ?", like, like, like)
	}
	ownerID := strings.TrimSpace(opts.OwnerID)
	if ownerID != "" {
		tx = tx.Where("owner_id = ?", ownerID)
	}
	status := strings.TrimSpace(strings.ToLower(opts.Status))
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	category := strings.TrimSpace(strings.ToLower(opts.PrimaryCategory))
	if category != "" {
		tx = tx.Where("primary_category = ?", category)
	}
	tag := strings.TrimSpace(opts.Tag)
	if tag != "" {
		tx = tx.Where("tags @> ?", datatypes.JSON([]byte(fmt.Sprintf("[%q]", tag))))
	}
	format := strings.TrimSpace(strings.ToLower(opts.Format))
	if format != "" {
		tx = tx.Where("format = ?", format)
	}
	language := strings.TrimSpace(strings.ToLower(opts.Language))
	if language != "" {
		tx = tx.Where("language = ?", language)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	sortBy := normalizeBookSortBy(opts.SortBy)
	sortOrder := normalizeSortOrder(opts.SortOrder)
	var models []BookModel
	queryTx := tx.Order(sortBy + " " + sortOrder)
	if opts.PageSize > 0 {
		page, pageSize := normalizePage(opts.Page, opts.PageSize)
		queryTx = queryTx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := queryTx.Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.Book, 0, len(models))
	for _, model := range models {
		items = append(items, bookFromModel(model))
	}
	return items, int(total), nil
}

func (s *GormStore) listBooks(order string, conds ...any) ([]domain.Book, error) {
	var models []BookModel
	tx := s.db.Order(order)
	tx = tx.Where("deleted_at IS NULL")
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
	if err := s.db.Where("deleted_at IS NULL").First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Book{}, false, nil
		}
		return domain.Book{}, false, err
	}
	return bookFromModel(model), true, nil
}

func (s *GormStore) GetBookIncludingDeleted(id string) (domain.Book, bool, error) {
	var model BookModel
	if err := s.db.First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.Book{}, false, nil
		}
		return domain.Book{}, false, err
	}
	return bookFromModel(model), true, nil
}

func (s *GormStore) ListBooksPendingCleanup(limit int) ([]domain.Book, error) {
	if limit <= 0 {
		limit = 20
	}
	var models []BookModel
	if err := s.db.Where("deleted_at IS NOT NULL AND cleanup_status IN ?", []string{string(domain.BookCleanupStatusQueued), string(domain.BookCleanupStatusFailed)}).
		Order("cleanup_updated_at ASC NULLS FIRST").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]domain.Book, 0, len(models))
	for _, model := range models {
		items = append(items, bookFromModel(model))
	}
	return items, nil
}

func (s *GormStore) MarkBookDeleted(id string, cleanupStatus domain.BookCleanupStatus) error {
	now := time.Now().UTC()
	return s.db.Model(&BookModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at":         &now,
			"cleanup_status":     string(cleanupStatus),
			"cleanup_error":      "",
			"cleanup_updated_at": &now,
			"updated_at":         now,
		}).Error
}

func (s *GormStore) UpdateBookCleanup(id string, status domain.BookCleanupStatus, errMsg string, incrementAttempts bool) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"cleanup_status":     string(status),
		"cleanup_error":      strings.TrimSpace(errMsg),
		"cleanup_updated_at": &now,
		"updated_at":         now,
	}
	if incrementAttempts {
		return s.db.Model(&BookModel{}).
			Where("id = ?", id).
			Updates(updates).
			UpdateColumn("cleanup_attempts", gorm.Expr("cleanup_attempts + 1")).Error
	}
	return s.db.Model(&BookModel{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteBook removes book and messages (chunks handled by FK cascade).
func (s *GormStore) DeleteBook(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&ChunkModel{}, "book_id = ?", id).Error; err != nil {
			return err
		}
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
		language := summarizeBookLanguage(chunks)
		if err := tx.Model(&BookModel{}).
			Where("id = ?", bookID).
			Updates(map[string]any{
				"language":   string(language),
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
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

// SaveAdminAuditLog persists an admin audit event.
func (s *GormStore) SaveAdminAuditLog(entry domain.AdminAuditLog) error {
	model, err := adminAuditLogToModel(entry)
	if err != nil {
		return err
	}
	return s.db.Create(&model).Error
}

// ListAdminAuditLogs returns paginated admin audit logs.
func (s *GormStore) ListAdminAuditLogs(opts AdminAuditLogListOptions) ([]domain.AdminAuditLog, int, error) {
	page, pageSize := normalizePage(opts.Page, opts.PageSize)
	tx := s.db.Model(&AdminAuditLogModel{})
	if actorID := strings.TrimSpace(opts.ActorID); actorID != "" {
		tx = tx.Where("actor_id = ?", actorID)
	}
	if action := strings.TrimSpace(opts.Action); action != "" {
		tx = tx.Where("action = ?", action)
	}
	if targetType := strings.TrimSpace(opts.TargetType); targetType != "" {
		tx = tx.Where("target_type = ?", targetType)
	}
	if targetID := strings.TrimSpace(opts.TargetID); targetID != "" {
		tx = tx.Where("target_id = ?", targetID)
	}
	if !opts.From.IsZero() {
		tx = tx.Where("created_at >= ?", opts.From.UTC())
	}
	if !opts.To.IsZero() {
		tx = tx.Where("created_at <= ?", opts.To.UTC())
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []AdminAuditLogModel
	if err := tx.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.AdminAuditLog, 0, len(models))
	for _, model := range models {
		entry, err := adminAuditLogFromModel(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, entry)
	}
	return items, int(total), nil
}

// GetAdminOverview returns aggregate metrics for admin dashboard.
func (s *GormStore) GetAdminOverview(windowStart time.Time, windowHours int) (domain.AdminOverview, error) {
	if windowHours <= 0 {
		windowHours = 24
	}
	start := windowStart.UTC()
	if start.IsZero() {
		start = time.Now().UTC().Add(-time.Duration(windowHours) * time.Hour)
	}
	var totalUsers, activeUsers, disabledUsers int64
	if err := s.db.Model(&UserModel{}).Count(&totalUsers).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	if err := s.db.Model(&UserModel{}).Where("status = ?", string(domain.StatusActive)).Count(&activeUsers).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	if err := s.db.Model(&UserModel{}).Where("status = ?", string(domain.StatusDisabled)).Count(&disabledUsers).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	var totalBooks, booksCreated24h, booksFailed24h int64
	if err := s.db.Model(&BookModel{}).Where("deleted_at IS NULL").Count(&totalBooks).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	if err := s.db.Model(&BookModel{}).Where("deleted_at IS NULL AND created_at >= ?", start).Count(&booksCreated24h).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	if err := s.db.Model(&BookModel{}).
		Where("deleted_at IS NULL AND status = ? AND updated_at >= ?", string(domain.StatusFailed), start).
		Count(&booksFailed24h).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	var grouped []struct {
		Status string
		Count  int64
	}
	if err := s.db.Model(&BookModel{}).
		Where("deleted_at IS NULL").
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&grouped).Error; err != nil {
		return domain.AdminOverview{}, err
	}
	byStatus := make([]domain.BookStatusCount, 0, len(grouped))
	for _, row := range grouped {
		byStatus = append(byStatus, domain.BookStatusCount{
			Status: row.Status,
			Count:  int(row.Count),
		})
	}
	return domain.AdminOverview{
		TotalUsers:      int(totalUsers),
		ActiveUsers:     int(activeUsers),
		DisabledUsers:   int(disabledUsers),
		TotalBooks:      int(totalBooks),
		BooksByStatus:   byStatus,
		BooksCreated24h: int(booksCreated24h),
		BooksFailed24h:  int(booksFailed24h),
		RefreshedAt:     time.Now().UTC(),
		WindowStart:     start,
		WindowHours:     windowHours,
	}, nil
}

// SaveEvalDataset persists an eval dataset.
func (s *GormStore) SaveEvalDataset(dataset domain.EvalDataset) error {
	model, err := evalDatasetToModel(dataset)
	if err != nil {
		return err
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "source_type", "book_id", "version", "status", "description", "files", "created_by", "updated_at",
		}),
	}).Create(&model).Error
}

// GetEvalDataset fetches a dataset by ID.
func (s *GormStore) GetEvalDataset(id string) (domain.EvalDataset, bool, error) {
	var model EvalDatasetModel
	if err := s.db.First(&model, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.EvalDataset{}, false, nil
		}
		return domain.EvalDataset{}, false, err
	}
	item, err := evalDatasetFromModel(model)
	if err != nil {
		return domain.EvalDataset{}, false, err
	}
	return item, true, nil
}

// ListEvalDatasets lists eval datasets with filtering.
func (s *GormStore) ListEvalDatasets(opts EvalDatasetListOptions) ([]domain.EvalDataset, int, error) {
	page, pageSize := normalizePage(opts.Page, opts.PageSize)
	tx := s.db.Model(&EvalDatasetModel{})
	if query := strings.TrimSpace(strings.ToLower(opts.Query)); query != "" {
		like := "%" + query + "%"
		tx = tx.Where("LOWER(name) LIKE ? OR LOWER(id) LIKE ?", like, like)
	}
	if sourceType := strings.TrimSpace(strings.ToLower(opts.SourceType)); sourceType != "" {
		tx = tx.Where("source_type = ?", sourceType)
	}
	if status := strings.TrimSpace(strings.ToLower(opts.Status)); status != "" {
		tx = tx.Where("status = ?", status)
	}
	if bookID := strings.TrimSpace(opts.BookID); bookID != "" {
		tx = tx.Where("book_id = ?", bookID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []EvalDatasetModel
	if err := tx.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.EvalDataset, 0, len(models))
	for _, model := range models {
		item, err := evalDatasetFromModel(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, int(total), nil
}

// DeleteEvalDataset hard-deletes a dataset.
func (s *GormStore) DeleteEvalDataset(id string) error {
	return s.db.Delete(&EvalDatasetModel{}, "id = ?", strings.TrimSpace(id)).Error
}

// ArchiveEvalDataset marks a dataset archived.
func (s *GormStore) ArchiveEvalDataset(id string) error {
	return s.db.Model(&EvalDatasetModel{}).
		Where("id = ?", strings.TrimSpace(id)).
		Updates(map[string]any{
			"status":     string(domain.EvalDatasetStatusArchived),
			"updated_at": time.Now().UTC(),
		}).Error
}

// SaveEvalRun persists an eval run.
func (s *GormStore) SaveEvalRun(run domain.EvalRun) error {
	model, err := evalRunToModel(run)
	if err != nil {
		return err
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"dataset_id", "fingerprint", "status", "mode", "retrieval_mode", "params", "gate_mode", "gate_status",
			"summary_metrics", "warnings", "artifacts", "stage_summaries", "progress", "error_message",
			"started_at", "finished_at", "created_by", "updated_at",
		}),
	}).Create(&model).Error
}

// GetEvalRun fetches an eval run by ID.
func (s *GormStore) GetEvalRun(id string) (domain.EvalRun, bool, error) {
	var model EvalRunModel
	if err := s.db.First(&model, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.EvalRun{}, false, nil
		}
		return domain.EvalRun{}, false, err
	}
	item, err := evalRunFromModel(model)
	if err != nil {
		return domain.EvalRun{}, false, err
	}
	return item, true, nil
}

func (s *GormStore) GetActiveEvalRunByFingerprint(fingerprint string) (domain.EvalRun, bool, error) {
	var model EvalRunModel
	if err := s.db.Where("fingerprint = ? AND status IN ?", strings.TrimSpace(fingerprint), []string{string(domain.EvalRunStatusQueued), string(domain.EvalRunStatusRunning)}).
		Order("created_at DESC").
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.EvalRun{}, false, nil
		}
		return domain.EvalRun{}, false, err
	}
	item, err := evalRunFromModel(model)
	if err != nil {
		return domain.EvalRun{}, false, err
	}
	return item, true, nil
}

// ListEvalRuns lists eval runs with filtering.
func (s *GormStore) ListEvalRuns(opts EvalRunListOptions) ([]domain.EvalRun, int, error) {
	page, pageSize := normalizePage(opts.Page, opts.PageSize)
	tx := s.db.Model(&EvalRunModel{})
	if datasetID := strings.TrimSpace(opts.DatasetID); datasetID != "" {
		tx = tx.Where("dataset_id = ?", datasetID)
	}
	if status := strings.TrimSpace(strings.ToLower(opts.Status)); status != "" {
		tx = tx.Where("status = ?", status)
	}
	if mode := strings.TrimSpace(strings.ToLower(opts.Mode)); mode != "" {
		tx = tx.Where("mode = ?", mode)
	}
	if retrievalMode := strings.TrimSpace(strings.ToLower(opts.RetrievalMode)); retrievalMode != "" {
		tx = tx.Where("retrieval_mode = ?", retrievalMode)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []EvalRunModel
	if err := tx.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.EvalRun, 0, len(models))
	for _, model := range models {
		item, err := evalRunFromModel(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, int(total), nil
}

// CountEvalRunsByDataset counts runs for a dataset.
func (s *GormStore) CountEvalRunsByDataset(datasetID string) (int, error) {
	var count int64
	if err := s.db.Model(&EvalRunModel{}).
		Where("dataset_id = ?", strings.TrimSpace(datasetID)).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetAdminEvalOverview returns aggregate eval dashboard metrics.
func (s *GormStore) GetAdminEvalOverview(windowStart time.Time) (domain.AdminEvalOverview, error) {
	start := windowStart.UTC()
	if start.IsZero() {
		start = time.Now().UTC().Add(-24 * time.Hour)
	}
	countStatus := func(model any, field, value string) (int64, error) {
		var count int64
		tx := s.db.Model(model)
		if strings.TrimSpace(field) != "" {
			tx = tx.Where(field+" = ?", value)
		}
		if err := tx.Count(&count).Error; err != nil {
			return 0, err
		}
		return count, nil
	}
	totalDatasets, err := countStatus(&EvalDatasetModel{}, "", "")
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	activeDatasets, err := countStatus(&EvalDatasetModel{}, "status", string(domain.EvalDatasetStatusActive))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	totalRuns, err := countStatus(&EvalRunModel{}, "", "")
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	queuedRuns, err := countStatus(&EvalRunModel{}, "status", string(domain.EvalRunStatusQueued))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	runningRuns, err := countStatus(&EvalRunModel{}, "status", string(domain.EvalRunStatusRunning))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	successfulRuns, err := countStatus(&EvalRunModel{}, "status", string(domain.EvalRunStatusSucceeded))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	failedRuns, err := countStatus(&EvalRunModel{}, "status", string(domain.EvalRunStatusFailed))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	canceledRuns, err := countStatus(&EvalRunModel{}, "status", string(domain.EvalRunStatusCanceled))
	if err != nil {
		return domain.AdminEvalOverview{}, err
	}
	var recentRuns, recentGateFailed int64
	if err := s.db.Model(&EvalRunModel{}).Where("created_at >= ?", start).Count(&recentRuns).Error; err != nil {
		return domain.AdminEvalOverview{}, err
	}
	if err := s.db.Model(&EvalRunModel{}).Where("created_at >= ? AND gate_status = ?", start, string(domain.EvalGateStatusFailed)).Count(&recentGateFailed).Error; err != nil {
		return domain.AdminEvalOverview{}, err
	}
	return domain.AdminEvalOverview{
		TotalDatasets:    int(totalDatasets),
		ActiveDatasets:   int(activeDatasets),
		TotalRuns:        int(totalRuns),
		QueuedRuns:       int(queuedRuns),
		RunningRuns:      int(runningRuns),
		SuccessfulRuns:   int(successfulRuns),
		FailedRuns:       int(failedRuns),
		CanceledRuns:     int(canceledRuns),
		RecentRuns:       int(recentRuns),
		RecentGateFailed: int(recentGateFailed),
		SuccessRate:      safeDivFloat(float64(successfulRuns), float64(totalRuns)),
		RefreshedAt:      time.Now().UTC(),
	}, nil
}

func (s *GormStore) SaveIdempotencyRecord(record domain.IdempotencyRecord) error {
	model := idempotencyRecordToModel(record)
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"scope", "actor_id", "idempotency_key", "request_hash", "resource_type", "resource_id", "status_code", "state", "updated_at",
		}),
	}).Create(&model).Error
}

func (s *GormStore) GetIdempotencyRecord(scope, actorID, key string) (domain.IdempotencyRecord, bool, error) {
	var model IdempotencyRecordModel
	if err := s.db.Where("scope = ? AND actor_id = ? AND idempotency_key = ?", strings.TrimSpace(scope), strings.TrimSpace(actorID), strings.TrimSpace(key)).
		First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.IdempotencyRecord{}, false, nil
		}
		return domain.IdempotencyRecord{}, false, err
	}
	return idempotencyRecordFromModel(model), true, nil
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

func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func normalizeSortOrder(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "asc":
		return "ASC"
	default:
		return "DESC"
	}
}

func safeDivFloat(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func normalizeUserSortBy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "email":
		return "email"
	case "updatedat", "updated_at":
		return "updated_at"
	default:
		return "created_at"
	}
}

func normalizeBookSortBy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "title":
		return "title"
	case "createdat", "created_at":
		return "created_at"
	default:
		return "updated_at"
	}
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
	tags, _ := marshalStringSliceJSON(b.Tags)
	return BookModel{
		ID:               b.ID,
		OwnerID:          b.OwnerID,
		Title:            b.Title,
		OriginalFilename: b.OriginalFilename,
		PrimaryCategory:  string(domain.NormalizeBookPrimaryCategory(b.PrimaryCategory)),
		Tags:             tags,
		Format:           string(domain.NormalizeBookFormat(b.Format)),
		Language:         string(domain.NormalizeBookLanguage(b.Language)),
		StorageKey:       b.StorageKey,
		Status:           string(b.Status),
		ErrorMessage:     b.ErrorMessage,
		SizeBytes:        b.SizeBytes,
		CreatedAt:        b.CreatedAt,
		UpdatedAt:        b.UpdatedAt,
		DeletedAt:        normalizeTimePtr(b.DeletedAt),
		CleanupStatus:    strings.TrimSpace(b.CleanupStatus),
		CleanupError:     strings.TrimSpace(b.CleanupError),
		CleanupAttempts:  b.CleanupAttempts,
		CleanupUpdatedAt: normalizeTimePtr(b.CleanupUpdatedAt),
	}
}

func bookFromModel(m BookModel) domain.Book {
	tags, _ := unmarshalStringSliceJSON(m.Tags)
	return domain.Book{
		ID:               m.ID,
		OwnerID:          m.OwnerID,
		Title:            m.Title,
		OriginalFilename: m.OriginalFilename,
		PrimaryCategory:  string(domain.NormalizeBookPrimaryCategory(m.PrimaryCategory)),
		Tags:             tags,
		Format:           string(domain.NormalizeBookFormat(m.Format)),
		Language:         string(domain.NormalizeBookLanguage(m.Language)),
		StorageKey:       m.StorageKey,
		Status:           domain.BookStatus(m.Status),
		ErrorMessage:     m.ErrorMessage,
		SizeBytes:        m.SizeBytes,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		DeletedAt:        m.DeletedAt,
		CleanupStatus:    m.CleanupStatus,
		CleanupError:     m.CleanupError,
		CleanupAttempts:  m.CleanupAttempts,
		CleanupUpdatedAt: m.CleanupUpdatedAt,
	}
}

func marshalStringSliceJSON(values []string) (datatypes.JSON, error) {
	if len(values) == 0 {
		return datatypes.JSON([]byte("[]")), nil
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(payload), nil
}

func unmarshalStringSliceJSON(raw datatypes.JSON) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func summarizeBookLanguage(chunks []domain.Chunk) domain.BookLanguage {
	if len(chunks) == 0 {
		return domain.BookLanguageUnknown
	}
	counts := map[domain.BookLanguage]int{}
	for _, chunk := range chunks {
		language := domain.NormalizeBookLanguage(chunk.Metadata["language"])
		if language == domain.BookLanguageUnknown {
			language = domain.NormalizeBookLanguage(retrieval.DetectLanguage(chunk.Content))
		}
		counts[language]++
	}
	best := domain.BookLanguageUnknown
	bestCount := -1
	for _, language := range []domain.BookLanguage{domain.BookLanguageZH, domain.BookLanguageEN, domain.BookLanguageOther, domain.BookLanguageUnknown} {
		if counts[language] > bestCount {
			best = language
			bestCount = counts[language]
		}
	}
	return best
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

func adminAuditLogToModel(entry domain.AdminAuditLog) (AdminAuditLogModel, error) {
	before, err := marshalOptionalJSON(entry.Before)
	if err != nil {
		return AdminAuditLogModel{}, fmt.Errorf("marshal admin audit before: %w", err)
	}
	after, err := marshalOptionalJSON(entry.After)
	if err != nil {
		return AdminAuditLogModel{}, fmt.Errorf("marshal admin audit after: %w", err)
	}
	return AdminAuditLogModel{
		ID:         strings.TrimSpace(entry.ID),
		ActorID:    strings.TrimSpace(entry.ActorID),
		Action:     strings.TrimSpace(entry.Action),
		TargetType: strings.TrimSpace(entry.TargetType),
		TargetID:   strings.TrimSpace(entry.TargetID),
		Before:     before,
		After:      after,
		RequestID:  strings.TrimSpace(entry.RequestID),
		IP:         strings.TrimSpace(entry.IP),
		UserAgent:  strings.TrimSpace(entry.UserAgent),
		CreatedAt:  entry.CreatedAt.UTC(),
	}, nil
}

func adminAuditLogFromModel(model AdminAuditLogModel) (domain.AdminAuditLog, error) {
	before, err := unmarshalOptionalJSON(model.Before)
	if err != nil {
		return domain.AdminAuditLog{}, fmt.Errorf("unmarshal admin audit before: %w", err)
	}
	after, err := unmarshalOptionalJSON(model.After)
	if err != nil {
		return domain.AdminAuditLog{}, fmt.Errorf("unmarshal admin audit after: %w", err)
	}
	return domain.AdminAuditLog{
		ID:         model.ID,
		ActorID:    model.ActorID,
		Action:     model.Action,
		TargetType: model.TargetType,
		TargetID:   model.TargetID,
		Before:     before,
		After:      after,
		RequestID:  model.RequestID,
		IP:         model.IP,
		UserAgent:  model.UserAgent,
		CreatedAt:  model.CreatedAt,
	}, nil
}

func evalDatasetToModel(dataset domain.EvalDataset) (EvalDatasetModel, error) {
	files, err := marshalStringMap(dataset.Files)
	if err != nil {
		return EvalDatasetModel{}, fmt.Errorf("marshal eval dataset files: %w", err)
	}
	var bookID *string
	if value := strings.TrimSpace(dataset.BookID); value != "" {
		bookID = &value
	}
	return EvalDatasetModel{
		ID:          strings.TrimSpace(dataset.ID),
		Name:        strings.TrimSpace(dataset.Name),
		SourceType:  strings.TrimSpace(string(dataset.SourceType)),
		BookID:      bookID,
		Version:     dataset.Version,
		Status:      strings.TrimSpace(string(dataset.Status)),
		Description: strings.TrimSpace(dataset.Description),
		Files:       files,
		CreatedBy:   strings.TrimSpace(dataset.CreatedBy),
		CreatedAt:   dataset.CreatedAt.UTC(),
		UpdatedAt:   dataset.UpdatedAt.UTC(),
	}, nil
}

func evalDatasetFromModel(model EvalDatasetModel) (domain.EvalDataset, error) {
	files, err := unmarshalStringMap(model.Files)
	if err != nil {
		return domain.EvalDataset{}, fmt.Errorf("unmarshal eval dataset files: %w", err)
	}
	bookID := ""
	if model.BookID != nil {
		bookID = strings.TrimSpace(*model.BookID)
	}
	return domain.EvalDataset{
		ID:          model.ID,
		Name:        model.Name,
		SourceType:  domain.EvalDatasetSourceType(model.SourceType),
		BookID:      bookID,
		Version:     model.Version,
		Status:      domain.EvalDatasetStatus(model.Status),
		Description: model.Description,
		Files:       files,
		CreatedBy:   model.CreatedBy,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func evalRunToModel(run domain.EvalRun) (EvalRunModel, error) {
	params, err := marshalOptionalJSON(run.Params)
	if err != nil {
		return EvalRunModel{}, fmt.Errorf("marshal eval run params: %w", err)
	}
	summaryMetrics, err := marshalOptionalJSON(run.SummaryMetrics)
	if err != nil {
		return EvalRunModel{}, fmt.Errorf("marshal eval run metrics: %w", err)
	}
	warnings, err := marshalStringSlice(run.Warnings)
	if err != nil {
		return EvalRunModel{}, fmt.Errorf("marshal eval run warnings: %w", err)
	}
	artifacts, err := marshalAnyJSON(run.Artifacts)
	if err != nil {
		return EvalRunModel{}, fmt.Errorf("marshal eval run artifacts: %w", err)
	}
	stageSummaries, err := marshalAnyJSON(run.StageSummaries)
	if err != nil {
		return EvalRunModel{}, fmt.Errorf("marshal eval run stage summaries: %w", err)
	}
	return EvalRunModel{
		ID:             strings.TrimSpace(run.ID),
		DatasetID:      strings.TrimSpace(run.DatasetID),
		Fingerprint:    strings.TrimSpace(run.Fingerprint),
		Status:         strings.TrimSpace(string(run.Status)),
		Mode:           strings.TrimSpace(string(run.Mode)),
		RetrievalMode:  strings.TrimSpace(string(run.RetrievalMode)),
		Params:         params,
		GateMode:       strings.TrimSpace(run.GateMode),
		GateStatus:     strings.TrimSpace(string(run.GateStatus)),
		SummaryMetrics: summaryMetrics,
		Warnings:       warnings,
		Artifacts:      artifacts,
		StageSummaries: stageSummaries,
		Progress:       run.Progress,
		ErrorMessage:   strings.TrimSpace(run.ErrorMessage),
		StartedAt:      normalizeTimePtr(run.StartedAt),
		FinishedAt:     normalizeTimePtr(run.FinishedAt),
		CreatedBy:      strings.TrimSpace(run.CreatedBy),
		CreatedAt:      run.CreatedAt.UTC(),
		UpdatedAt:      run.UpdatedAt.UTC(),
	}, nil
}

func evalRunFromModel(model EvalRunModel) (domain.EvalRun, error) {
	params, err := unmarshalOptionalJSON(model.Params)
	if err != nil {
		return domain.EvalRun{}, fmt.Errorf("unmarshal eval run params: %w", err)
	}
	summaryMetrics, err := unmarshalOptionalJSON(model.SummaryMetrics)
	if err != nil {
		return domain.EvalRun{}, fmt.Errorf("unmarshal eval run metrics: %w", err)
	}
	warnings, err := unmarshalStringSlice(model.Warnings)
	if err != nil {
		return domain.EvalRun{}, fmt.Errorf("unmarshal eval run warnings: %w", err)
	}
	var artifacts []domain.EvalRunArtifact
	if err := unmarshalAnyJSON(model.Artifacts, &artifacts); err != nil {
		return domain.EvalRun{}, fmt.Errorf("unmarshal eval run artifacts: %w", err)
	}
	var stageSummaries []domain.EvalRunStageSummary
	if err := unmarshalAnyJSON(model.StageSummaries, &stageSummaries); err != nil {
		return domain.EvalRun{}, fmt.Errorf("unmarshal eval run stage summaries: %w", err)
	}
	return domain.EvalRun{
		ID:             model.ID,
		DatasetID:      model.DatasetID,
		Fingerprint:    model.Fingerprint,
		Status:         domain.EvalRunStatus(model.Status),
		Mode:           domain.EvalRunMode(model.Mode),
		RetrievalMode:  domain.EvalRetrievalMode(model.RetrievalMode),
		Params:         params,
		GateMode:       model.GateMode,
		GateStatus:     domain.EvalGateStatus(model.GateStatus),
		SummaryMetrics: summaryMetrics,
		Warnings:       warnings,
		Artifacts:      artifacts,
		StageSummaries: stageSummaries,
		Progress:       model.Progress,
		ErrorMessage:   model.ErrorMessage,
		StartedAt:      model.StartedAt,
		FinishedAt:     model.FinishedAt,
		CreatedBy:      model.CreatedBy,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}, nil
}

func idempotencyRecordToModel(record domain.IdempotencyRecord) IdempotencyRecordModel {
	return IdempotencyRecordModel{
		ID:             strings.TrimSpace(record.ID),
		Scope:          strings.TrimSpace(record.Scope),
		ActorID:        strings.TrimSpace(record.ActorID),
		IdempotencyKey: strings.TrimSpace(record.IdempotencyKey),
		RequestHash:    strings.TrimSpace(record.RequestHash),
		ResourceType:   strings.TrimSpace(record.ResourceType),
		ResourceID:     strings.TrimSpace(record.ResourceID),
		StatusCode:     record.StatusCode,
		State:          strings.TrimSpace(string(record.State)),
		CreatedAt:      record.CreatedAt.UTC(),
		UpdatedAt:      record.UpdatedAt.UTC(),
	}
}

func idempotencyRecordFromModel(model IdempotencyRecordModel) domain.IdempotencyRecord {
	return domain.IdempotencyRecord{
		ID:             model.ID,
		Scope:          model.Scope,
		ActorID:        model.ActorID,
		IdempotencyKey: model.IdempotencyKey,
		RequestHash:    model.RequestHash,
		ResourceType:   model.ResourceType,
		ResourceID:     model.ResourceID,
		StatusCode:     model.StatusCode,
		State:          domain.IdempotencyState(model.State),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func marshalOptionalJSON(input map[string]any) (datatypes.JSON, error) {
	if len(input) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
}

func unmarshalOptionalJSON(raw datatypes.JSON) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func marshalStringMap(input map[string]string) (datatypes.JSON, error) {
	if len(input) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
}

func unmarshalStringMap(raw datatypes.JSON) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func marshalStringSlice(input []string) (datatypes.JSON, error) {
	if len(input) == 0 {
		return datatypes.JSON([]byte("[]")), nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
}

func unmarshalStringSlice(raw datatypes.JSON) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func marshalAnyJSON(input any) (datatypes.JSON, error) {
	if input == nil {
		return datatypes.JSON([]byte("[]")), nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(raw), nil
}

func unmarshalAnyJSON(raw datatypes.JSON, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func normalizeTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}
