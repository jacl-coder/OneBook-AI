package eval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

func EvaluateRetrieval(opts RetrievalOptions) (EvalResult, []RunEntry, error) {
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

	runs, warnings, err := loadOrBuildRunForRetrieval(opts, queries)
	if err != nil {
		return EvalResult{}, nil, err
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}

	metrics, per := computeIRMetrics(queries, qrels, runs, topK)
	warnings = append(warnings, evaluateRetrievalWarnings(metrics)...)
	return EvalResult{Metrics: metrics, PerQuery: per, Warnings: uniqueStrings(warnings)}, runs, nil
}

func loadOrBuildRunForRetrieval(opts RetrievalOptions, queries []QueryRecord) ([]RunEntry, []string, error) {
	warnings := make([]string, 0)
	if strings.TrimSpace(opts.RunPath) != "" {
		runs, err := ReadRunJSONL(opts.RunPath)
		return runs, warnings, err
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

	index := make([]EmbeddingRecord, 0, len(chunks))
	for _, c := range chunks {
		vec, ok := embeddings[c.ChunkID]
		if !ok || len(vec) == 0 {
			continue
		}
		index = append(index, EmbeddingRecord{ID: c.ChunkID, Vector: vec})
	}
	if len(index) == 0 {
		return nil, warnings, fmt.Errorf("online retrieval index is empty")
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}
	out := make([]RunEntry, 0, len(queries))
	for _, q := range queries {
		if strings.TrimSpace(q.Query) == "" {
			out = append(out, RunEntry{QID: q.QID})
			continue
		}
		qvec, err := embedder.EmbedText(context.Background(), q.Query, "RETRIEVAL_QUERY")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("query embedding failed for %s: %v", q.QID, err))
			out = append(out, RunEntry{QID: q.QID})
			continue
		}
		hits := rankByCosine(qvec, index, topK)
		out = append(out, RunEntry{QID: q.QID, Results: hits})
	}
	return out, warnings, nil
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
	if recall < 0.4 {
		warnings = append(warnings, fmt.Sprintf("Recall@20 %.4f below baseline 0.40", recall))
	}
	if ndcg < 0.3 {
		warnings = append(warnings, fmt.Sprintf("nDCG@10 %.4f below baseline 0.30", ndcg))
	}
	return warnings
}
