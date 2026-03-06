package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"onebookai/internal/eval"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/store"
)

type evalCenter struct {
	store        store.Store
	baseDir      string
	pollInterval time.Duration
}

type EvalUploadedFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

type EvalDatasetCreateInput struct {
	Name        string
	SourceType  domain.EvalDatasetSourceType
	BookID      string
	Version     int
	Description string
	Files       map[string]EvalUploadedFile
}

type EvalDatasetUpdateInput struct {
	Name        *string
	Description *string
	Status      *domain.EvalDatasetStatus
}

type EvalRunCreateInput struct {
	DatasetID      string
	Mode           domain.EvalRunMode
	RetrievalMode  domain.EvalRetrievalMode
	GateMode       string
	Params         map[string]any
	IdempotencyKey string
}

func newEvalCenter(dataStore store.Store, baseDir string, pollInterval time.Duration) (*evalCenter, error) {
	if dataStore == nil {
		return nil, fmt.Errorf("store required")
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = filepath.Join("data", "eval-center")
	}
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create eval storage dir: %w", err)
	}
	return &evalCenter{store: dataStore, baseDir: baseDir, pollInterval: pollInterval}, nil
}

func (a *App) StartEvalWorker(ctx context.Context) {
	if a.evals == nil {
		return
	}
	go a.evals.runQueue(ctx)
}

func (c *evalCenter) runQueue(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processNext(ctx)
		}
	}
}

func (c *evalCenter) processNext(ctx context.Context) {
	runs, _, err := c.store.ListEvalRuns(store.EvalRunListOptions{
		Status:   string(domain.EvalRunStatusQueued),
		Page:     1,
		PageSize: 1,
	})
	if err != nil || len(runs) == 0 {
		return
	}
	_ = c.executeRun(ctx, runs[0])
}

func (c *evalCenter) executeRun(ctx context.Context, run domain.EvalRun) error {
	if active, ok, err := c.store.GetActiveEvalRunByFingerprint(run.Fingerprint); err == nil && ok && active.ID != run.ID && active.Status == domain.EvalRunStatusRunning {
		now := time.Now().UTC()
		run.Status = domain.EvalRunStatusCanceled
		run.Progress = 100
		run.ErrorMessage = "duplicate fingerprint suppressed"
		run.FinishedAt = &now
		run.UpdatedAt = now
		return c.store.SaveEvalRun(run)
	}
	now := time.Now().UTC()
	run.Status = domain.EvalRunStatusRunning
	run.StartedAt = &now
	run.Progress = 10
	run.UpdatedAt = now
	if err := c.store.SaveEvalRun(run); err != nil {
		return err
	}

	dataset, ok, err := c.store.GetEvalDataset(run.DatasetID)
	if err != nil {
		return c.failRun(run, fmt.Errorf("load dataset: %w", err))
	}
	if !ok {
		return c.failRun(run, fmt.Errorf("dataset %s not found", run.DatasetID))
	}

	bundle, err := eval.ExecuteCenterRun(c.buildRunOptions(run, dataset))
	if err != nil {
		return c.failRun(run, err)
	}

	current, ok, err := c.store.GetEvalRun(run.ID)
	if err != nil {
		return err
	}
	if ok && current.Status == domain.EvalRunStatusCanceled {
		current.Progress = 100
		finished := time.Now().UTC()
		current.FinishedAt = &finished
		current.UpdatedAt = finished
		return c.store.SaveEvalRun(current)
	}

	finished := time.Now().UTC()
	run.Status = domain.EvalRunStatusSucceeded
	run.Progress = 100
	run.FinishedAt = &finished
	run.UpdatedAt = finished
	run.ErrorMessage = ""
	run.SummaryMetrics = bundle.Metrics
	run.Warnings = bundle.Warnings
	run.GateStatus = mapGateStatus(bundle.Gate)
	run.StageSummaries = make([]domain.EvalRunStageSummary, 0, len(bundle.StageSummaries))
	for _, summary := range bundle.StageSummaries {
		run.StageSummaries = append(run.StageSummaries, domain.EvalRunStageSummary{
			Stage:   summary.Stage,
			Metrics: summary.Metrics,
		})
	}
	run.Artifacts = make([]domain.EvalRunArtifact, 0, len(bundle.Artifacts))
	for _, artifact := range bundle.Artifacts {
		relativePath, relErr := filepath.Rel(c.baseDir, artifact.Path)
		if relErr != nil {
			relativePath = artifact.Path
		}
		run.Artifacts = append(run.Artifacts, domain.EvalRunArtifact{
			Name:        artifact.Name,
			Path:        filepath.ToSlash(relativePath),
			ContentType: artifact.ContentType,
			SizeBytes:   artifact.SizeBytes,
			CreatedAt:   finished,
		})
	}
	return c.store.SaveEvalRun(run)
}

