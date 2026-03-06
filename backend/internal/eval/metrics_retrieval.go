package eval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"onebookai/pkg/retrieval"
)

func EvaluateRetrieval(opts RetrievalOptions) (EvalResult, []RunEntry, error) {
	detailed, err := EvaluateRetrievalDetailed(opts)
	if err != nil {
		return EvalResult{}, nil, err
	}
	finalStage := finalRetrievalStage(opts)
	if _, ok := detailed.StageRuns[finalStage]; !ok {
		if provided, ok := detailed.StageRuns["provided"]; ok {
			finalStage = "provided"
			return detailed.Result, provided, nil
		}
		if rerank, ok := detailed.StageRuns["rerank"]; ok {
			finalStage = "rerank"
			return detailed.Result, rerank, nil
		}
	}
	return detailed.Result, detailed.StageRuns[finalStage], nil
}

func EvaluateRetrievalDetailed(opts RetrievalOptions) (RetrievalStageResult, error) {
	if strings.TrimSpace(opts.QueriesPath) == "" {
		return RetrievalStageResult{}, fmt.Errorf("queries path required")
	}
	if strings.TrimSpace(opts.QrelsPath) == "" {
		return RetrievalStageResult{}, fmt.Errorf("qrels path required")
	}
	queries, err := ReadQueriesJSONL(opts.QueriesPath)
	if err != nil {
		return RetrievalStageResult{}, err
	}
	qrels, err := ReadQrels(opts.QrelsPath)
	if err != nil {
		return RetrievalStageResult{}, err
	}

	runs, warnings, err := loadOrBuildRetrievalStages(opts, queries)
	if err != nil {
		return RetrievalStageResult{}, err
	}

	stageMetrics := map[string]any{}
	perAll := make([]map[string]any, 0, len(queries)*len(runs))
	stageNames := orderedRetrievalStages(runs)
	for _, stage := range stageNames {
		metrics, per := computeIRMetrics(queries, qrels, runs[stage], topKForStage(opts, stage))
		stageMetrics[stage] = metrics
		for _, row := range per {
			row["stage"] = stage
			perAll = append(perAll, row)
		}
	}

	finalStage := finalRetrievalStage(opts)
	if _, ok := runs[finalStage]; !ok {
		if _, ok := runs["provided"]; ok {
			finalStage = "provided"
		} else if _, ok := runs["rerank"]; ok {
			finalStage = "rerank"
		}
	}
	metrics, _ := computeIRMetrics(queries, qrels, runs[finalStage], topKForStage(opts, finalStage))
	metrics["stages"] = stageMetrics
	metrics["final_stage"] = finalStage
	warnings = append(warnings, evaluateRetrievalWarnings(metrics)...)
	return RetrievalStageResult{
		Result:    EvalResult{Metrics: metrics, PerQuery: perAll, Warnings: uniqueStrings(warnings)},
		StageRuns: runs,
	}, nil
}

