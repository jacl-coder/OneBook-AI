package eval

import "time"

// ChunkRecord is a normalized chunk used by evaluators.
type ChunkRecord struct {
	ChunkID  string
	DocID    string
	Text     string
	Metadata map[string]string
}

// QueryRecord is a normalized retrieval/e2e query.
type QueryRecord struct {
	QID            string `json:"qid"`
	Query          string `json:"query"`
	BookID         string `json:"book_id,omitempty"`
	ExpectedAnswer string `json:"expected_answer,omitempty"`
	ExpectAbstain  bool   `json:"expect_abstain,omitempty"`
}

// QRel is one relevance judgment.
type QRel struct {
	QID       string `json:"qid"`
	DocID     string `json:"doc_id"`
	Relevance int    `json:"relevance"`
}

// RunHit is one ranked retrieval result.
type RunHit struct {
	DocID string  `json:"doc_id"`
	Score float64 `json:"score,omitempty"`
}

// RunEntry stores ranked results for one query.
type RunEntry struct {
	QID     string   `json:"qid"`
	Results []RunHit `json:"results"`
}

type RetrievalStageResult struct {
	Result    EvalResult
	StageRuns map[string][]RunEntry
}

// PredictionRecord stores model output for e2e answer evaluation.
type PredictionRecord struct {
	QID       string   `json:"qid"`
	Answer    string   `json:"answer"`
	Citations []string `json:"citations,omitempty"`
	Abstained bool     `json:"abstained,omitempty"`
}

// EmbeddingRecord stores one id->vector pair.
type EmbeddingRecord struct {
	ID     string    `json:"id"`
	Vector []float32 `json:"vector"`
}

// ReportRun is persisted to run.json for reproducibility.
type ReportRun struct {
	RunID     string                 `json:"run_id"`
	Command   string                 `json:"command"`
	CreatedAt time.Time              `json:"created_at"`
	Inputs    map[string]string      `json:"inputs,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
	GitCommit string                 `json:"git_commit,omitempty"`
}

// EvalResult is returned by each evaluator.
type EvalResult struct {
	Metrics  map[string]any
	PerQuery []map[string]any
	Warnings []string
}

// EmbedderConfig configures optional online embedding.
type EmbedderConfig struct {
	Provider string
	BaseURL  string
	Model    string
	Dim      int
	Batch    int
}

// IngestionOptions configures ingestion evaluator.
type IngestionOptions struct {
	ChunksPath string
}

// ChunkingOptions configures chunking evaluator.
type ChunkingOptions struct {
	ChunksPath string
	ShortLimit int
	LongLimit  int
}

// EmbeddingOptions configures embedding evaluator.
type EmbeddingOptions struct {
	ChunksPath     string
	EmbeddingsPath string
	Online         bool
	Embedder       EmbedderConfig
}

// RetrievalOptions configures retrieval evaluator.
type RetrievalOptions struct {
	QueriesPath    string
	QrelsPath      string
	RunPath        string
	ChunksPath     string
	EmbeddingsPath string
	Online         bool
	TopK           int
	DenseTopK      int
	SparseTopK     int
	FusionTopK     int
	RerankTopN     int
	RetrievalMode  string
	Embedder       EmbedderConfig
}

// PostRetrievalOptions configures post-retrieval evaluator.
type PostRetrievalOptions struct {
	QueriesPath    string
	QrelsPath      string
	RunPath        string
	ChunksPath     string
	EmbeddingsPath string
	Online         bool
	TopK           int
	ContextBudget  int
	RetrievalMode  string
	Embedder       EmbedderConfig
}

// AnswerOptions configures answer evaluator.
type AnswerOptions struct {
	QueriesPath     string
	QrelsPath       string
	PredictionsPath string
}