func (c *evalCenter) failRun(run domain.EvalRun, err error) error {
	finished := time.Now().UTC()
	run.Status = domain.EvalRunStatusFailed
	run.Progress = 100
	run.ErrorMessage = strings.TrimSpace(err.Error())
	run.FinishedAt = &finished
	run.UpdatedAt = finished
	if run.GateStatus == "" {
		run.GateStatus = domain.EvalGateStatusFailed
	}
	return c.store.SaveEvalRun(run)
}

func mapGateStatus(summary eval.GateSummary) domain.EvalGateStatus {
	if !summary.Passed && strings.EqualFold(summary.Mode, "strict") {
		return domain.EvalGateStatusFailed
	}
	if len(summary.Warnings) > 0 {
		return domain.EvalGateStatusWarn
	}
	return domain.EvalGateStatusPassed
}

func (c *evalCenter) buildRunOptions(run domain.EvalRun, dataset domain.EvalDataset) eval.CenterRunOptions {
	params := run.Params
	outDir := c.absPath(filepath.ToSlash(filepath.Join("runs", run.ID)))
	chunksPath := c.datasetFilePath(dataset, "chunks")
	queriesPath := c.datasetFilePath(dataset, "queries")
	qrelsPath := c.datasetFilePath(dataset, "qrels")
	predictionsPath := c.datasetFilePath(dataset, "predictions")
	topK := intFromParams(params, "topK", 20)
	contextBudget := intFromParams(params, "contextBudget", 4000)
	command := string(run.Mode)
	if command == "" {
		command = "all"
	}
	return eval.CenterRunOptions{
		RunID:    run.ID,
		Command:  command,
		OutDir:   outDir,
		GateMode: run.GateMode,
		Ingestion: eval.IngestionOptions{
			ChunksPath: chunksPath,
		},
		Chunking: eval.ChunkingOptions{
			ChunksPath: chunksPath,
			ShortLimit: intFromParams(params, "shortLimit", 80),
			LongLimit:  intFromParams(params, "longLimit", 1200),
		},
		Embedding: eval.EmbeddingOptions{
			ChunksPath:     chunksPath,
			EmbeddingsPath: c.datasetFilePath(dataset, "embeddings"),
			Online:         boolFromParams(params, "online", true),
			Embedder:       embedderConfigFromParams(params),
		},
		Retrieval: eval.RetrievalOptions{
			QueriesPath:    queriesPath,
			QrelsPath:      qrelsPath,
			RunPath:        c.datasetFilePath(dataset, "run"),
			ChunksPath:     chunksPath,
			EmbeddingsPath: c.datasetFilePath(dataset, "embeddings"),
			Online:         boolFromParams(params, "online", true),
			TopK:           topK,
			DenseTopK:      intFromParams(params, "denseTopK", topK),
			SparseTopK:     intFromParams(params, "sparseTopK", topK),
			FusionTopK:     intFromParams(params, "fusionTopK", topK),
			RerankTopN:     intFromParams(params, "rerankTopN", 10),
			RetrievalMode:  string(run.RetrievalMode),
			Embedder:       embedderConfigFromParams(params),
		},
		PostRetr: eval.PostRetrievalOptions{
			QueriesPath:    queriesPath,
			QrelsPath:      qrelsPath,
			RunPath:        c.datasetFilePath(dataset, "run"),
			ChunksPath:     chunksPath,
			EmbeddingsPath: c.datasetFilePath(dataset, "embeddings"),
			Online:         boolFromParams(params, "online", true),
			TopK:           topK,
			ContextBudget:  contextBudget,
			RetrievalMode:  string(run.RetrievalMode),
			Embedder:       embedderConfigFromParams(params),
		},
		Answer: eval.AnswerOptions{
			QueriesPath:     queriesPath,
			QrelsPath:       qrelsPath,
			PredictionsPath: predictionsPath,
		},
		WriteOnline: true,
	}
}