func loadOrBuildRetrievalStages(opts RetrievalOptions, queries []QueryRecord) (map[string][]RunEntry, []string, error) {
	warnings := make([]string, 0)
	if strings.TrimSpace(opts.RunPath) != "" {
		runs, err := ReadRunJSONL(opts.RunPath)
		if err != nil {
			return nil, warnings, err
		}
		return map[string][]RunEntry{"provided": runs}, warnings, nil
	}
	if !opts.Online {
		return nil, warnings, fmt.Errorf("run path required in offline mode")
	}
	if strings.TrimSpace(opts.ChunksPath) == "" {
		return nil, warnings, fmt.Errorf("chunks path required in online mode")
	}
	chunks, err := ReadChunksJSONL(opts.ChunksPath)
	if err != nil {
		return nil, warnings, err
	}
	embeddings, embWarnings, err := loadOrBuildChunkEmbeddings(opts, chunks)
	if err != nil {
		return nil, warnings, err
	}
	warnings = append(warnings, embWarnings...)
	embedder, err := buildEmbedder(opts.Embedder)
	if err != nil {
		return nil, warnings, err
	}

	docs := buildEvalDocuments(chunks, embeddings)
	if len(docs.byID) == 0 {
		return nil, warnings, fmt.Errorf("online retrieval index is empty")
	}

	denseTopK := topKOrDefault(opts.DenseTopK, opts.TopK, 20)
	sparseTopK := topKOrDefault(opts.SparseTopK, opts.TopK, 20)
	fusionTopK := topKOrDefault(opts.FusionTopK, opts.TopK, 20)
	rerankTopN := topKOrDefault(opts.RerankTopN, fusionTopK, 10)

	denseRuns := make([]RunEntry, 0, len(queries))
	sparseRuns := make([]RunEntry, 0, len(queries))
	fusionRuns := make([]RunEntry, 0, len(queries))
	rerankRuns := make([]RunEntry, 0, len(queries))

	for _, q := range queries {
		if strings.TrimSpace(q.Query) == "" {
			denseRuns = append(denseRuns, RunEntry{QID: q.QID})
			sparseRuns = append(sparseRuns, RunEntry{QID: q.QID})
			fusionRuns = append(fusionRuns, RunEntry{QID: q.QID})
			rerankRuns = append(rerankRuns, RunEntry{QID: q.QID})
			continue
		}
		query := retrieval.NormalizeText(q.Query)
		language := retrieval.DetectLanguage(query)
		qvec, err := embedder.EmbedText(context.Background(), query, "RETRIEVAL_QUERY")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("query embedding failed for %s: %v", q.QID, err))
			denseRuns = append(denseRuns, RunEntry{QID: q.QID})
			sparseRuns = append(sparseRuns, RunEntry{QID: q.QID})
			fusionRuns = append(fusionRuns, RunEntry{QID: q.QID})
			rerankRuns = append(rerankRuns, RunEntry{QID: q.QID})
			continue
		}
		denseHits := rankByCosine(qvec, docs.embedded, denseTopK)
		sparseHits := rankBySparse(retrieval.BuildSparseVector(query, language), docs.sparse, sparseTopK)
		fusedHits := fuseRunHits(denseHits, sparseHits, fusionTopK)
		rerankedHits := rerankHits(query, docs.byID, fusedHits, rerankTopN)

		denseRuns = append(denseRuns, RunEntry{QID: q.QID, Results: denseHits})
		sparseRuns = append(sparseRuns, RunEntry{QID: q.QID, Results: sparseHits})
		fusionRuns = append(fusionRuns, RunEntry{QID: q.QID, Results: fusedHits})
		rerankRuns = append(rerankRuns, RunEntry{QID: q.QID, Results: rerankedHits})
	}

	return map[string][]RunEntry{
		"dense":  denseRuns,
		"sparse": sparseRuns,
		"fusion": fusionRuns,
		"rerank": rerankRuns,
	}, warnings, nil
}

type evalDocument struct {
	ID       string
	Text     string
	Language string
	Dense    []float32
	Sparse   retrieval.SparseVector
}

type evalDocumentIndex struct {
	embedded []EmbeddingRecord
	sparse   []evalDocument
	byID     map[string]evalDocument
}

func buildEvalDocuments(chunks []ChunkRecord, embeddings map[string][]float32) evalDocumentIndex {
	out := evalDocumentIndex{
		embedded: make([]EmbeddingRecord, 0, len(chunks)),
		sparse:   make([]evalDocument, 0, len(chunks)),
		byID:     make(map[string]evalDocument, len(chunks)),
	}
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		language := strings.TrimSpace(chunk.Metadata["language"])
		if language == "" {
			language = retrieval.DetectLanguage(text)
		}
		doc := evalDocument{
			ID:       chunk.ChunkID,
			Text:     text,
			Language: language,
			Sparse:   retrieval.BuildSparseVector(text, language),
		}
		if vec := embeddings[chunk.ChunkID]; len(vec) > 0 {
			doc.Dense = vec
			out.embedded = append(out.embedded, EmbeddingRecord{ID: chunk.ChunkID, Vector: vec})
		}
		out.sparse = append(out.sparse, doc)
		out.byID[doc.ID] = doc
	}
	return out
}

