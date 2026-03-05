package eval

import (
	"fmt"
	"strings"
)

func EvaluateIngestion(opts IngestionOptions) (EvalResult, error) {
	if strings.TrimSpace(opts.ChunksPath) == "" {
		return EvalResult{}, fmt.Errorf("chunks path required")
	}
	chunks, err := ReadChunksJSONL(opts.ChunksPath)
	if err != nil {
		return EvalResult{}, err
	}
	metrics := map[string]any{
		"total_chunks": len(chunks),
	}
	if len(chunks) == 0 {
		metrics["empty_rate"] = 0.0
		metrics["duplicate_rate_exact"] = 0.0
		metrics["metadata_missing_rate"] = 0.0
		metrics["noise_marker_rate"] = 0.0
		return EvalResult{Metrics: metrics}, nil
	}

	emptyCount := 0
	metaMissing := 0
	noiseChars := 0
	totalChars := 0
	dupMap := map[string]int{}
	per := make([]map[string]any, 0, len(chunks))

	for _, c := range chunks {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			emptyCount++
		}
		norm := normalizeTextForDup(text)
		if norm != "" {
			dupMap[norm]++
		}
		if c.Metadata == nil || c.Metadata["source_type"] == "" || c.Metadata["source_ref"] == "" || c.Metadata["extract_method"] == "" {
			metaMissing++
		}
		runes := []rune(text)
		totalChars += len(runes)
		chunkNoise := 0
		for _, r := range runes {
			if r == '\uFFFD' || r == '�' {
				chunkNoise++
			}
		}
		noiseChars += chunkNoise
		per = append(per, map[string]any{
			"id":                 c.ChunkID,
			"doc_id":             c.DocID,
			"empty":              text == "",
			"metadata_missing":   c.Metadata == nil || c.Metadata["source_type"] == "" || c.Metadata["source_ref"] == "" || c.Metadata["extract_method"] == "",
			"noise_marker_count": chunkNoise,
		})
	}

	dupCount := 0
	for _, n := range dupMap {
		if n > 1 {
			dupCount += n - 1
		}
	}

	emptyRate := float64(emptyCount) / float64(len(chunks))
	dupRate := float64(dupCount) / float64(len(chunks))
	metaRate := float64(metaMissing) / float64(len(chunks))
	noiseRate := 0.0
	if totalChars > 0 {
		noiseRate = float64(noiseChars) / float64(totalChars)
	}

	metrics["empty_rate"] = emptyRate
	metrics["duplicate_rate_exact"] = dupRate
	metrics["metadata_missing_rate"] = metaRate
	metrics["noise_marker_rate"] = noiseRate

	warnings := evaluateIngestionWarnings(metrics)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: warnings}, nil
}

func evaluateIngestionWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	empty := metricFloat(metrics, "empty_rate")
	dup := metricFloat(metrics, "duplicate_rate_exact")
	meta := metricFloat(metrics, "metadata_missing_rate")
	if empty > 0.001 {
		warnings = append(warnings, fmt.Sprintf("empty_rate %.4f exceeds threshold 0.001", empty))
	}
	if dup > 0.02 {
		warnings = append(warnings, fmt.Sprintf("duplicate_rate_exact %.4f exceeds threshold 0.02", dup))
	}
	if meta > 0.01 {
		warnings = append(warnings, fmt.Sprintf("metadata_missing_rate %.4f exceeds threshold 0.01", meta))
	}
	return warnings
}
