package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

type ingestJobPayload struct {
	BookID     string `json:"bookId"`
	Generation int64  `json:"generation,omitempty"`
}

// Config holds runtime configuration.
type Config struct {
	DatabaseURL               string
	Store                     store.Store
	BookServiceURL            string
	IndexerURL                string
	InternalJWTPrivateKeyPath string
	InternalJWTKeyID          string
	RabbitMQURL               string
	QueueExchange             string
	QueueName                 string
	QueueConsumer             string
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
	q, err := queue.NewRabbitMQJobQueue(queue.RabbitMQQueueConfig{
		URL:          cfg.RabbitMQURL,
		Exchange:     defaultQueueExchange(cfg.QueueExchange),
		QueueName:    defaultQueueName(cfg.QueueName),
		ConsumerName: defaultQueueConsumer(cfg.QueueConsumer),
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
func (a *App) Enqueue(bookID string, generation int64) (Job, error) {
	if strings.TrimSpace(bookID) == "" {
		return Job{}, fmt.Errorf("bookId required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload, err := json.Marshal(ingestJobPayload{
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
	fileInfo, err := a.bookClient.FetchFile(ctx, job.BookID)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	tempPath, err := a.downloadFile(ctx, fileInfo.URL, fileInfo.Filename)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	defer os.Remove(tempPath)

	blocks, err := a.parseAndChunk(fileInfo.Filename, tempPath)
	if err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if len(blocks) == 0 {
		err := fmt.Errorf("no content extracted")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	domainChunks := a.buildRetrievalChunks(job.BookID, blocks)
	if len(domainChunks) == 0 {
		err := fmt.Errorf("no retrieval chunks generated")
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	profile := buildBookDocumentProfile(fileInfo.Filename, blocks)
	if err := a.store.UpdateBookDocumentProfile(job.BookID, profile); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.store.ReplaceChunks(job.BookID, domainChunks); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
		return err
	}
	if err := a.indexClient.Enqueue(ctx, job.BookID, generation); err != nil {
		_ = a.bookClient.UpdateStatus(ctx, job.BookID, generation, domain.StatusFailed, err.Error())
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

func generationFromPayload(payload json.RawMessage) int64 {
	if len(payload) == 0 {
		return 0
	}
	var body ingestJobPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return 0
	}
	return body.Generation
}

func defaultQueueName(name string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "onebook.ingest.jobs"
}

func defaultQueueConsumer(name string) string {
	if strings.TrimSpace(name) == "" {
		return "onebook-ingest-service"
	}
	return name
}

func defaultQueueExchange(name string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "onebook.jobs"
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
		if strings.TrimSpace(blockMeta["page"]) == "1" {
			blockMeta["is_first_page"] = "true"
		}
		if entities := extractDocumentEntities("", blockContent, strings.TrimSpace(blockMeta["page"]), strings.TrimSpace(blockMeta["source_ref"])); len(entities) > 0 {
			if raw, err := json.Marshal(entities); err == nil {
				blockMeta["entities"] = string(raw)
			}
		}
		if facts := extractDocumentFacts(blockContent, strings.TrimSpace(blockMeta["page"]), strings.TrimSpace(blockMeta["source_ref"])); len(facts) > 0 {
			if raw, err := json.Marshal(facts); err == nil {
				blockMeta["facts"] = string(raw)
				blockMeta["has_structured_facts"] = "true"
			}
		}
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

func buildBookDocumentProfile(filename string, blocks []chunkPayload) domain.BookDocumentProfile {
	firstPage := firstMeaningfulBlockText(blocks)
	overviewText := strings.TrimSpace(filename + "\n" + firstPage + "\n" + joinLeadingBlockText(blocks, 4))
	return domain.BookDocumentProfile{
		DocumentType:    inferDocumentType(filename, overviewText),
		DocumentSummary: summarizeDocument(filename, overviewText),
		FirstPageText:   limitRunes(firstPage, 2400),
		Keywords:        extractDocumentKeywords(filename, overviewText),
		Entities:        extractDocumentEntities(filename, overviewText, "1", "page:1"),
		Facts:           extractDocumentFacts(overviewText, "1", "page:1"),
	}
}

func extractDocumentEntities(filename string, text string, page string, sourceRef string) []domain.DocumentEntity {
	text = normalizeSummaryText(filename + "\n" + text)
	candidates := make([]domain.DocumentEntity, 0, 12)
	if name := firstRegexSubmatch(text, `学生\s*([\p{Han}]{2,4})`); name != "" {
		candidates = append(candidates, domain.DocumentEntity{Type: "person", Value: name, Label: "学生", Page: page})
	}
	if school := firstRegexSubmatch(text, `([\p{Han}]{2,30}(?:大学|学院|学校))\s*学校`); school != "" {
		candidates = append(candidates, domain.DocumentEntity{Type: "organization", Value: school, Label: "学校", Page: page})
	}
	if department := firstRegexSubmatch(text, `([\p{Han}A-Za-z0-9]{2,20})\s*部门`); department != "" {
		candidates = append(candidates, domain.DocumentEntity{Type: "department", Value: department, Label: "部门", Page: page})
	}
	if position := firstRegexSubmatch(text, `([\p{Han}A-Za-z0-9]{2,20})\s*岗位`); position != "" {
		candidates = append(candidates, domain.DocumentEntity{Type: "position", Value: position, Label: "岗位", Page: page})
	}
	for _, date := range allRegexSubmatches(text, `\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日`, 6) {
		candidates = append(candidates, domain.DocumentEntity{Type: "date", Value: normalizeChineseDate(date), Label: "日期", Page: page})
	}
	if idNumber := firstRegexSubmatch(text, `\d{12,18}`); idNumber != "" {
		candidates = append(candidates, domain.DocumentEntity{Type: "identity_number", Value: idNumber, Label: "编号", Page: page})
	}
	return dedupeDocumentEntities(candidates, sourceRef)
}

func extractDocumentFacts(text string, page string, sourceRef string) []domain.DocumentFact {
	text = normalizeSummaryText(text)
	facts := make([]domain.DocumentFact, 0, 12)
	if name := firstRegexSubmatch(text, `学生\s*([\p{Han}]{2,4})`); name != "" {
		facts = append(facts, domain.DocumentFact{Key: "student_name", Value: name, Label: "学生姓名", Page: page, SourceRef: sourceRef})
	}
	if gender := firstRegexSubmatch(text, `性别\s*([\p{Han}A-Za-z]{1,8})`); gender != "" {
		facts = append(facts, domain.DocumentFact{Key: "gender", Value: gender, Label: "性别", Page: page, SourceRef: sourceRef})
	}
	if school := firstRegexSubmatch(text, `([\p{Han}]{2,30}(?:大学|学院|学校))\s*学校`); school != "" {
		facts = append(facts, domain.DocumentFact{Key: "school", Value: school, Label: "学校", Page: page, SourceRef: sourceRef})
	}
	if department := firstRegexSubmatch(text, `([\p{Han}A-Za-z0-9]{2,20})\s*部门`); department != "" {
		facts = append(facts, domain.DocumentFact{Key: "department", Value: department, Label: "部门", Page: page, SourceRef: sourceRef})
	}
	if position := firstRegexSubmatch(text, `([\p{Han}A-Za-z0-9]{2,20})\s*岗位`); position != "" {
		facts = append(facts, domain.DocumentFact{Key: "position", Value: position, Label: "岗位", Page: page, SourceRef: sourceRef})
	}
	if start := firstRegexSubmatch(text, `实习时间[:：]?\s*(\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日)`); start != "" {
		facts = append(facts, domain.DocumentFact{Key: "internship_start", Value: normalizeChineseDate(start), Label: "实习开始时间", Page: page, SourceRef: sourceRef})
	}
	if end := firstRegexSubmatch(text, `至\s*(\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日)\s*截止`); end != "" {
		facts = append(facts, domain.DocumentFact{Key: "internship_end", Value: normalizeChineseDate(end), Label: "实习结束时间", Page: page, SourceRef: sourceRef})
	}
	if proofDate := lastRegexSubmatch(text, `\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日`); proofDate != "" {
		facts = append(facts, domain.DocumentFact{Key: "proof_date", Value: normalizeChineseDate(proofDate), Label: "证明日期", Page: page, SourceRef: sourceRef})
	}
	return dedupeDocumentFacts(facts)
}

func firstMeaningfulBlockText(blocks []chunkPayload) string {
	for _, block := range blocks {
		if strings.TrimSpace(block.Metadata["page"]) == "1" {
			if text := strings.TrimSpace(block.Content); text != "" {
				return text
			}
		}
	}
	for _, block := range blocks {
		if text := strings.TrimSpace(block.Content); text != "" {
			return text
		}
	}
	return ""
}

func joinLeadingBlockText(blocks []chunkPayload, limit int) string {
	if limit <= 0 {
		return ""
	}
	parts := make([]string, 0, limit)
	for _, block := range blocks {
		text := strings.TrimSpace(block.Content)
		if text == "" {
			continue
		}
		parts = append(parts, text)
		if len(parts) >= limit {
			break
		}
	}
	return strings.Join(parts, "\n")
}

func inferDocumentType(filename, text string) string {
	haystack := strings.ToLower(filename + "\n" + text)
	typeRule := []struct {
		kind  string
		terms []string
	}{
		{kind: "internship_certificate", terms: []string{"实习证明", "实习单位", "实习时间", "实习期表现"}},
		{kind: "certificate", terms: []string{"证明", "兹证明", "特此证明"}},
		{kind: "resume", terms: []string{"简历", "resume", "工作经历", "教育经历"}},
		{kind: "contract", terms: []string{"合同", "协议", "甲方", "乙方"}},
		{kind: "invoice", terms: []string{"发票", "invoice", "税号", "价税合计"}},
		{kind: "report", terms: []string{"报告", "report", "摘要", "结论"}},
		{kind: "book", terms: []string{"目录", "contents", "chapter", "preface"}},
	}
	for _, rule := range typeRule {
		score := 0
		for _, term := range rule.terms {
			if strings.Contains(haystack, strings.ToLower(term)) {
				score++
			}
		}
		if score > 0 {
			return rule.kind
		}
	}
	return "document"
}

func summarizeDocument(filename, text string) string {
	text = normalizeSummaryText(text)
	filename = strings.TrimSpace(filename)
	if text == "" {
		return strings.TrimSpace(filename)
	}
	if filename != "" {
		return limitRunes(strings.TrimSpace(filename+"： "+text), 520)
	}
	return limitRunes(text, 520)
}

func normalizeSummaryText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func extractDocumentKeywords(filename, text string) []string {
	candidates := []string{
		strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
	}
	for _, term := range []string{"实习证明", "实习", "工程研发", "兰州理工大学", "证明", "合同", "简历", "报告"} {
		if strings.Contains(text, term) || strings.Contains(filename, term) {
			candidates = append(candidates, term)
		}
	}
	out := make([]string, 0, 8)
	seen := map[string]struct{}{}
	for _, item := range candidates {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func limitRunes(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func firstRegexSubmatch(text string, pattern string) string {
	matches := allRegexSubmatches(text, pattern, 1)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func lastRegexSubmatch(text string, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	last := matches[len(matches)-1]
	if len(last) > 1 {
		return strings.TrimSpace(last[1])
	}
	return strings.TrimSpace(last[0])
}

func allRegexSubmatches(text string, pattern string, limit int) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	matches := re.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		value := ""
		if len(match) > 1 {
			value = strings.TrimSpace(match[1])
		} else if len(match) > 0 {
			value = strings.TrimSpace(match[0])
		}
		if value == "" {
			continue
		}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func normalizeChineseDate(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func dedupeDocumentEntities(items []domain.DocumentEntity, _ string) []domain.DocumentEntity {
	seen := map[string]struct{}{}
	out := make([]domain.DocumentEntity, 0, len(items))
	for _, item := range items {
		item.Type = strings.TrimSpace(item.Type)
		item.Value = strings.TrimSpace(item.Value)
		if item.Type == "" || item.Value == "" {
			continue
		}
		key := item.Type + "\x00" + item.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeDocumentFacts(items []domain.DocumentFact) []domain.DocumentFact {
	seen := map[string]struct{}{}
	out := make([]domain.DocumentFact, 0, len(items))
	for _, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		item.Value = strings.TrimSpace(item.Value)
		if item.Key == "" || item.Value == "" {
			continue
		}
		key := item.Key + "\x00" + item.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
