package eval

import (
	"fmt"
	"strings"
)

func EvaluatePostRetrieval(opts PostRetrievalOptions) (EvalResult, []RunEntry, error) {
	if strings.TrimSpace(opts.QueriesPath) == "" {
		return EvalResult{}, nil, fmt.Errorf("queries path required")
	}
	if strings.TrimSpace(opts.QrelsPath) == "" {
		return EvalResult{}, nil, fmt.Errorf("qrels path required")
	}
	queries, err := ReadQueriesJSONL(opts.QueriesPath)
	if err != nil {
		return EvalResult{}, nil, err
	}
	qrels, err := ReadQrels(opts.QrelsPath)
	if err != nil {
		return EvalResult{}, nil, err
	}

	runs := []RunEntry{}
	warnings := make([]string, 0)
	if strings.TrimSpace(opts.RunPath) != "" {
		runs, err = ReadRunJSONL(opts.RunPath)
		if err != nil {
			return EvalResult{}, nil, err
		}
	} else {
		detailed, err := EvaluateRetrievalDetailed(RetrievalOptions{
			QueriesPath:    opts.QueriesPath,
			QrelsPath:      opts.QrelsPath,
			ChunksPath:     opts.ChunksPath,
			EmbeddingsPath: opts.EmbeddingsPath,
			Online:         opts.Online,
			TopK:           opts.TopK,
			Embedder:       opts.Embedder,
		})
		if err != nil {
			return EvalResult{}, nil, err
		}
		runs = detailed.StageRuns[finalRetrievalStage(RetrievalOptions{RetrievalMode: opts.RetrievalMode, TopK: opts.TopK})]
		warnings = append(warnings, detailed.Result.Warnings...)
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}
	contextBudget := opts.ContextBudget
	if contextBudget <= 0 {
		contextBudget = 4000
	}

	relMap := buildRelevantMap(qrels)
	runByQ := map[string]RunEntry{}
	for _, run := range runs {
		runByQ[run.QID] = run
	}
	lenByID := map[string]int{}
	if strings.TrimSpace(opts.ChunksPath) != "" {
		chunks, err := ReadChunksJSONL(opts.ChunksPath)
		if err == nil {
			for _, c := range chunks {
				lenByID[c.ChunkID] = estimateLengthUnits(c.Text)
			}
		} else {
			warnings = append(warnings, "failed to load chunks for context budget; fallback length=1")
		}
	} else {
		warnings = append(warnings, "chunks path not provided for context budget; fallback length=1")
	}

	dupRates := make([]float64, 0, len(queries))
	diversities := make([]float64, 0, len(queries))
	coverages := make([]float64, 0, len(queries))
	budgets := make([]float64, 0, len(queries))
	packedCounts := make([]float64, 0, len(queries))
	packedRelevantCounts := make([]float64, 0, len(queries))
	per := make([]map[string]any, 0, len(queries))

	for _, q := range queries {
		run := runByQ[q.QID]
		hits := run.Results
		if topK < len(hits) {
			hits = hits[:topK]
		}
		if len(hits) == 0 {
			dupRates = append(dupRates, 0)
			diversities = append(diversities, 0)
			coverages = append(coverages, 0)
			budgets = append(budgets, 0)
			per = append(per, map[string]any{"qid": q.QID})
			continue
		}

		packed := packPostRetrievalHits(hits, lenByID, contextBudget)
		seen := map[string]struct{}{}
		dup := 0
		ctxLen := 0
		relevantCount := 0
		for _, h := range packed {
			if _, ok := seen[h.DocID]; ok {
				dup++
			}
			seen[h.DocID] = struct{}{}
			l := lenByID[h.DocID]
			if l <= 0 {
				l = 1
			}
			ctxLen += l
			if relMap[q.QID][h.DocID] > 0 {
				relevantCount++
			}
		}
		dupRate := safeDiv(float64(dup), float64(len(packed)))
		diversity := float64(len(seen))
		coverage, _ := recallAndHitAtK(packed, relMap[q.QID], len(packed))
		budgetUtil := float64(ctxLen) / float64(contextBudget)

		dupRates = append(dupRates, dupRate)
		diversities = append(diversities, diversity)
		coverages = append(coverages, coverage)
		budgets = append(budgets, budgetUtil)
		packedCounts = append(packedCounts, float64(len(packed)))
		packedRelevantCounts = append(packedRelevantCounts, float64(relevantCount))
		per = append(per, map[string]any{
			"qid":                        q.QID,
			"retrieved_dup_rate":         dupRate,
			"doc_diversity":              diversity,
			"context_coverage":           coverage,
			"context_budget_utilization": budgetUtil,
			"packed_chunk_count":         len(packed),
			"packed_relevant_count":      relevantCount,
		})
	}

	metrics := map[string]any{
		"queries":                    len(queries),
		"top_k":                      topK,
		"context_budget":             contextBudget,
		"retrieved_dup_rate":         mean(dupRates),
		"doc_diversity":              mean(diversities),
		"context_coverage":           mean(coverages),
		"context_budget_utilization": mean(budgets),
		"packed_chunk_count":         mean(packedCounts),
		"packed_relevant_count":      mean(packedRelevantCounts),
	}
	warnings = append(warnings, evaluatePostRetrievalWarnings(metrics)...)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: uniqueStrings(warnings)}, runs, nil
}

func packPostRetrievalHits(hits []RunHit, lenByID map[string]int, contextBudget int) []RunHit {
	if len(hits) == 0 {
		return nil
	}
	if contextBudget <= 0 {
		return hits
	}
	out := make([]RunHit, 0, len(hits))
	seen := map[string]struct{}{}
	used := 0
	for _, hit := range hits {
		if _, ok := seen[hit.DocID]; ok {
			continue
		}
		seen[hit.DocID] = struct{}{}
		length := lenByID[hit.DocID]
		if length <= 0 {
			length = 1
		}
		if len(out) > 0 && used+length > contextBudget {
			break
		}
		used += length
		out = append(out, hit)
	}
	return out
}

func evaluatePostRetrievalWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	dup := metricFloat(metrics, "retrieved_dup_rate")
	coverage := metricFloat(metrics, "context_coverage")
	budget := metricFloat(metrics, "context_budget_utilization")
	if dup > 0.2 {
		warnings = append(warnings, fmt.Sprintf("retrieved_dup_rate %.4f exceeds threshold 0.20", dup))
	}
	if coverage < 0.4 {
		warnings = append(warnings, fmt.Sprintf("context_coverage %.4f below threshold 0.40", coverage))
	}
	if budget > 1.2 {
		warnings = append(warnings, fmt.Sprintf("context_budget_utilization %.4f exceeds threshold 1.20", budget))
	}
	return warnings
}
