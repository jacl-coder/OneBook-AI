package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"onebookai/internal/util"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type AsyncJobModel struct {
	ID           string `gorm:"primaryKey"`
	JobType      string `gorm:"not null;index"`
	ResourceType string `gorm:"not null;index"`
	ResourceID   string `gorm:"not null;index"`
	Status       string `gorm:"not null;index"`
	Attempts     int    `gorm:"not null;default:0"`
	ErrorMessage string
	PayloadJSON  json.RawMessage `gorm:"type:jsonb"`
	CreatedAt    time.Time       `gorm:"not null;index"`
	UpdatedAt    time.Time       `gorm:"not null;index"`
}

type PostgresJobStore struct {
	db *gorm.DB
}

func NewPostgresJobStore(databaseURL string) (*PostgresJobStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database URL required")
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
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.AutoMigrate(&AsyncJobModel{}); err != nil {
		return nil, fmt.Errorf("auto migrate async jobs: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_async_jobs_active_resource
		ON async_job_models (job_type, resource_type, resource_id)
		WHERE status IN ('queued', 'processing');
	`).Error; err != nil {
		return nil, fmt.Errorf("ensure active async job uniqueness: %w", err)
	}
	return &PostgresJobStore{db: db}, nil
}

func (s *PostgresJobStore) CreateOrGetActiveJob(ctx context.Context, jobType, resourceType, resourceID string, payload json.RawMessage) (JobStatus, bool, error) {
	var existing AsyncJobModel
	err := s.db.WithContext(ctx).
		Where("job_type = ? AND resource_type = ? AND resource_id = ? AND status IN ?", jobType, resourceType, resourceID, []string{StatusQueued, StatusProcessing}).
		Order("created_at DESC").
		First(&existing).Error
	if err == nil {
		return jobFromModel(existing), false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return JobStatus{}, false, err
	}
	now := time.Now().UTC()
	model := AsyncJobModel{
		ID:           util.NewID(),
		JobType:      jobType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Status:       StatusQueued,
		Attempts:     0,
		PayloadJSON:  payload,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		var race AsyncJobModel
		findErr := s.db.WithContext(ctx).
			Where("job_type = ? AND resource_type = ? AND resource_id = ? AND status IN ?", jobType, resourceType, resourceID, []string{StatusQueued, StatusProcessing}).
			Order("created_at DESC").
			First(&race).Error
		if findErr == nil {
			return jobFromModel(race), false, nil
		}
		return JobStatus{}, false, err
	}
	return jobFromModel(model), true, nil
}

func (s *PostgresJobStore) GetJob(ctx context.Context, jobID string) (JobStatus, bool, error) {
	var model AsyncJobModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", strings.TrimSpace(jobID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return JobStatus{}, false, nil
		}
		return JobStatus{}, false, err
	}
	return jobFromModel(model), true, nil
}

func (s *PostgresJobStore) MarkProcessing(ctx context.Context, jobID string) (JobStatus, error) {
	var model AsyncJobModel
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&model, "id = ?", strings.TrimSpace(jobID)).Error; err != nil {
			return err
		}
		if model.Status == StatusDone || model.Status == StatusFailed {
			return ErrJobTerminal
		}
		model.Attempts++
		model.Status = StatusProcessing
		model.UpdatedAt = time.Now().UTC()
		return tx.Save(&model).Error
	})
	if err != nil {
		return JobStatus{}, err
	}
	return jobFromModel(model), nil
}

func (s *PostgresJobStore) MarkQueued(ctx context.Context, jobID, errMsg string) error {
	return s.updateState(ctx, jobID, StatusQueued, errMsg)
}

func (s *PostgresJobStore) MarkDone(ctx context.Context, jobID string) error {
	return s.updateState(ctx, jobID, StatusDone, "")
}

func (s *PostgresJobStore) MarkFailed(ctx context.Context, jobID, errMsg string) error {
	return s.updateState(ctx, jobID, StatusFailed, errMsg)
}

func (s *PostgresJobStore) updateState(ctx context.Context, jobID, status, errMsg string) error {
	return s.db.WithContext(ctx).Model(&AsyncJobModel{}).
		Where("id = ?", strings.TrimSpace(jobID)).
		Updates(map[string]any{
			"status":        status,
			"error_message": errMsg,
			"updated_at":    time.Now().UTC(),
		}).Error
}

func jobFromModel(model AsyncJobModel) JobStatus {
	return JobStatus{
		ID:           model.ID,
		BookID:       model.ResourceID,
		Status:       model.Status,
		ErrorMessage: model.ErrorMessage,
		Attempts:     model.Attempts,
		Payload:      append([]byte(nil), model.PayloadJSON...),
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}