func embedderConfigFromParams(params map[string]any) eval.EmbedderConfig {
	return eval.EmbedderConfig{
		Provider: stringFromParams(params, "embeddingProvider", "ollama"),
		BaseURL:  stringFromParams(params, "embeddingBaseURL", os.Getenv("OLLAMA_HOST")),
		Model:    stringFromParams(params, "embeddingModel", os.Getenv("OLLAMA_EMBEDDING_MODEL")),
		Dim:      intFromParams(params, "embeddingDim", 3072),
		Batch:    intFromParams(params, "embeddingBatch", 16),
	}
}

func (a *App) AdminGetEvalOverview(windowStart time.Time) (domain.AdminEvalOverview, error) {
	return a.store.GetAdminEvalOverview(windowStart)
}

func (a *App) AdminListEvalDatasets(opts store.EvalDatasetListOptions) ([]domain.EvalDataset, int, error) {
	return a.store.ListEvalDatasets(opts)
}

func (a *App) AdminGetEvalDataset(id string) (domain.EvalDataset, error) {
	item, ok, err := a.store.GetEvalDataset(id)
	if err != nil {
		return domain.EvalDataset{}, err
	}
	if !ok {
		return domain.EvalDataset{}, fmt.Errorf("dataset not found")
	}
	return item, nil
}