func loadOrBuildChunkEmbeddings(opts RetrievalOptions, chunks []ChunkRecord) (map[string][]float32, []string, error) {
	warnings := make([]string, 0)
	if strings.TrimSpace(opts.EmbeddingsPath) != "" {
		records, err := ReadEmbeddingsJSONL(opts.EmbeddingsPath)
		if err != nil {
			return nil, warnings, err
		}
		index := make(map[string][]float32, len(records))
		for _, r := range records {
			index[r.ID] = r.Vector
		}
		return index, warnings, nil
	}
	embedder, err := buildEmbedder(opts.Embedder)
	if err != nil {
		return nil, warnings, err
	}
	index := make(map[string][]float32, len(chunks))
	for _, c := range chunks {
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		vec, err := embedder.EmbedText(context.Background(), c.Text, "RETRIEVAL_DOCUMENT")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("chunk embedding failed for %s: %v", c.ChunkID, err))
			continue
		}
		index[c.ChunkID] = vec
	}
	return index, warnings, nil
}

func rankByCosine(query []float32, docs []EmbeddingRecord, topK int) []RunHit {
	type pair struct {
		id    string
		score float64
	}
	pairs := make([]pair, 0, len(docs))
	for _, doc := range docs {
		score, ok := cosineSimilarity(query, doc.Vector)
		if !ok {
			continue
		}
		pairs = append(pairs, pair{id: doc.ID, score: score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].id < pairs[j].id
		}
		return pairs[i].score > pairs[j].score
	})
	if topK > len(pairs) {
		topK = len(pairs)
	}
	out := make([]RunHit, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, RunHit{DocID: pairs[i].id, Score: pairs[i].score})
	}
	return out
}

func rankBySparse(query retrieval.SparseVector, docs []evalDocument, topK int) []RunHit {
	type pair struct {
		id    string
		score float64
	}
	pairs := make([]pair, 0, len(docs))
	for _, doc := range docs {
		score := sparseSimilarity(query, doc.Sparse)
		if score <= 0 {
			continue
		}
		pairs = append(pairs, pair{id: doc.ID, score: score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score == pairs[j].score {
			return pairs[i].id < pairs[j].id
		}
		return pairs[i].score > pairs[j].score
	})
	if topK > len(pairs) {
		topK = len(pairs)
	}
	out := make([]RunHit, 0, topK)
	for i := 0; i < topK; i++ {
		out = append(out, RunHit{DocID: pairs[i].id, Score: pairs[i].score})
	}
	return out
}

func sparseSimilarity(a, b retrieval.SparseVector) float64 {
	if len(a.Indices) == 0 || len(b.Indices) == 0 {
		return 0
	}
	i, j := 0, 0
	score := 0.0
	for i < len(a.Indices) && j < len(b.Indices) {
		switch {
		case a.Indices[i] == b.Indices[j]:
			score += float64(a.Values[i] * b.Values[j])
			i++
			j++
		case a.Indices[i] < b.Indices[j]:
			i++
		default:
			j++
		}
	}
	return score
}

func fuseRunHits(dense, sparse []RunHit, topK int) []RunHit {
	scores := map[string]float64{}
	for i, hit := range dense {
		scores[hit.DocID] += 1.0 / float64(i+60)
	}
	for i, hit := range sparse {
		scores[hit.DocID] += 1.0 / float64(i+60)
	}
	out := make([]RunHit, 0, len(scores))
	for id, score := range scores {
		out = append(out, RunHit{DocID: id, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].DocID < out[j].DocID
		}
		return out[i].Score > out[j].Score
	})
	if topK > len(out) {
		topK = len(out)
	}
	return out[:topK]
}

