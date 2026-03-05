package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"onebookai/internal/eval"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "rag_eval error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printRootUsage()
		return nil
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "ingest":
		return runIngest(rest)
	case "chunking":
		return runChunking(rest)
	case "embedding":
		return runEmbedding(rest)
	case "retrieval":
		return runRetrieval(rest)
	case "post-retrieval":
		return runPostRetrieval(rest)
	case "answer":
		return runAnswer(rest)
	case "all":
		return runAll(rest)
	case "-h", "--help", "help":
		printRootUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func printRootUsage() {
	fmt.Println("rag_eval usage:")
	fmt.Println("  rag_eval ingest --chunks <path> --out-dir <path>")
	fmt.Println("  rag_eval chunking --chunks <path> --out-dir <path>")
	fmt.Println("  rag_eval embedding --chunks <path> [--embeddings <path>] [--online] --out-dir <path>")
	fmt.Println("  rag_eval retrieval --queries <path> --qrels <path> [--run <path>] [--online] --out-dir <path>")
	fmt.Println("  rag_eval post-retrieval --queries <path> --qrels <path> [--run <path>] --out-dir <path>")
	fmt.Println("  rag_eval answer --queries <path> --qrels <path> --predictions <path> --out-dir <path>")
	fmt.Println("  rag_eval all --chunks <path> --queries <path> --qrels <path> --predictions <path> --out-dir <path>")
}

type commonFlags struct {
	runID    string
	outDir   string
	gateMode string
}

type embedFlags struct {
	provider string
	baseURL  string
	model    string
	dim      int
	batch    int
}

func bindCommon(fs *flag.FlagSet) *commonFlags {
	c := &commonFlags{}
	fs.StringVar(&c.runID, "run-id", "", "run identifier (default UTC timestamp)")
	fs.StringVar(&c.outDir, "out-dir", "", "output directory")
	fs.StringVar(&c.gateMode, "gate-mode", "warn", "gate mode: off|warn|strict")
	return c
}

func bindEmbed(fs *flag.FlagSet) *embedFlags {
	e := &embedFlags{}
	fs.StringVar(&e.provider, "embedding-provider", "ollama", "embedding provider")
	fs.StringVar(&e.baseURL, "embedding-base-url", os.Getenv("OLLAMA_HOST"), "embedding provider base URL")
	fs.StringVar(&e.model, "embedding-model", os.Getenv("OLLAMA_EMBEDDING_MODEL"), "embedding model")
	fs.IntVar(&e.dim, "embedding-dim", intEnv("ONEBOOK_EMBEDDING_DIM", 3072), "embedding dimension")
	fs.IntVar(&e.batch, "embedding-batch", 16, "batch size hint")
	return e
}

func toEmbedCfg(e *embedFlags) eval.EmbedderConfig {
	return eval.EmbedderConfig{Provider: e.provider, BaseURL: e.baseURL, Model: e.model, Dim: e.dim, Batch: e.batch}
}

func normalizeOutDir(command string, c *commonFlags) (string, string) {
	runID := strings.TrimSpace(c.runID)
	if runID == "" {
		runID = time.Now().UTC().Format("20060102T150405Z")
	}
	outDir := strings.TrimSpace(c.outDir)
	if outDir == "" {
		outDir = filepath.Join(".cache", "rag-eval", command, runID)
	}
	return outDir, runID
}

func buildRun(command, runID string, inputs map[string]string, params map[string]any) eval.ReportRun {
	return eval.ReportRun{
		RunID:     runID,
		Command:   command,
		CreatedAt: time.Now().UTC(),
		Inputs:    inputs,
		Params:    params,
		GitCommit: gitCommitSafe(),
	}
}

func runIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	c := bindCommon(fs)
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("ingest", c)
	res, err := eval.EvaluateIngestion(eval.IngestionOptions{ChunksPath: *chunks})
	if err != nil {
		return err
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("ingest", runID, map[string]string{"chunks": *chunks}, nil)
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runChunking(args []string) error {
	fs := flag.NewFlagSet("chunking", flag.ContinueOnError)
	c := bindCommon(fs)
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	shortLimit := fs.Int("short-limit", 80, "short threshold")
	longLimit := fs.Int("long-limit", 1200, "long threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("chunking", c)
	res, err := eval.EvaluateChunking(eval.ChunkingOptions{ChunksPath: *chunks, ShortLimit: *shortLimit, LongLimit: *longLimit})
	if err != nil {
		return err
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("chunking", runID, map[string]string{"chunks": *chunks}, map[string]any{"short_limit": *shortLimit, "long_limit": *longLimit})
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runEmbedding(args []string) error {
	fs := flag.NewFlagSet("embedding", flag.ContinueOnError)
	c := bindCommon(fs)
	e := bindEmbed(fs)
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	embeddings := fs.String("embeddings", "", "path to embeddings.jsonl")
	online := fs.Bool("online", false, "compute embeddings online")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("embedding", c)
	res, generated, err := eval.EvaluateEmbedding(eval.EmbeddingOptions{ChunksPath: *chunks, EmbeddingsPath: *embeddings, Online: *online, Embedder: toEmbedCfg(e)})
	if err != nil {
		return err
	}
	if *online && len(generated) > 0 {
		generatedPath := filepath.Join(outDir, "generated_embeddings.jsonl")
		if err := eval.WriteEmbeddingsJSONL(generatedPath, generated); err == nil {
			res.Metrics["generated_embeddings_path"] = generatedPath
		}
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("embedding", runID, map[string]string{"chunks": *chunks, "embeddings": *embeddings}, map[string]any{"online": *online, "embedder": toEmbedCfg(e)})
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runRetrieval(args []string) error {
	fs := flag.NewFlagSet("retrieval", flag.ContinueOnError)
	c := bindCommon(fs)
	e := bindEmbed(fs)
	queries := fs.String("queries", "", "path to queries.jsonl")
	qrels := fs.String("qrels", "", "path to qrels.tsv/jsonl")
	runPath := fs.String("run", "", "path to run.jsonl")
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	embeddings := fs.String("embeddings", "", "path to embeddings.jsonl")
	online := fs.Bool("online", false, "build run online")
	topK := fs.Int("top-k", 20, "top-k")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("retrieval", c)
	res, runs, err := eval.EvaluateRetrieval(eval.RetrievalOptions{
		QueriesPath:    *queries,
		QrelsPath:      *qrels,
		RunPath:        *runPath,
		ChunksPath:     *chunks,
		EmbeddingsPath: *embeddings,
		Online:         *online,
		TopK:           *topK,
		Embedder:       toEmbedCfg(e),
	})
	if err != nil {
		return err
	}
	if *online && len(runs) > 0 {
		generatedPath := filepath.Join(outDir, "generated_run.jsonl")
		if err := eval.WriteRunJSONL(generatedPath, runs); err == nil {
			res.Metrics["generated_run_path"] = generatedPath
		}
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("retrieval", runID, map[string]string{"queries": *queries, "qrels": *qrels, "run": *runPath, "chunks": *chunks, "embeddings": *embeddings}, map[string]any{"online": *online, "top_k": *topK, "embedder": toEmbedCfg(e)})
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runPostRetrieval(args []string) error {
	fs := flag.NewFlagSet("post-retrieval", flag.ContinueOnError)
	c := bindCommon(fs)
	e := bindEmbed(fs)
	queries := fs.String("queries", "", "path to queries.jsonl")
	qrels := fs.String("qrels", "", "path to qrels.tsv/jsonl")
	runPath := fs.String("run", "", "path to run.jsonl")
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	embeddings := fs.String("embeddings", "", "path to embeddings.jsonl")
	online := fs.Bool("online", false, "build run online when run is absent")
	topK := fs.Int("top-k", 20, "top-k")
	contextBudget := fs.Int("context-budget", 4000, "context budget")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("post-retrieval", c)
	res, runs, err := eval.EvaluatePostRetrieval(eval.PostRetrievalOptions{
		QueriesPath:    *queries,
		QrelsPath:      *qrels,
		RunPath:        *runPath,
		ChunksPath:     *chunks,
		EmbeddingsPath: *embeddings,
		Online:         *online,
		TopK:           *topK,
		ContextBudget:  *contextBudget,
		Embedder:       toEmbedCfg(e),
	})
	if err != nil {
		return err
	}
	if *online && *runPath == "" && len(runs) > 0 {
		generatedPath := filepath.Join(outDir, "generated_run.jsonl")
		if err := eval.WriteRunJSONL(generatedPath, runs); err == nil {
			res.Metrics["generated_run_path"] = generatedPath
		}
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("post-retrieval", runID, map[string]string{"queries": *queries, "qrels": *qrels, "run": *runPath, "chunks": *chunks, "embeddings": *embeddings}, map[string]any{"online": *online, "top_k": *topK, "context_budget": *contextBudget, "embedder": toEmbedCfg(e)})
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runAnswer(args []string) error {
	fs := flag.NewFlagSet("answer", flag.ContinueOnError)
	c := bindCommon(fs)
	queries := fs.String("queries", "", "path to queries.jsonl")
	qrels := fs.String("qrels", "", "path to qrels.tsv/jsonl")
	predictions := fs.String("predictions", "", "path to predictions.jsonl")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("answer", c)
	res, err := eval.EvaluateAnswer(eval.AnswerOptions{QueriesPath: *queries, QrelsPath: *qrels, PredictionsPath: *predictions})
	if err != nil {
		return err
	}
	gate := eval.EvaluateGate(c.gateMode, res.Warnings)
	run := buildRun("answer", runID, map[string]string{"queries": *queries, "qrels": *qrels, "predictions": *predictions}, nil)
	return eval.WriteReport(outDir, run, res.Metrics, res.PerQuery, gate)
}

func runAll(args []string) error {
	fs := flag.NewFlagSet("all", flag.ContinueOnError)
	c := bindCommon(fs)
	e := bindEmbed(fs)
	chunks := fs.String("chunks", "", "path to chunks.jsonl")
	queries := fs.String("queries", "", "path to queries.jsonl")
	qrels := fs.String("qrels", "", "path to qrels.tsv/jsonl")
	predictions := fs.String("predictions", "", "path to predictions.jsonl")
	runPath := fs.String("run", "", "path to run.jsonl")
	embeddings := fs.String("embeddings", "", "path to embeddings.jsonl")
	online := fs.Bool("online", false, "run optional online stages")
	topK := fs.Int("top-k", 20, "top-k")
	contextBudget := fs.Int("context-budget", 4000, "context budget")
	shortLimit := fs.Int("short-limit", 80, "short threshold")
	longLimit := fs.Int("long-limit", 1200, "long threshold")
	if err := fs.Parse(args); err != nil {
		return err
	}
	outDir, runID := normalizeOutDir("all", c)
	metrics, per, warnings, err := eval.EvaluateAll(eval.AllOptions{
		RunID:       runID,
		OutDir:      outDir,
		GateMode:    c.gateMode,
		Ingestion:   eval.IngestionOptions{ChunksPath: *chunks},
		Chunking:    eval.ChunkingOptions{ChunksPath: *chunks, ShortLimit: *shortLimit, LongLimit: *longLimit},
		Embedding:   eval.EmbeddingOptions{ChunksPath: *chunks, EmbeddingsPath: *embeddings, Online: *online, Embedder: toEmbedCfg(e)},
		Retrieval:   eval.RetrievalOptions{QueriesPath: *queries, QrelsPath: *qrels, RunPath: *runPath, ChunksPath: *chunks, EmbeddingsPath: *embeddings, Online: *online, TopK: *topK, Embedder: toEmbedCfg(e)},
		PostRetr:    eval.PostRetrievalOptions{QueriesPath: *queries, QrelsPath: *qrels, RunPath: *runPath, ChunksPath: *chunks, EmbeddingsPath: *embeddings, Online: *online, TopK: *topK, ContextBudget: *contextBudget, Embedder: toEmbedCfg(e)},
		Answer:      eval.AnswerOptions{QueriesPath: *queries, QrelsPath: *qrels, PredictionsPath: *predictions},
		WriteOnline: true,
	})
	if err != nil {
		return err
	}
	gate := eval.EvaluateGate(c.gateMode, warnings)
	run := buildRun("all", runID, map[string]string{"chunks": *chunks, "queries": *queries, "qrels": *qrels, "predictions": *predictions, "run": *runPath, "embeddings": *embeddings}, map[string]any{"online": *online, "top_k": *topK, "context_budget": *contextBudget, "short_limit": *shortLimit, "long_limit": *longLimit, "embedder": toEmbedCfg(e)})
	return eval.WriteReport(outDir, run, metrics, per, gate)
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var v int
	if _, err := fmt.Sscanf(value, "%d", &v); err != nil || v <= 0 {
		return fallback
	}
	return v
}

func gitCommitSafe() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
