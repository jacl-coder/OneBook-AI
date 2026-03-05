package eval

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// AllOptions configures the all-in-one evaluator runner.
type AllOptions struct {
	RunID       string
	OutDir      string
	GateMode    string
	Ingestion   IngestionOptions
	Chunking    ChunkingOptions
	Embedding   EmbeddingOptions
	Retrieval   RetrievalOptions
	PostRetr    PostRetrievalOptions
	Answer      AnswerOptions
	WriteOnline bool
}

func EvaluateAll(opts AllOptions) (map[string]any, []map[string]any, []string, error) {
	metrics := map[string]any{}
	per := make([]map[string]any, 0, 1024)
	warnings := make([]string, 0)

	ingest, err := EvaluateIngestion(opts.Ingestion)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ingestion eval: %w", err)
	}
	metrics["ingestion"] = ingest.Metrics
	warnings = append(warnings, ingest.Warnings...)
	per = append(per, prefixRows("ingestion", ingest.PerQuery)...)

	chunking, err := EvaluateChunking(opts.Chunking)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("chunking eval: %w", err)
	}
	metrics["chunking"] = chunking.Metrics
	warnings = append(warnings, chunking.Warnings...)
	per = append(per, prefixRows("chunking", chunking.PerQuery)...)

	embedding, embeddings, err := EvaluateEmbedding(opts.Embedding)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("embedding eval: %w", err)
	}
	metrics["embedding"] = embedding.Metrics
	warnings = append(warnings, embedding.Warnings...)
	per = append(per, prefixRows("embedding", embedding.PerQuery)...)
	if opts.Embedding.Online && opts.WriteOnline && len(embeddings) > 0 {
		embPath := filepath.Join(opts.OutDir, "generated_embeddings.jsonl")
		if err := writeEmbeddingsJSONL(embPath, embeddings); err == nil {
			embedding.Metrics["generated_embeddings_path"] = embPath
			opts.Retrieval.EmbeddingsPath = embPath
			opts.PostRetr.EmbeddingsPath = embPath
		}
	}

	retrieval, runs, err := EvaluateRetrieval(opts.Retrieval)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("retrieval eval: %w", err)
	}
	metrics["retrieval"] = retrieval.Metrics
	warnings = append(warnings, retrieval.Warnings...)
	per = append(per, prefixRows("retrieval", retrieval.PerQuery)...)
	if opts.Retrieval.Online && opts.WriteOnline && len(runs) > 0 {
		runPath := filepath.Join(opts.OutDir, "generated_run.jsonl")
		if err := writeRunJSONL(runPath, runs); err == nil {
			retrieval.Metrics["generated_run_path"] = runPath
			opts.PostRetr.RunPath = runPath
		}
	}

	post, postRuns, err := EvaluatePostRetrieval(opts.PostRetr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("post-retrieval eval: %w", err)
	}
	_ = postRuns
	metrics["post_retrieval"] = post.Metrics
	warnings = append(warnings, post.Warnings...)
	per = append(per, prefixRows("post_retrieval", post.PerQuery)...)

	answer, err := EvaluateAnswer(opts.Answer)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("answer eval: %w", err)
	}
	metrics["answer"] = answer.Metrics
	warnings = append(warnings, answer.Warnings...)
	per = append(per, prefixRows("answer", answer.PerQuery)...)

	metrics["warnings"] = uniqueStrings(warnings)
	metrics["generated_at"] = time.Now().UTC().Format(time.RFC3339)
	return metrics, per, uniqueStrings(warnings), nil
}

func prefixRows(module string, rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		copy := map[string]any{"module": strings.TrimSpace(module)}
		for k, v := range row {
			copy[k] = v
		}
		out = append(out, copy)
	}
	return out
}
