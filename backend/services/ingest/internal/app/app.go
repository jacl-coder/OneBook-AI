package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
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
	DatabaseURL   string
	Store         store.Store
	BookServiceURL string
	IndexerURL     string
	InternalToken  string
	ChunkSize      int
	ChunkOverlap   int
}

// App processes ingest jobs.
type App struct {
	mu          sync.Mutex
	jobs        map[string]Job
	store       store.Store
	bookClient  *bookClient
	indexClient *indexerClient
	chunkSize   int
	chunkOverlap int
	httpClient  *http.Client
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
	return &App{
		jobs:         make(map[string]Job),
		store:        dataStore,
		bookClient:   newBookClient(cfg.BookServiceURL, cfg.InternalToken),
		indexClient:  newIndexerClient(cfg.IndexerURL, cfg.InternalToken),
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Enqueue registers a new ingest job and begins processing.
func (a *App) Enqueue(bookID string) (Job, error) {
	if strings.TrimSpace(bookID) == "" {
		return Job{}, fmt.Errorf("bookId required")
	}
	now := time.Now().UTC()
	job := Job{
		ID:        util.NewID(),
		BookID:    bookID,
		Status:    StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	a.mu.Lock()
	a.jobs[job.ID] = job
	a.mu.Unlock()

	go a.process(job.ID)
	return job, nil
}

// GetJob returns a job by ID.
func (a *App) GetJob(id string) (Job, bool) {
	a.mu.Lock()
	job, ok := a.jobs[id]
	a.mu.Unlock()
	return job, ok
}

func (a *App) process(id string) {
	a.updateStatus(id, StatusProcessing, "")
	job, ok := a.GetJob(id)
	if !ok {
		return
	}
	ctx := context.Background()
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusProcessing, ""); err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	fileInfo, err := a.bookClient.FetchFile(ctx, job.BookID)
	if err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	tempPath, err := a.downloadFile(ctx, fileInfo.URL, fileInfo.Filename)
	if err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	defer os.Remove(tempPath)

	chunks, err := a.parseAndChunk(fileInfo.Filename, tempPath)
	if err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	if len(chunks) == 0 {
		a.failJob(id, job.BookID, fmt.Errorf("no content extracted"))
		return
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
		a.failJob(id, job.BookID, err)
		return
	}
	if err := a.indexClient.Enqueue(ctx, job.BookID); err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	a.updateStatus(id, StatusDone, "")
}

func (a *App) failJob(jobID, bookID string, err error) {
	_ = a.bookClient.UpdateStatus(context.Background(), bookID, domain.StatusFailed, err.Error())
	a.updateStatus(jobID, StatusFailed, err.Error())
}

func (a *App) updateStatus(id string, status Status, errMsg string) {
	a.mu.Lock()
	job, ok := a.jobs[id]
	if !ok {
		a.mu.Unlock()
		return
	}
	job.Status = status
	job.ErrorMessage = errMsg
	job.UpdatedAt = time.Now().UTC()
	a.jobs[id] = job
	a.mu.Unlock()
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
