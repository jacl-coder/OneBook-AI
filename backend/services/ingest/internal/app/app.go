package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/queue"
	"onebookai/pkg/store"
)

// Status represents the lifecycle of an ingest job.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Job tracks an ingest request.
type Job struct {
	ID           string    `json:"id"`
	BookID       string    `json:"bookId"`
	Status       Status    `json:"status"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Config holds runtime configuration.
type Config struct {
	DatabaseURL            string
	Store                  store.Store
	BookServiceURL         string
	IndexerURL             string
	InternalToken          string
	RedisAddr              string
	RedisPassword          string
	QueueName              string
	QueueGroup             string
	QueueConcurrency       int
	QueueMaxRetries        int
	QueueRetryDelaySeconds int
	ChunkSize              int
	ChunkOverlap           int
}

// App processes ingest jobs.
type App struct {
	store        store.Store
	bookClient   *bookClient
	indexClient  *indexerClient
	queue        *queue.RedisJobQueue
	chunkSize    int
	chunkOverlap int
	httpClient   *http.Client
}

// New constructs the ingest service with persistence.
func New(cfg Config) (*App, error) {
	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required")
		}
		var err error
		dataStore, err = store.NewGormStore(cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("init postgres store: %w", err)
		}
	}
	if cfg.BookServiceURL == "" {
		return nil, fmt.Errorf("book service URL required")
	}
	if cfg.IndexerURL == "" {
		return nil, fmt.Errorf("indexer URL required")
	}
	if cfg.InternalToken == "" {
		return nil, fmt.Errorf("internal token required")
	}
	chunkSize := cfg.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 800
	}
	chunkOverlap := cfg.ChunkOverlap
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	q, err := queue.NewRedisJobQueue(queue.RedisQueueConfig{
		Addr:       cfg.RedisAddr,
		Password:   cfg.RedisPassword,
		Stream:     defaultQueueName(cfg.QueueName),
		Group:      defaultQueueGroup(cfg.QueueGroup),
		Consumer:   util.NewID(),
		MaxRetries: cfg.QueueMaxRetries,
		RetryDelay: time.Duration(cfg.QueueRetryDelaySeconds) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	app := &App{
		store:        dataStore,
		bookClient:   newBookClient(cfg.BookServiceURL, cfg.InternalToken),
		indexClient:  newIndexerClient(cfg.IndexerURL, cfg.InternalToken),
		queue:        q,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
	}
	app.startWorkers(cfg.QueueConcurrency)
	return app, nil
}

// Enqueue registers a new ingest job and begins processing.
func (a *App) Enqueue(bookID string) (Job, error) {
	if strings.TrimSpace(bookID) == "" {
		return Job{}, fmt.Errorf("bookId required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	status, err := a.queue.Enqueue(ctx, bookID)
	if err != nil {
		return Job{}, err
	}
	return jobFromStatus(status), nil
}

// GetJob returns a job by ID.
func (a *App) GetJob(id string) (Job, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	status, ok, err := a.queue.GetJob(ctx, id)
	if err != nil || !ok {
		return Job{}, false
	}
	return jobFromStatus(status), true
}

func (a *App) process(ctx context.Context, job queue.JobStatus) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusProcessing, ""); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	fileInfo, err := a.bookClient.FetchFile(ctx, job.BookID)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	tempPath, err := a.downloadFile(ctx, fileInfo.URL, fileInfo.Filename)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	defer os.Remove(tempPath)

	chunks, err := a.parseAndChunk(fileInfo.Filename, tempPath)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if len(chunks) == 0 {
		err := fmt.Errorf("no content extracted")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	now := time.Now().UTC()
	domainChunks := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		domainChunks = append(domainChunks, domain.Chunk{
			ID:        util.NewID(),
			BookID:    job.BookID,
			Content:   chunk.Content,
			Metadata:  chunk.Metadata,
			CreatedAt: now,
		})
	}
	if err := a.store.ReplaceChunks(job.BookID, domainChunks); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.indexClient.Enqueue(ctx, job.BookID); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	return nil
}

func (a *App) startWorkers(concurrency int) {
	ctx := context.Background()
	a.queue.Start(ctx, concurrency, a.process)
}

func jobFromStatus(status queue.JobStatus) Job {
	return Job{
		ID:           status.ID,
		BookID:       status.BookID,
		Status:       Status(status.Status),
		ErrorMessage: status.ErrorMessage,
		CreatedAt:    status.CreatedAt,
		UpdatedAt:    status.UpdatedAt,
	}
}

func defaultQueueName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "onebook:ingest"
	}
	return name
}

func defaultQueueGroup(name string) string {
	if strings.TrimSpace(name) == "" {
		return "ingest"
	}
	return name
}

func (a *App) downloadFile(ctx context.Context, url string, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	ext := filepath.Ext(filename)
	tmpFile, err := os.CreateTemp("", "onebook-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}
