package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"onebookai/pkg/ai"
)

func EvaluateEmbedding(opts EmbeddingOptions) (EvalResult, []EmbeddingRecord, error) {
	records := make([]EmbeddingRecord, 0)
	warnings := make([]string, 0)
	latencies := make([]float64, 0)

	if opts.Online {
		if strings.TrimSpace(opts.ChunksPath) == "" {
			return EvalResult{}, nil, fmt.Errorf("chunks path required in online mode")
		}
		chunks, err := ReadChunksJSONL(opts.ChunksPath)
		if err != nil {
			return EvalResult{}, nil, err
		}
		embedder, err := buildEmbedder(opts.Embedder)
		if err != nil {
			return EvalResult{}, nil, err
		}
		records, latencies, warnings, err = embedChunksOnline(chunks, embedder)
		if err != nil {
			return EvalResult{}, nil, err
		}
	} else {
		if strings.TrimSpace(opts.EmbeddingsPath) == "" {
			return EvalResult{}, nil, fmt.Errorf("embeddings path required in offline mode")
		}
		loaded, err := ReadEmbeddingsJSONL(opts.EmbeddingsPath)
		if err != nil {
			return EvalResult{}, nil, err
		}
		records = loaded
	}

	metrics := map[string]any{"total_embeddings": len(records)}
	if len(records) == 0 {
		metrics["embed_success_rate"] = 0.0
		metrics["dim_mismatch_rate"] = 0.0
		metrics["empty_vector_rate"] = 0.0
		metrics["norm_mean"] = 0.0
		metrics["norm_p50"] = 0.0
		metrics["norm_p95"] = 0.0
		metrics["latency_ms_mean"] = 0.0
		metrics["latency_ms_p95"] = 0.0
		return EvalResult{Metrics: metrics, Warnings: warnings}, records, nil
	}

	dimMismatch := 0
	emptyVec := 0
	norms := make([]float64, 0, len(records))
	per := make([]map[string]any, 0, len(records))
	expectedDim := opts.Embedder.Dim
	for _, rec := range records {
		if len(rec.Vector) == 0 {
			emptyVec++
		}
		if expectedDim > 0 && len(rec.Vector) > 0 && len(rec.Vector) != expectedDim {
			dimMismatch++
		}
		norm := vectorNorm(rec.Vector)
		norms = append(norms, norm)
		per = append(per, map[string]any{
			"id":           rec.ID,
			"dim":          len(rec.Vector),
			"norm":         norm,
			"empty_vector": len(rec.Vector) == 0,
		})
	}

	metrics["embed_success_rate"] = float64(len(records)-emptyVec) / float64(len(records))
	metrics["dim_mismatch_rate"] = float64(dimMismatch) / float64(len(records))
	metrics["empty_vector_rate"] = float64(emptyVec) / float64(len(records))
	metrics["norm_mean"] = mean(norms)
	metrics["norm_p50"] = percentile(norms, 0.5)
	metrics["norm_p95"] = percentile(norms, 0.95)
	if len(latencies) > 0 {
		metrics["latency_ms_mean"] = mean(latencies)
		metrics["latency_ms_p95"] = percentile(latencies, 0.95)
	} else {
		metrics["latency_ms_mean"] = 0.0
		metrics["latency_ms_p95"] = 0.0
	}
	if expectedDim > 0 {
		metrics["expected_dim"] = expectedDim
	}
	warnings = append(warnings, evaluateEmbeddingWarnings(metrics)...)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: uniqueStrings(warnings)}, records, nil
}

func embedChunksOnline(chunks []ChunkRecord, embedder ai.Embedder) ([]EmbeddingRecord, []float64, []string, error) {
	records := make([]EmbeddingRecord, 0, len(chunks))
	warnings := make([]string, 0)
	latencies := make([]float64, 0, len(chunks))

	if be, ok := embedder.(ai.BatchEmbedder); ok {
		texts := make([]string, 0, len(chunks))
		ids := make([]string, 0, len(chunks))
		for _, c := range chunks {
			if strings.TrimSpace(c.Text) == "" {
				records = append(records, EmbeddingRecord{ID: c.ChunkID})
				continue
			}
			texts = append(texts, c.Text)
			ids = append(ids, c.ChunkID)
		}
		if len(texts) > 0 {
			start := time.Now()
			vectors, err := be.EmbedTexts(context.Background(), texts, "RETRIEVAL_DOCUMENT")
			latencies = append(latencies, float64(time.Since(start).Milliseconds()))
			if err == nil && len(vectors) == len(texts) {
				for i, vec := range vectors {
					records = append(records, EmbeddingRecord{ID: ids[i], Vector: vec})
				}
				return records, latencies, warnings, nil
			}
			warnings = append(warnings, "batch embedding unavailable, fallback to single embedding")
			records = records[:0]
		}
	}

	for _, c := range chunks {
		if strings.TrimSpace(c.Text) == "" {
			records = append(records, EmbeddingRecord{ID: c.ChunkID})
			continue
		}
		start := time.Now()
		vec, err := embedder.EmbedText(context.Background(), c.Text, "RETRIEVAL_DOCUMENT")
		latencies = append(latencies, float64(time.Since(start).Milliseconds()))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("embed failed for %s: %v", c.ChunkID, err))
			records = append(records, EmbeddingRecord{ID: c.ChunkID})
			continue
		}
		records = append(records, EmbeddingRecord{ID: c.ChunkID, Vector: vec})
	}
	return records, latencies, warnings, nil
}

func buildEmbedder(cfg EmbedderConfig) (ai.Embedder, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "ollama"
	}
	switch provider {
	case "ollama":
		if strings.TrimSpace(cfg.Model) == "" {
			return nil, fmt.Errorf("embedding model required for ollama")
		}
		return ai.NewOllamaEmbedder(ai.NewOllamaClient(cfg.BaseURL), cfg.Model, cfg.Dim), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

func evaluateEmbeddingWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	mismatch := metricFloat(metrics, "dim_mismatch_rate")
	empty := metricFloat(metrics, "empty_vector_rate")
	if mismatch > 0 {
		warnings = append(warnings, fmt.Sprintf("dim_mismatch_rate %.4f is non-zero", mismatch))
	}
	if empty > 0.01 {
		warnings = append(warnings, fmt.Sprintf("empty_vector_rate %.4f exceeds threshold 0.01", empty))
	}
	return warnings
}