func rerankHits(query string, docs map[string]evalDocument, hits []RunHit, limit int) []RunHit {
	language := retrieval.DetectLanguage(query)
	queryTokens := retrieval.Tokenize(query, language)
	querySet := map[string]struct{}{}
	for _, token := range queryTokens {
		querySet[token] = struct{}{}
	}
	scored := make([]RunHit, 0, len(hits))
	for _, hit := range hits {
		doc, ok := docs[hit.DocID]
		if !ok {
			continue
		}
		docTokens := retrieval.Tokenize(doc.Text, language)
		overlap := 0
		for _, token := range docTokens {
			if _, exists := querySet[token]; exists {
				overlap++
			}
		}
		boost := safeDiv(float64(overlap), float64(maxInt(len(docTokens), 1)))
		scored = append(scored, RunHit{DocID: hit.DocID, Score: hit.Score + (boost * 0.5)})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].DocID < scored[j].DocID
		}
		return scored[i].Score > scored[j].Score
	})
	if limit > len(scored) {
		limit = len(scored)
	}
	return scored[:limit]
}

func computeIRMetrics(queries []QueryRecord, qrels []QRel, runs []RunEntry, topK int) (map[string]any, []map[string]any) {
	runByQ := map[string]RunEntry{}
	for _, run := range runs {
		runByQ[run.QID] = run
	}
	relMap := buildRelevantMap(qrels)

	recall5 := make([]float64, 0, len(queries))
	recall10 := make([]float64, 0, len(queries))
	recall20 := make([]float64, 0, len(queries))
	hit5 := make([]float64, 0, len(queries))
	hit10 := make([]float64, 0, len(queries))
	hit20 := make([]float64, 0, len(queries))
	mrr10 := make([]float64, 0, len(queries))
	ndcg10 := make([]float64, 0, len(queries))
	per := make([]map[string]any, 0, len(queries))

	for _, q := range queries {
		run := runByQ[q.QID]
		rels := relMap[q.QID]
		r5, h5 := recallAndHitAtK(run.Results, rels, 5)
		r10, h10 := recallAndHitAtK(run.Results, rels, 10)
		r20, h20 := recallAndHitAtK(run.Results, rels, 20)
		recall5 = append(recall5, r5)
		recall10 = append(recall10, r10)
		recall20 = append(recall20, r20)
		hit5 = append(hit5, h5)
		hit10 = append(hit10, h10)
		hit20 = append(hit20, h20)
		mrr := mrrAtK(run.Results, rels, 10)
		ndcg := ndcgAtK(run.Results, rels, 10)
		mrr10 = append(mrr10, mrr)
		ndcg10 = append(ndcg10, ndcg)
		per = append(per, map[string]any{
			"qid":       q.QID,
			"recall@5":  r5,
			"recall@10": r10,
			"recall@20": r20,
			"hit@5":     h5,
			"hit@10":    h10,
			"hit@20":    h20,
			"mrr@10":    mrr,
			"ndcg@10":   ndcg,
		})
	}

	metrics := map[string]any{
		"queries":   len(queries),
		"runs":      len(runs),
		"top_k":     topK,
		"Recall@5":  mean(recall5),
		"Recall@10": mean(recall10),
		"Recall@20": mean(recall20),
		"Hit@5":     mean(hit5),
		"Hit@10":    mean(hit10),
		"Hit@20":    mean(hit20),
		"MRR@10":    mean(mrr10),
		"nDCG@10":   mean(ndcg10),
	}
	return metrics, per
}

func buildRelevantMap(qrels []QRel) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, rel := range qrels {
		if rel.Relevance <= 0 {
			continue
		}
		bucket := out[rel.QID]
		if bucket == nil {
			bucket = map[string]int{}
			out[rel.QID] = bucket
		}
		bucket[rel.DocID] = rel.Relevance
	}
	return out
}

func recallAndHitAtK(results []RunHit, relevant map[string]int, k int) (float64, float64) {
	if len(relevant) == 0 {
		return 0, 0
	}
	if k > len(results) {
		k = len(results)
	}
	hitCount := 0
	for i := 0; i < k; i++ {
		if relevant[results[i].DocID] > 0 {
			hitCount++
		}
	}
	hit := 0.0
	if hitCount > 0 {
		hit = 1.0
	}
	return float64(hitCount) / float64(len(relevant)), hit
}

