package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/store"
)

// Status represents the lifecycle of an index job.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Job tracks an index request.
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
	DatabaseURL    string
	Store          store.Store
	BookServiceURL string
	InternalToken  string
	GeminiAPIKey   string
	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingModel string
	EmbeddingDim   int
}

// App processes indexing jobs.
type App struct {
	mu           sync.Mutex
	jobs         map[string]Job
	store        store.Store
	bookClient   *bookClient
	embedder     ai.Embedder
	embedDim     int
}

// New constructs the indexer service with persistence.
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
	if cfg.InternalToken == "" {
		return nil, fmt.Errorf("internal token required")
	}
	if cfg.EmbeddingModel == "" {
		return nil, fmt.Errorf("embedding model required")
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.EmbeddingProvider))
	if provider == "" {
		provider = "gemini"
	}
	dim := cfg.EmbeddingDim
	var embedder ai.Embedder
	switch provider {
	case "ollama":
		if dim <= 0 {
			return nil, fmt.Errorf("embedding dim required for ollama")
		}
		ollama := ai.NewOllamaClient(cfg.EmbeddingBaseURL)
		embedder = ai.NewOllamaEmbedder(ollama, cfg.EmbeddingModel, dim)
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("gemini api key required")
		}
		gemini, err := ai.NewGeminiClient(cfg.GeminiAPIKey)
		if err != nil {
			return nil, err
		}
		embedder = ai.NewGeminiEmbedder(gemini, cfg.EmbeddingModel)
		if dim <= 0 {
			dim = 768
		}
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", provider)
	}
	return &App{
		jobs:       make(map[string]Job),
		store:      dataStore,
		bookClient: newBookClient(cfg.BookServiceURL, cfg.InternalToken),
		embedder:   embedder,
		embedDim:   dim,
	}, nil
}

// Enqueue registers a new index job and begins processing.
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
	chunks, err := a.store.ListChunksByBook(job.BookID)
	if err != nil {
		a.failJob(id, job.BookID, err)
		return
	}
	if len(chunks) == 0 {
		a.failJob(id, job.BookID, fmt.Errorf("no chunks to index"))
		return
	}
	for _, chunk := range chunks {
		embedding, err := a.embedder.EmbedText(ctx, chunk.Content, "RETRIEVAL_DOCUMENT")
		if err != nil {
			a.failJob(id, job.BookID, err)
			return
		}
		if a.embedDim > 0 && len(embedding) != a.embedDim {
			a.failJob(id, job.BookID, fmt.Errorf("embedding dimension mismatch: got %d", len(embedding)))
			return
		}
		if err := a.store.SetChunkEmbedding(chunk.ID, embedding); err != nil {
			a.failJob(id, job.BookID, err)
			return
		}
	}
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusReady, ""); err != nil {
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
