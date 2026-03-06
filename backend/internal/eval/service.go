package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CenterArtifact struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type CenterStageSummary struct {
	Stage   string         `json:"stage"`
	Metrics map[string]any `json:"metrics"`
}

type CenterRunBundle struct {
	Metrics        map[string]any
	PerQuery       []map[string]any
	Warnings       []string
	Gate           GateSummary
	Artifacts      []CenterArtifact
	StageSummaries []CenterStageSummary
}

type CenterRunOptions struct {
	RunID       string
	Command     string
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

func ExecuteCenterRun(opts CenterRunOptions) (CenterRunBundle, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		opts.OutDir = filepath.Join(".cache", "rag-eval", "center", defaultRunID())
	}
	if strings.TrimSpace(opts.RunID) == "" {
		opts.RunID = defaultRunID()
	}
	if strings.TrimSpace(opts.Command) == "" {
		opts.Command = "all"
	}
	if err := ensureDir(opts.OutDir); err != nil {
		return CenterRunBundle{}, err
	}

	metrics := map[string]any{}
	perQuery := make([]map[string]any, 0, 256)
	warnings := make([]string, 0)
	stageSummaries := make([]CenterStageSummary, 0, 8)
	artifacts := make([]CenterArtifact, 0, 8)

	appendResult := func(stage string, result EvalResult) {
		metrics[stage] = result.Metrics
		warnings = append(warnings, result.Warnings...)
		perQuery = append(perQuery, prefixRows(stage, result.PerQuery)...)
		stageSummaries = append(stageSummaries, CenterStageSummary{Stage: stage, Metrics: result.Metrics})
	}

	switch opts.Command {
	case "retrieval":
		retrieval, err := EvaluateRetrievalDetailed(opts.Retrieval)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("retrieval", retrieval.Result)
		for stage, runs := range retrieval.StageRuns {
			path := filepath.Join(opts.OutDir, stage+"_run.jsonl")
			if err := writeRunJSONL(path, runs); err != nil {
				return CenterRunBundle{}, err
			}
			artifact, err := statArtifact(stage+"_run.jsonl", path, "application/x-ndjson")
			if err != nil {
				return CenterRunBundle{}, err
			}
			artifacts = append(artifacts, artifact)
			if stageMetrics, ok := lookupStageMetrics(resultStageMap(retrieval.Result.Metrics), stage); ok {
				stageSummaries = append(stageSummaries, CenterStageSummary{Stage: "retrieval." + stage, Metrics: stageMetrics})
			}
		}
	case "post_retrieval":
		result, _, err := EvaluatePostRetrieval(opts.PostRetr)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("post_retrieval", result)
	case "answer":
		result, err := EvaluateAnswer(opts.Answer)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("answer", result)
	default:
		ingest, err := EvaluateIngestion(opts.Ingestion)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("ingestion", ingest)

		chunking, err := EvaluateChunking(opts.Chunking)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("chunking", chunking)

		embedding, embeddings, err := EvaluateEmbedding(opts.Embedding)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("embedding", embedding)
		if opts.Embedding.Online && opts.WriteOnline && len(embeddings) > 0 {
			path := filepath.Join(opts.OutDir, "generated_embeddings.jsonl")
			if err := writeEmbeddingsJSONL(path, embeddings); err != nil {
				return CenterRunBundle{}, err
			}
			artifact, err := statArtifact("generated_embeddings.jsonl", path, "application/x-ndjson")
			if err != nil {
				return CenterRunBundle{}, err
			}
			artifacts = append(artifacts, artifact)
			opts.Retrieval.EmbeddingsPath = path
			opts.PostRetr.EmbeddingsPath = path
		}

		retrieval, err := EvaluateRetrievalDetailed(opts.Retrieval)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("retrieval", retrieval.Result)
		for stage, runs := range retrieval.StageRuns {
			path := filepath.Join(opts.OutDir, stage+"_run.jsonl")
			if err := writeRunJSONL(path, runs); err != nil {
				return CenterRunBundle{}, err
			}
			artifact, err := statArtifact(stage+"_run.jsonl", path, "application/x-ndjson")
			if err != nil {
				return CenterRunBundle{}, err
			}
			artifacts = append(artifacts, artifact)
			if stageMetrics, ok := lookupStageMetrics(resultStageMap(retrieval.Result.Metrics), stage); ok {
				stageSummaries = append(stageSummaries, CenterStageSummary{Stage: "retrieval." + stage, Metrics: stageMetrics})
			}
		}
		if finalRun := retrieval.StageRuns[finalRetrievalStage(opts.Retrieval)]; len(finalRun) > 0 {
			path := filepath.Join(opts.OutDir, "generated_run.jsonl")
			if err := writeRunJSONL(path, finalRun); err != nil {
				return CenterRunBundle{}, err
			}
			opts.PostRetr.RunPath = path
			artifact, err := statArtifact("generated_run.jsonl", path, "application/x-ndjson")
			if err != nil {
				return CenterRunBundle{}, err
			}
			artifacts = append(artifacts, artifact)
		}

		postRetrieval, _, err := EvaluatePostRetrieval(opts.PostRetr)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("post_retrieval", postRetrieval)

		answer, err := EvaluateAnswer(opts.Answer)
		if err != nil {
			return CenterRunBundle{}, err
		}
		appendResult("answer", answer)
	}

	warnings = uniqueStrings(warnings)
	gate := EvaluateGate(opts.GateMode, warnings)
	metrics["warnings"] = warnings
	metrics["generated_at"] = time.Now().UTC().Format(time.RFC3339)
	metrics["gate_result"] = gate

	run := ReportRun{
		RunID:     opts.RunID,
		Command:   opts.Command,
		CreatedAt: time.Now().UTC(),
		Inputs: map[string]string{
			"chunks":      opts.Ingestion.ChunksPath,
			"queries":     opts.Retrieval.QueriesPath,
			"qrels":       opts.Retrieval.QrelsPath,
			"predictions": opts.Answer.PredictionsPath,
		},
		Params: map[string]any{
			"gateMode":      opts.GateMode,
			"retrievalMode": opts.Retrieval.RetrievalMode,
			"topK":          opts.Retrieval.TopK,
			"contextBudget": opts.PostRetr.ContextBudget,
		},
		GitCommit: gitCommit(),
	}
	if err := writeJSON(filepath.Join(opts.OutDir, "run.json"), run); err != nil {
		return CenterRunBundle{}, err
	}
	if err := writeJSON(filepath.Join(opts.OutDir, "metrics.json"), metrics); err != nil {
		return CenterRunBundle{}, err
	}
	if err := writeJSONL(filepath.Join(opts.OutDir, "per_query.jsonl"), perQuery); err != nil {
		return CenterRunBundle{}, err
	}
	runArtifact, err := statArtifact("run.json", filepath.Join(opts.OutDir, "run.json"), "application/json")
	if err != nil {
		return CenterRunBundle{}, err
	}
	metricsArtifact, err := statArtifact("metrics.json", filepath.Join(opts.OutDir, "metrics.json"), "application/json")
	if err != nil {
		return CenterRunBundle{}, err
	}
	perQueryArtifact, err := statArtifact("per_query.jsonl", filepath.Join(opts.OutDir, "per_query.jsonl"), "application/x-ndjson")
	if err != nil {
		return CenterRunBundle{}, err
	}
	artifacts = append([]CenterArtifact{runArtifact, metricsArtifact, perQueryArtifact}, artifacts...)

	return CenterRunBundle{
		Metrics:        metrics,
		PerQuery:       perQuery,
		Warnings:       warnings,
		Gate:           gate,
		Artifacts:      artifacts,
		StageSummaries: stageSummaries,
	}, nil
}

func statArtifact(name, path, contentType string) (CenterArtifact, error) {
	info, err := os.Stat(path)
	if err != nil {
		return CenterArtifact{}, err
	}
	return CenterArtifact{
		Name:        name,
		Path:        path,
		ContentType: contentType,
		SizeBytes:   info.Size(),
	}, nil
}

func resultStageMap(metrics map[string]any) map[string]any {
	stageMetrics, _ := metrics["stages"].(map[string]any)
	return stageMetrics
}

func lookupStageMetrics(parent map[string]any, key string) (map[string]any, bool) {
	if parent == nil {
		return nil, false
	}
	child, ok := parent[key].(map[string]any)
	return child, ok
}

func ParsePerQueryFile(path string) ([]map[string]any, error) {
	rows, err := readJSONLMaps(path)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]any, len(row))
		for key, value := range row {
			var decoded any
			if err := json.Unmarshal(value, &decoded); err != nil {
				continue
			}
			item[key] = decoded
		}
		out = append(out, item)
	}
	return out, nil
}
