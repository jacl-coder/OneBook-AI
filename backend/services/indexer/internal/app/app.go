package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/queue"
	"onebookai/pkg/retrieval"
	"onebookai/pkg/store"

	"golang.org/x/sync/errgroup"
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

type indexJobPayload struct {
	BookID     string `json:"bookId"`
	Generation int64  `json:"generation,omitempty"`
}

// Config holds runtime configuration.
type Config struct {
	DatabaseURL               string
	Store                     store.Store
	BookServiceURL            string
	InternalJWTPrivateKeyPath string
	InternalJWTKeyID          string
	RabbitMQURL               string
	QueueExchange             string
	QueueName                 string
	QueueConsumer             string
	QueueConcurrency          int
	QueueMaxRetries           int
	QueueRetryDelaySeconds    int
	EmbeddingProvider         string
	EmbeddingBaseURL          string
	EmbeddingModel            string
	EmbeddingDim              int
	EmbeddingBatchSize        int
	EmbeddingConcurrency      int
	QdrantURL                 string
	QdrantAPIKey              string
	QdrantCollection          string
	OpenSearchURL             string
	OpenSearchIndex           string
	OpenSearchUsername        string
	OpenSearchPassword        string
}

// App processes indexing jobs.
type App struct {
	store            store.Store
	bookClient       *bookClient
	embedder         ai.Embedder
	embedDim         int
	queue            queue.JobQueue
	embedBatchSize   int
	embedConcurrency int
	search           *retrieval.Client
	lexical          *retrieval.OpenSearchClient
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
	signer, err := servicetoken.NewSignerWithOptions(servicetoken.SignerOptions{
		PrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		KeyID:          cfg.InternalJWTKeyID,
		Issuer:         "indexer-service",
		TTL:            servicetoken.DefaultTokenTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("init service token signer: %w", err)
	}
	if cfg.EmbeddingModel == "" {
		return nil, fmt.Errorf("embedding model required")
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.EmbeddingProvider))
	if provider == "" {
		provider = "ollama"
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
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", provider)
	}
	jobStore, err := queue.NewPostgresJobStore(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	q, err := queue.NewRabbitMQJobQueue(queue.RabbitMQQueueConfig{
		URL:          cfg.RabbitMQURL,
		Exchange:     defaultQueueExchange(cfg.QueueExchange),
		QueueName:    defaultQueueName(cfg.QueueName),
		ConsumerName: defaultQueueConsumer(cfg.QueueConsumer),
		JobType:      "indexer",
		ResourceType: "book",
		MaxRetries:   cfg.QueueMaxRetries,
		RetryDelay:   time.Duration(cfg.QueueRetryDelaySeconds) * time.Second,
		Store:        jobStore,
	})
	if err != nil {
		return nil, err
	}
	searchClient, err := retrieval.NewQdrantClient(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, dim)
	if err != nil {
		return nil, fmt.Errorf("init qdrant client: %w", err)
	}
	lexicalClient, err := retrieval.NewOpenSearchClient(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword)
	if err != nil {
		return nil, fmt.Errorf("init opensearch client: %w", err)
	}
	app := &App{
		store:            dataStore,
		bookClient:       newBookClient(cfg.BookServiceURL, signer),
		embedder:         embedder,
		embedDim:         dim,
		queue:            q,
		embedBatchSize:   cfg.EmbeddingBatchSize,
		embedConcurrency: cfg.EmbeddingConcurrency,
		search:           searchClient,
		lexical:          lexicalClient,
	}
	app.startWorkers(cfg.QueueConcurrency)
	return app, nil
}

// Enqueue registers a new index job and begins processing.
func (a *App) Enqueue(bookID string, generation int64) (Job, error) {
	if strings.TrimSpace(bookID) == "" {
		return Job{}, fmt.Errorf("bookId required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload, err := json.Marshal(indexJobPayload{
		BookID:     strings.TrimSpace(bookID),
		Generation: generation,
	})
	if err != nil {
		return Job{}, err
	}
	status, err := a.queue.EnqueueWithPayload(ctx, bookID, payload)
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

func (a *App) Ready(ctx context.Context) error {
	return a.queue.Ready(ctx)
}

func (a *App) process(ctx context.Context, job queue.JobStatus) error {
	if ctx == nil {
		ctx = context.Background()
	}
	generation := generationFromPayload(job.Payload)
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusProcessing, ""); err != nil {
		if errors.Is(err, ErrStaleBookGeneration) {
			return nil
		}
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	chunks, err := a.store.ListChunksByBook(job.BookID)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if len(chunks) == 0 {
		err := fmt.Errorf("no chunks to index")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	semanticChunks, lexicalChunks := splitChunksByTier(chunks)
	if len(semanticChunks) == 0 {
		err := fmt.Errorf("no semantic chunks to index")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.search.EnsureCollection(ctx); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.lexical.EnsureIndex(ctx); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.search.DeleteByBook(ctx, job.BookID); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.lexical.DeleteByBook(ctx, job.BookID); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.embedAndStore(ctx, semanticChunks); err != nil {
		_ = a.store.UpdateChunkIndexStatus(chunkIDs(semanticChunks), domain.ChunkIndexBackendQdrant, domain.ChunkIndexSyncStatusFailed, cfgEmbeddingModel(a.embedder), a.embedDim, err.Error())
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.store.UpdateChunkIndexStatus(chunkIDs(semanticChunks), domain.ChunkIndexBackendQdrant, domain.ChunkIndexSyncStatusSynced, cfgEmbeddingModel(a.embedder), a.embedDim, ""); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.indexLexical(ctx, lexicalChunks); err != nil {
		_ = a.store.UpdateChunkIndexStatus(chunkIDs(lexicalChunks), domain.ChunkIndexBackendOpenSearch, domain.ChunkIndexSyncStatusFailed, cfgEmbeddingModel(a.embedder), a.embedDim, err.Error())
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.store.UpdateChunkIndexStatus(chunkIDs(lexicalChunks), domain.ChunkIndexBackendOpenSearch, domain.ChunkIndexSyncStatusSynced, cfgEmbeddingModel(a.embedder), a.embedDim, ""); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusReady, ""); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	return nil
}

func splitChunksByTier(chunks []domain.Chunk) ([]domain.Chunk, []domain.Chunk) {
	semantic := make([]domain.Chunk, 0, len(chunks))
	lexical := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		switch strings.TrimSpace(chunk.Metadata["retrieval_tier"]) {
		case "lexical":
			lexical = append(lexical, chunk)
		default:
			semantic = append(semantic, chunk)
		}
	}
	return semantic, lexical
}

func generationFromPayload(payload json.RawMessage) int64 {
	if len(payload) == 0 {
		return 0
	}
	var body indexJobPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return 0
	}
	return body.Generation
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
	points := make([]retrieval.UpsertPoint, 0, len(batch))
	for i, embedding := range embeddings {
		if a.embedDim > 0 && len(embedding) != a.embedDim {
			return fmt.Errorf("embedding dimension mismatch: got %d", len(embedding))
		}
		language := strings.TrimSpace(batch[i].Metadata["language"])
		if language == "" {
			language = retrieval.DetectLanguage(batch[i].Content)
		}
		points = append(points, retrieval.UpsertPoint{
			ID:     batch[i].ID,
			Dense:  embedding,
			Sparse: retrieval.BuildSparseVector(batch[i].Content, language),
			Payload: map[string]any{
				"chunk_id":      batch[i].ID,
				"book_id":       batch[i].BookID,
				"chunk_family":  strings.TrimSpace(batch[i].Metadata["chunk_family"]),
				"section_id":    strings.TrimSpace(batch[i].Metadata["section_id"]),
				"block_type":    firstNonEmpty(batch[i].Metadata["block_type"], batch[i].Metadata["source_type"]),
				"language":      language,
				"is_first_page": strings.TrimSpace(batch[i].Metadata["is_first_page"]),
				"entities":      strings.TrimSpace(batch[i].Metadata["entities"]),
				"facts":         strings.TrimSpace(batch[i].Metadata["facts"]),
			},
		})
	}
	return a.search.UpsertPoints(ctx, points)
}

func (a *App) indexLexical(ctx context.Context, chunks []domain.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	docs := make([]retrieval.LexicalDocument, 0, len(chunks))
	for _, chunk := range chunks {
		language := strings.TrimSpace(chunk.Metadata["language"])
		if language == "" {
			language = retrieval.DetectLanguage(chunk.Content)
		}
		payload := map[string]any{
			"chunk_id":      chunk.ID,
			"book_id":       chunk.BookID,
			"chunk_family":  strings.TrimSpace(chunk.Metadata["chunk_family"]),
			"section_id":    strings.TrimSpace(chunk.Metadata["section_id"]),
			"title":         strings.TrimSpace(chunk.Metadata["title"]),
			"section_title": firstNonEmpty(chunk.Metadata["section_title"], chunk.Metadata["section"], chunk.Metadata["section_path"]),
			"keywords":      strings.TrimSpace(chunk.Metadata["keywords"]),
			"tags":          strings.TrimSpace(chunk.Metadata["tags"]),
			"block_type":    firstNonEmpty(chunk.Metadata["block_type"], chunk.Metadata["source_type"]),
			"language":      language,
			"is_first_page": strings.TrimSpace(chunk.Metadata["is_first_page"]),
			"entities":      strings.TrimSpace(chunk.Metadata["entities"]),
			"facts":         strings.TrimSpace(chunk.Metadata["facts"]),
		}
		docs = append(docs, retrieval.LexicalDocument{
			ID:      chunk.ID,
			Content: chunk.Content,
			Terms:   strings.Join(retrieval.Tokenize(chunk.Content, language), " "),
			Payload: payload,
		})
	}
	return a.lexical.IndexDocuments(ctx, docs)
}

func chunkIDs(chunks []domain.Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.ID) == "" {
			continue
		}
		out = append(out, chunk.ID)
	}
	return out
}

func cfgEmbeddingModel(embedder ai.Embedder) string {
	type modelNamer interface {
		ModelName() string
	}
	if named, ok := embedder.(modelNamer); ok {
		return strings.TrimSpace(named.ModelName())
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "onebook.indexer.jobs"
}

func defaultQueueConsumer(name string) string {
	if strings.TrimSpace(name) == "" {
		return "onebook-indexer-service"
	}
	return name
}

func defaultQueueExchange(name string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "onebook.jobs"
}
