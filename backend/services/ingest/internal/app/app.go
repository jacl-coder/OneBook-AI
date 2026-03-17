package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
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
	DatabaseURL               string
	Store                     store.Store
	BookServiceURL            string
	IndexerURL                string
	InternalJWTPrivateKeyPath string
	InternalJWTKeyID          string
	KafkaBrokers              []string
	KafkaClientID             string
	KafkaTopicPrefix          string
	QueueTopic                string
	QueueGroup                string
	QueueConcurrency          int
	QueueMaxRetries           int
	QueueRetryDelaySeconds    int
	ChunkSize                 int
	ChunkOverlap              int
	LexicalChunkSize          int
	LexicalChunkOverlap       int
	SemanticChunkSize         int
	SemanticChunkOverlap      int
	OCREnabled                bool
	OCRCommand                string
	OCRDevice                 string
	OCRTimeoutSeconds         int
	OCRServiceURL             string
	PDFMinPageRunes           int
	PDFMinPageScore           float64
	PDFOCRMinScoreDelta       float64
}

// App processes ingest jobs.
type App struct {
	store                store.Store
	bookClient           *bookClient
	indexClient          *indexerClient
	queue                queue.JobQueue
	lexicalChunkSize     int
	lexicalChunkOverlap  int
	semanticChunkSize    int
	semanticChunkOverlap int
	ocrEnabled           bool
	ocrCommand           string
	ocrDevice            string
	ocrTimeout           time.Duration
	ocrServiceURL        string
	pdfMinRunes          int
	pdfMinScore          float64
	pdfScoreDiff         float64
	httpClient           *http.Client
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
	signer, err := servicetoken.NewSignerWithOptions(servicetoken.SignerOptions{
		PrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		KeyID:          cfg.InternalJWTKeyID,
		Issuer:         "ingest-service",
		TTL:            servicetoken.DefaultTokenTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("init service token signer: %w", err)
	}
	semanticChunkSize := cfg.SemanticChunkSize
	if semanticChunkSize <= 0 {
		semanticChunkSize = cfg.ChunkSize
	}
	if semanticChunkSize <= 0 {
		semanticChunkSize = 480
	}
	semanticChunkOverlap := cfg.SemanticChunkOverlap
	if semanticChunkOverlap <= 0 {
		semanticChunkOverlap = cfg.ChunkOverlap
	}
	if semanticChunkOverlap < 0 {
		semanticChunkOverlap = 0
	}
	lexicalChunkSize := cfg.LexicalChunkSize
	if lexicalChunkSize <= 0 {
		lexicalChunkSize = 160
	}
	lexicalChunkOverlap := cfg.LexicalChunkOverlap
	if lexicalChunkOverlap < 0 {
		lexicalChunkOverlap = 0
	}
	ocrCommand := strings.TrimSpace(cfg.OCRCommand)
	if ocrCommand == "" {
		ocrCommand = "paddleocr"
	}
	ocrDevice := strings.TrimSpace(cfg.OCRDevice)
	if ocrDevice == "" {
		ocrDevice = "cpu"
	}
	ocrTimeoutSeconds := cfg.OCRTimeoutSeconds
	if ocrTimeoutSeconds <= 0 {
		ocrTimeoutSeconds = 120
	}
	pdfMinRunes := cfg.PDFMinPageRunes
	if pdfMinRunes <= 0 {
		pdfMinRunes = 80
	}
	pdfMinScore := cfg.PDFMinPageScore
	if pdfMinScore <= 0 {
		pdfMinScore = 0.45
	}
	pdfScoreDiff := cfg.PDFOCRMinScoreDelta
	if pdfScoreDiff < 0 {
		pdfScoreDiff = 0
	}
	jobStore, err := queue.NewPostgresJobStore(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	q, err := queue.NewKafkaJobQueue(queue.KafkaQueueConfig{
		Brokers:      cfg.KafkaBrokers,
		ClientID:     defaultKafkaClientID(cfg.KafkaClientID, "ingest"),
		Topic:        defaultQueueTopic(cfg.QueueTopic, cfg.KafkaTopicPrefix, "ingest"),
		Group:        defaultQueueGroup(cfg.QueueGroup),
		JobType:      "ingest",
		ResourceType: "book",
		MaxRetries:   cfg.QueueMaxRetries,
		RetryDelay:   time.Duration(cfg.QueueRetryDelaySeconds) * time.Second,
		Store:        jobStore,
	})
	if err != nil {
		return nil, err
	}
	app := &App{
		store:                dataStore,
		bookClient:           newBookClient(cfg.BookServiceURL, signer),
		indexClient:          newIndexerClient(cfg.IndexerURL, signer),
		queue:                q,
		lexicalChunkSize:     lexicalChunkSize,
		lexicalChunkOverlap:  lexicalChunkOverlap,
		semanticChunkSize:    semanticChunkSize,
		semanticChunkOverlap: semanticChunkOverlap,
		ocrEnabled:           cfg.OCREnabled,
		ocrCommand:           ocrCommand,
		ocrDevice:            ocrDevice,
		ocrTimeout:           time.Duration(ocrTimeoutSeconds) * time.Second,
		ocrServiceURL:        strings.TrimSpace(cfg.OCRServiceURL),
		pdfMinRunes:          pdfMinRunes,
		pdfMinScore:          pdfMinScore,
		pdfScoreDiff:         pdfScoreDiff,
		httpClient:           &http.Client{Timeout: 60 * time.Second},
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

func (a *App) Ready(ctx context.Context) error {
	return a.queue.Ready(ctx)
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

	blocks, err := a.parseAndChunk(fileInfo.Filename, tempPath)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	if len(blocks) == 0 {
		err := fmt.Errorf("no content extracted")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
	}
	domainChunks := a.buildRetrievalChunks(job.BookID, blocks)
	if len(domainChunks) == 0 {
		err := fmt.Errorf("no retrieval chunks generated")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, domain.StatusFailed, err.Error())
		return err
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
	return defaultQueueTopic(name, "", "ingest")
}

func defaultQueueGroup(name string) string {
	if strings.TrimSpace(name) == "" {
		return "onebook-ingest-service"
	}
	return name
}

func defaultQueueTopic(topic string, prefix string, domain string) string {
	if strings.TrimSpace(topic) != "" {
		return strings.TrimSpace(topic)
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "onebook"
	}
	return prefix + "." + domain + ".jobs"
}

func defaultKafkaClientID(clientID string, service string) string {
	clientID = strings.TrimSpace(clientID)
	if clientID != "" {
		return clientID
	}
	return "onebook-" + service
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

func (a *App) buildRetrievalChunks(bookID string, blocks []chunkPayload) []domain.Chunk {
	now := time.Now().UTC()
	type tierSpec struct {
		name    string
		size    int
		overlap int
	}
	specs := []tierSpec{
		{name: "lexical", size: a.lexicalChunkSize, overlap: a.lexicalChunkOverlap},
		{name: "semantic", size: a.semanticChunkSize, overlap: a.semanticChunkOverlap},
	}
	out := make([]domain.Chunk, 0, len(blocks)*4)
	for _, block := range blocks {
		blockContent := strings.TrimSpace(block.Content)
		if blockContent == "" {
			continue
		}
		blockMeta := cloneMetadata(block.Metadata)
		chunkFamily := strings.TrimSpace(blockMeta["chunk_family"])
		if chunkFamily == "" {
			chunkFamily = sha256Hex(strings.TrimSpace(blockMeta["source_ref"]) + "\n" + blockContent)
		}
		blockMeta["chunk_family"] = chunkFamily
		for _, spec := range specs {
			parts := chunkTextByTokens(blockContent, spec.size, spec.overlap)
			if len(parts) == 0 {
				continue
			}
			for idx, part := range parts {
				meta := cloneMetadata(blockMeta)
				meta["retrieval_tier"] = spec.name
				meta["chunk_profile"] = spec.name
				meta["tier_chunk_index"] = strconv.Itoa(idx)
				meta["tier_chunk_count"] = strconv.Itoa(len(parts))
				meta["chunk"] = strconv.Itoa(idx)
				out = append(out, domain.Chunk{
					ID:        util.NewID(),
					BookID:    bookID,
					Content:   part,
					Metadata:  enrichChunkMetadata(meta, bookID, len(out), 0, part),
					CreatedAt: now,
				})
			}
		}
	}
	for idx := range out {
		out[idx].Metadata["chunk_count"] = strconv.Itoa(len(out))
	}
	return out
}

func cloneMetadata(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func enrichChunkMetadata(base map[string]string, bookID string, chunkIndex, chunkCount int, content string) map[string]string {
	out := make(map[string]string, len(base)+6)
	for k, v := range base {
		out[k] = v
	}
	out["document_id"] = strings.TrimSpace(bookID)
	out["chunk_index"] = strconv.Itoa(chunkIndex)
	out["chunk_count"] = strconv.Itoa(chunkCount)
	out["content_runes"] = strconv.Itoa(len([]rune(content)))
	out["content_sha256"] = sha256Hex(content)
	return out
}

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
