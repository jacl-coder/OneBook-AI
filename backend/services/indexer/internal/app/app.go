package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"onebookai/internal/util"
	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/queue"
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
	DatabaseURL            string
	Store                  store.Store
	BookServiceURL         string
	InternalToken          string
	RedisAddr              string
	RedisPassword          string
	QueueName              string
	QueueGroup             string
	QueueConcurrency       int
	QueueMaxRetries        int
	QueueRetryDelaySeconds int
	GeminiAPIKey           string
	EmbeddingProvider      string
	EmbeddingBaseURL       string
	EmbeddingModel         string
	EmbeddingDim           int
	EmbeddingBatchSize     int
	EmbeddingConcurrency   int
}

// App processes indexing jobs.
type App struct {
	store            store.Store
	bookClient       *bookClient
	embedder         ai.Embedder
	embedDim         int
	queue            *queue.RedisJobQueue
	embedBatchSize   int
	embedConcurrency int
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
		store:            dataStore,
		bookClient:       newBookClient(cfg.BookServiceURL, cfg.InternalToken),
		embedder:         embedder,
		embedDim:         dim,
		queue:            q,
		embedBatchSize:   cfg.EmbeddingBatchSize,
		embedConcurrency: cfg.EmbeddingConcurrency,
	}
	app.startWorkers(cfg.QueueConcurrency)
	return app, nil
}

// Enqueue registers a new index job and begins processing.
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
	chunks, err := a.store.ListChunksByBook(job.BookID)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if len(chunks) == 0 {
		err := fmt.Errorf("no chunks to index")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.embedAndStore(ctx, chunks); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusReady, ""); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	return nil
}

func (a *App) startWorkers(concurrency int) {
	ctx := context.Background()
	a.queue.Start(ctx, concurrency, a.process)
}

func (a *App) embedAndStore(ctx context.Context, chunks []domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	batchSize := a.embedBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	concurrency := a.embedConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	batches := make([][]domain.Chunk, 0, (len(chunks)/batchSize)+1)
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batches = append(batches, chunks[i:end])
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)
	for _, batch := range batches {
		b := batch
		g.Go(func() error {
			return a.processBatch(gctx, b)
		})
	}
	return g.Wait()
}

func (a *App) processBatch(ctx context.Context, batch []domain.Chunk) error {
	if len(batch) == 0 {
		return nil
	}
	texts := make([]string, 0, len(batch))
	for _, chunk := range batch {
		texts = append(texts, chunk.Content)
	}
	var embeddings [][]float32
	if embedder, ok := a.embedder.(ai.BatchEmbedder); ok && len(texts) > 1 {
		out, err := embedder.EmbedTexts(ctx, texts, "RETRIEVAL_DOCUMENT")
		if err != nil {
			return err
		}
		embeddings = out
	} else {
		out := make([][]float32, 0, len(texts))
		for _, text := range texts {
			embedding, err := a.embedder.EmbedText(ctx, text, "RETRIEVAL_DOCUMENT")
			if err != nil {
				return err
			}
			out = append(out, embedding)
		}
		embeddings = out
	}
	if len(embeddings) != len(batch) {
		return fmt.Errorf("embedding count mismatch: got %d, want %d", len(embeddings), len(batch))
	}
	for i, embedding := range embeddings {
		if a.embedDim > 0 && len(embedding) != a.embedDim {
			return fmt.Errorf("embedding dimension mismatch: got %d", len(embedding))
		}
		if err := a.store.SetChunkEmbedding(batch[i].ID, embedding); err != nil {
			return err
		}
	}
	return nil
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
		return "onebook:indexer"
	}
	return name
}

func defaultQueueGroup(name string) string {
	if strings.TrimSpace(name) == "" {
		return "indexer"
	}
	return name
}