func mrrAtK(results []RunHit, relevant map[string]int, k int) float64 {
	if k > len(results) {
		k = len(results)
	}
	for i := 0; i < k; i++ {
		if relevant[results[i].DocID] > 0 {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

func ndcgAtK(results []RunHit, relevant map[string]int, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	if k > len(results) {
		k = len(results)
	}
	dcg := 0.0
	for i := 0; i < k; i++ {
		rel := relevant[results[i].DocID]
		if rel <= 0 {
			continue
		}
		dcg += (math.Pow(2, float64(rel)) - 1) / math.Log2(float64(i+2))
	}
	ideal := make([]int, 0, len(relevant))
	for _, rel := range relevant {
		ideal = append(ideal, rel)
	}
	sort.Slice(ideal, func(i, j int) bool { return ideal[i] > ideal[j] })
	if k > len(ideal) {
		k = len(ideal)
	}
	idcg := 0.0
	for i := 0; i < k; i++ {
		idcg += (math.Pow(2, float64(ideal[i])) - 1) / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func cosineSimilarity(a, b []float32) (float64, bool) {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0, false
	}
	dot := 0.0
	na := 0.0
	nb := 0.0
	for i := range a {
		x := float64(a[i])
		y := float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), true
}

func evaluateRetrievalWarnings(metrics map[string]any) []string {
	warnings := make([]string, 0)
	recall := metricFloat(metrics, "Recall@20")
	ndcg := metricFloat(metrics, "nDCG@10")
	stageMetrics, _ := metrics["stages"].(map[string]any)
	denseRecall := metricFloat(mapOfAny(stageMetrics, "dense"), "Recall@20")
	fusionRecall := metricFloat(mapOfAny(stageMetrics, "fusion"), "Recall@20")
	fusionNDCG := metricFloat(mapOfAny(stageMetrics, "fusion"), "nDCG@10")
	rerankNDCG := metricFloat(mapOfAny(stageMetrics, "rerank"), "nDCG@10")
	if recall < 0.4 {
		warnings = append(warnings, fmt.Sprintf("Recall@20 %.4f below baseline 0.40", recall))
	}
	if ndcg < 0.3 {
		warnings = append(warnings, fmt.Sprintf("nDCG@10 %.4f below baseline 0.30", ndcg))
	}
	if fusionRecall < denseRecall {
		warnings = append(warnings, fmt.Sprintf("fusion Recall@20 %.4f below dense %.4f", fusionRecall, denseRecall))
	}
	if rerankNDCG < fusionNDCG {
		warnings = append(warnings, fmt.Sprintf("rerank nDCG@10 %.4f below fusion %.4f", rerankNDCG, fusionNDCG))
	}
	return warnings
}

func orderedRetrievalStages(runs map[string][]RunEntry) []string {
	order := []string{"dense", "sparse", "fusion", "rerank", "provided"}
	out := make([]string, 0, len(runs))
	for _, stage := range order {
		if _, ok := runs[stage]; ok {
			out = append(out, stage)
		}
	}
	for stage := range runs {
		if !slicesContains(out, stage) {
			out = append(out, stage)
		}
	}
	return out
}

func finalRetrievalStage(opts RetrievalOptions) string {
	mode := strings.TrimSpace(strings.ToLower(opts.RetrievalMode))
	switch mode {
	case "dense_only":
		return "dense"
	case "sparse_only":
		return "sparse"
	case "provided":
		return "provided"
	default:
		return "rerank"
	}
}

func topKForStage(opts RetrievalOptions, stage string) int {
	switch stage {
	case "dense":
		return topKOrDefault(opts.DenseTopK, opts.TopK, 20)
	case "sparse":
		return topKOrDefault(opts.SparseTopK, opts.TopK, 20)
	case "fusion":
		return topKOrDefault(opts.FusionTopK, opts.TopK, 20)
	case "rerank":
		return topKOrDefault(opts.RerankTopN, opts.TopK, 10)
	default:
		return topKOrDefault(opts.TopK, 20, 20)
	}
}

func topKOrDefault(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 20
}

func mapOfAny(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	child, _ := parent[key].(map[string]any)
	return child
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