func (a *App) AdminCreateEvalDataset(actor domain.User, input EvalDatasetCreateInput) (domain.EvalDataset, error) {
	if a.evals == nil {
		return domain.EvalDataset{}, fmt.Errorf("eval center disabled")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.EvalDataset{}, fmt.Errorf("name is required")
	}
	sourceType := input.SourceType
	if sourceType != domain.EvalDatasetSourceUpload && sourceType != domain.EvalDatasetSourceBook {
		return domain.EvalDataset{}, fmt.Errorf("invalid source type")
	}
	version := input.Version
	if version <= 0 {
		version = 1
	}
	dataset := domain.EvalDataset{
		ID:          util.NewID(),
		Name:        name,
		SourceType:  sourceType,
		BookID:      strings.TrimSpace(input.BookID),
		Version:     version,
		Status:      domain.EvalDatasetStatusActive,
		Description: strings.TrimSpace(input.Description),
		Files:       map[string]string{},
		CreatedBy:   actor.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	dir := a.evals.datasetDir(dataset.ID, dataset.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return domain.EvalDataset{}, err
	}
	for key, file := range input.Files {
		if len(file.Data) == 0 {
			continue
		}
		filename := datasetFilenameForKey(key)
		if filename == "" {
			continue
		}
		relativePath := filepath.ToSlash(filepath.Join("datasets", dataset.ID, fmt.Sprintf("v%d", dataset.Version), filename))
		if err := os.WriteFile(a.evals.absPath(relativePath), file.Data, 0o644); err != nil {
			return domain.EvalDataset{}, err
		}
		dataset.Files[key] = relativePath
	}
	if sourceType == domain.EvalDatasetSourceBook {
		if dataset.BookID == "" {
			return domain.EvalDataset{}, fmt.Errorf("bookId is required for book datasets")
		}
		if _, ok, err := a.store.GetBook(dataset.BookID); err != nil {
			return domain.EvalDataset{}, err
		} else if !ok {
			return domain.EvalDataset{}, fmt.Errorf("book not found")
		}
		if _, exists := dataset.Files["chunks"]; !exists {
			if err := a.evals.exportBookChunks(dataset); err != nil {
				return domain.EvalDataset{}, err
			}
			dataset.Files["chunks"] = filepath.ToSlash(filepath.Join("datasets", dataset.ID, fmt.Sprintf("v%d", dataset.Version), "chunks.jsonl"))
		}
	}
	if sourceType == domain.EvalDatasetSourceUpload && dataset.Files["chunks"] == "" {
		return domain.EvalDataset{}, fmt.Errorf("chunks file is required for upload datasets")
	}
	if dataset.Files["queries"] == "" || dataset.Files["qrels"] == "" {
		return domain.EvalDataset{}, fmt.Errorf("queries and qrels files are required")
	}
	if err := a.store.SaveEvalDataset(dataset); err != nil {
		return domain.EvalDataset{}, err
	}
	return dataset, nil
}

func (c *evalCenter) exportBookChunks(dataset domain.EvalDataset) error {
	chunks, err := c.store.ListChunksByBook(dataset.BookID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return fmt.Errorf("book chunks not found")
	}
	target := c.absPath(filepath.ToSlash(filepath.Join("datasets", dataset.ID, fmt.Sprintf("v%d", dataset.Version), "chunks.jsonl")))
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	for _, chunk := range chunks {
		row := map[string]any{
			"chunk_id": chunk.ID,
			"doc_id":   chunk.BookID,
			"text":     chunk.Content,
			"metadata": chunk.Metadata,
		}
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) AdminUpdateEvalDataset(id string, input EvalDatasetUpdateInput) (domain.EvalDataset, error) {
	item, ok, err := a.store.GetEvalDataset(id)
	if err != nil {
		return domain.EvalDataset{}, err
	}
	if !ok {
		return domain.EvalDataset{}, fmt.Errorf("dataset not found")
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return domain.EvalDataset{}, fmt.Errorf("name cannot be empty")
		}
		item.Name = name
	}
	if input.Description != nil {
		item.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		item.Status = *input.Status
	}
	item.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveEvalDataset(item); err != nil {
		return domain.EvalDataset{}, err
	}
	return item, nil
}

func (a *App) AdminDeleteEvalDataset(id string) error {
	if a.evals == nil {
		return fmt.Errorf("eval center disabled")
	}
	count, err := a.store.CountEvalRunsByDataset(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return a.store.ArchiveEvalDataset(id)
	}
	item, ok, err := a.store.GetEvalDataset(id)
	if err != nil {
		return err
	}
	if ok {
		_ = os.RemoveAll(a.evals.datasetDir(item.ID, item.Version))
	}
	return a.store.DeleteEvalDataset(id)
}

func (a *App) AdminListEvalRuns(opts store.EvalRunListOptions) ([]domain.EvalRun, int, error) {
	return a.store.ListEvalRuns(opts)
}

func (a *App) AdminGetEvalRun(id string) (domain.EvalRun, error) {
	item, ok, err := a.store.GetEvalRun(id)
	if err != nil {
		return domain.EvalRun{}, err
	}
	if !ok {
		return domain.EvalRun{}, fmt.Errorf("run not found")
	}
	return item, nil
}

func (a *App) AdminCreateEvalRun(actor domain.User, input EvalRunCreateInput) (domain.EvalRun, bool, error) {
	if a.evals == nil {
		return domain.EvalRun{}, false, fmt.Errorf("eval center disabled")
	}
	if _, ok, err := a.store.GetEvalDataset(strings.TrimSpace(input.DatasetID)); err != nil {
		return domain.EvalRun{}, false, err
	} else if !ok {
		return domain.EvalRun{}, false, fmt.Errorf("dataset not found")
	}
	mode := input.Mode
	if mode == "" {
		mode = domain.EvalRunModeAll
	}
	retrievalMode := input.RetrievalMode
	if retrievalMode == "" {
		retrievalMode = domain.EvalRetrievalModeHybrid
	}
	gateMode := strings.TrimSpace(input.GateMode)
	if gateMode == "" {
		gateMode = "warn"
	}
	fingerprint := buildEvalRunFingerprint(strings.TrimSpace(input.DatasetID), mode, retrievalMode, gateMode, input.Params)
	record, replayRun, replayed, err := a.beginEvalIdempotency(actor.ID, strings.TrimSpace(input.IdempotencyKey), fingerprint, input)
	if err != nil {
		return domain.EvalRun{}, false, err
	}
	if replayed {
		return replayRun, true, nil
	}
	if existing, ok, err := a.store.GetActiveEvalRunByFingerprint(fingerprint); err != nil {
		return domain.EvalRun{}, false, err
	} else if ok {
		_ = a.completeEvalIdempotency(record, existing.ID, http.StatusOK)
		return existing, true, nil
	}
	run := domain.EvalRun{
		ID:            util.NewID(),
		DatasetID:     strings.TrimSpace(input.DatasetID),
		Fingerprint:   fingerprint,
		Status:        domain.EvalRunStatusQueued,
		Mode:          mode,
		RetrievalMode: retrievalMode,
		Params:        input.Params,
		GateMode:      gateMode,
		GateStatus:    domain.EvalGateStatusWarn,
		CreatedBy:     actor.ID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := a.store.SaveEvalRun(run); err != nil {
		_ = a.markEvalIdempotencyFailed(record, http.StatusInternalServerError)
		return domain.EvalRun{}, false, err
	}
	_ = a.completeEvalIdempotency(record, run.ID, http.StatusCreated)
	return run, false, nil
}

func (a *App) AdminCancelEvalRun(id string) (domain.EvalRun, error) {
	run, ok, err := a.store.GetEvalRun(id)
	if err != nil {
		return domain.EvalRun{}, err
	}
	if !ok {
		return domain.EvalRun{}, fmt.Errorf("run not found")
	}
	if run.Status == domain.EvalRunStatusSucceeded || run.Status == domain.EvalRunStatusFailed {
		return run, nil
	}
	run.Status = domain.EvalRunStatusCanceled
	run.Progress = 100
	now := time.Now().UTC()
	run.FinishedAt = &now
	run.UpdatedAt = now
	if err := a.store.SaveEvalRun(run); err != nil {
		return domain.EvalRun{}, err
	}
	return run, nil
}

func (a *App) AdminGetEvalArtifactPath(runID, name string) (string, domain.EvalRunArtifact, error) {
	run, ok, err := a.store.GetEvalRun(runID)
	if err != nil {
		return "", domain.EvalRunArtifact{}, err
	}
	if !ok {
		return "", domain.EvalRunArtifact{}, fmt.Errorf("run not found")
	}
	for _, artifact := range run.Artifacts {
		if artifact.Name == name {
			return a.evals.absPath(artifact.Path), artifact, nil
		}
	}
	return "", domain.EvalRunArtifact{}, fmt.Errorf("artifact not found")
}

func (a *App) AdminGetEvalPerQuery(runID string) ([]map[string]any, error) {
	path, _, err := a.AdminGetEvalArtifactPath(runID, "per_query.jsonl")
	if err != nil {
		return nil, err
	}
	return eval.ParsePerQueryFile(path)
}

func (c *evalCenter) absPath(relative string) string {
	relative = filepath.Clean(strings.TrimSpace(relative))
	return filepath.Join(c.baseDir, relative)
}

func (c *evalCenter) datasetDir(datasetID string, version int) string {
	return c.absPath(filepath.ToSlash(filepath.Join("datasets", datasetID, fmt.Sprintf("v%d", version))))
}

func (c *evalCenter) datasetFilePath(dataset domain.EvalDataset, key string) string {
	if dataset.Files == nil {
		return ""
	}
	path := strings.TrimSpace(dataset.Files[key])
	if path == "" {
		return ""
	}
	return c.absPath(path)
}

func datasetFilenameForKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "chunks":
		return "chunks.jsonl"
	case "queries":
		return "queries.jsonl"
	case "qrels":
		return "qrels.tsv"
	case "predictions":
		return "predictions.jsonl"
	case "metadata":
		return "metadata.json"
	case "embeddings":
		return "embeddings.jsonl"
	case "run":
		return "run.jsonl"
	default:
		return ""
	}
}

func intFromParams(params map[string]any, key string, fallback int) int {
	if len(params) == 0 {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case int:
		if typed > 0 {
			return typed
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func stringFromParams(params map[string]any, key, fallback string) string {
	if len(params) == 0 {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", value))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func boolFromParams(params map[string]any, key string, fallback bool) bool {
	if len(params) == 0 {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return fallback
}
