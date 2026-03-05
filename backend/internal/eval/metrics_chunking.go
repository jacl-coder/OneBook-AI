package eval

import (
	"fmt"
	"strings"
)

func EvaluateChunking(opts ChunkingOptions) (EvalResult, error) {
	if strings.TrimSpace(opts.ChunksPath) == "" {
		return EvalResult{}, fmt.Errorf("chunks path required")
	}
	chunks, err := ReadChunksJSONL(opts.ChunksPath)
	if err != nil {
		return EvalResult{}, err
	}
	shortLimit := opts.ShortLimit
	if shortLimit <= 0 {
		shortLimit = 80
	}
	longLimit := opts.LongLimit
	if longLimit <= 0 {
		longLimit = 1200
	}

	metrics := map[string]any{"total_chunks": len(chunks), "short_limit": shortLimit, "long_limit": longLimit}
	if len(chunks) == 0 {
		metrics["length_p50"] = 0.0
		metrics["length_p95"] = 0.0
		metrics["too_short_rate"] = 0.0
		metrics["too_long_rate"] = 0.0
		metrics["boundary_punct_ok_rate"] = 0.0
		return EvalResult{Metrics: metrics}, nil
	}

	lengths := make([]float64, 0, len(chunks))
	shortCount := 0
	longCount := 0
	boundaryOK := 0
	per := make([]map[string]any, 0, len(chunks))
	for _, c := range chunks {
		length := estimateLengthUnits(c.Text)
		lengths = append(lengths, float64(length))
		if length < shortLimit {
			shortCount++
		}
		if length > longLimit {
			longCount++
		}
		ok := hasGoodBoundary(c.Text)
		if ok {
			boundaryOK++
		}
		per = append(per, map[string]any{
			"id":                c.ChunkID,
			"doc_id":            c.DocID,
			"length":            length,
			"too_short":         length < shortLimit,
			"too_long":          length > longLimit,
			"boundary_punct_ok": ok,
		})
	}
	metrics["length_p50"] = percentile(lengths, 0.5)
	metrics["length_p95"] = percentile(lengths, 0.95)
	metrics["too_short_rate"] = float64(shortCount) / float64(len(chunks))
	metrics["too_long_rate"] = float64(longCount) / float64(len(chunks))
	metrics["boundary_punct_ok_rate"] = float64(boundaryOK) / float64(len(chunks))

	warnings := evaluateChunkingWarnings(metrics)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: warnings}, nil
}

func hasGoodBoundary(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	runes := []rune(trimmed)
	last := runes[len(runes)-1]
	good := "。！？!?；;:：,，、)]】）}\"'”"
	return strings.ContainsRune(good, last)
}

func evaluateChunkingWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	shortRate := metricFloat(metrics, "too_short_rate")
	longRate := metricFloat(metrics, "too_long_rate")
	boundary := metricFloat(metrics, "boundary_punct_ok_rate")
	if shortRate > 0.3 {
		warnings = append(warnings, fmt.Sprintf("too_short_rate %.4f exceeds threshold 0.30", shortRate))
	}
	if longRate > 0.1 {
		warnings = append(warnings, fmt.Sprintf("too_long_rate %.4f exceeds threshold 0.10", longRate))
	}
	if boundary < 0.6 {
		warnings = append(warnings, fmt.Sprintf("boundary_punct_ok_rate %.4f below threshold 0.60", boundary))
	}
	return warnings
}
