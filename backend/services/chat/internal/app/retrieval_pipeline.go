package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
)

const defaultAbstainAnswer = "证据不足，当前无法基于已上传内容给出可靠回答。"

type retrievalHit struct {
	chunk   domain.Chunk
	score   float64
	stage   string
	content string
}

type QueryRewriter interface {
	Rewrite(ctx context.Context, query, language string) ([]string, error)
}

type Reranker interface {
	Rerank(ctx context.Context, query string, hits []retrievalHit, limit int) ([]retrievalHit, error)
}

type GroundingValidator interface {
	Validate(question, answer string, citations []domain.Source) bool
}

type modelQueryRewriter struct {
	generator ai.TextGenerator
}

func newModelQueryRewriter(generator ai.TextGenerator) QueryRewriter {
	return &modelQueryRewriter{generator: generator}
}

func (r *modelQueryRewriter) Rewrite(ctx context.Context, query, language string) ([]string, error) {
	base := retrieval.BuildQueryVariants(query)
	if r.generator == nil {
		return base, nil
	}
	prompt := fmt.Sprintf("Language: %s\nQuestion: %s\nReturn a JSON array with up to 3 short search rewrites.", language, query)
	out, err := r.generator.GenerateText(ctx, "You rewrite search queries. Output valid JSON only.", prompt)
	if err != nil {
		return base, err
	}
	var rewrites []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rewrites); err != nil {
		return base, err
	}
	return retrieval.BuildQueryVariants(strings.Join(append(base, rewrites...), "\n")), nil
}

type hybridReranker struct {
	url      string
	fallback Reranker
	client   *http.Client
}

func newHybridReranker(url string) Reranker {
	return hybridReranker{
		url:      strings.TrimSpace(url),
		fallback: fallbackReranker{},
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (r hybridReranker) Rerank(ctx context.Context, query string, hits []retrievalHit, limit int) ([]retrievalHit, error) {
	if strings.TrimSpace(r.url) == "" {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	reqBody := struct {
		Query     string `json:"query"`
		TopN      int    `json:"top_n"`
		Documents []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"documents"`
	}{
		Query: query,
		TopN:  limit,
	}
	reqBody.Documents = make([]struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}, 0, len(hits))
	index := make(map[string]retrievalHit, len(hits))
	for _, hit := range hits {
		id := fuseKey(hit)
		index[id] = hit
		reqBody.Documents = append(reqBody.Documents, struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		}{ID: id, Text: hit.content})
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(data))
	if err != nil {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	var out struct {
		Results []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	scored := make([]retrievalHit, 0, len(out.Results))
	for _, result := range out.Results {
		hit, ok := index[result.ID]
		if !ok {
			continue
		}
		hit.score = result.Score
		hit.stage = "rerank"
		scored = append(scored, hit)
	}
	if len(scored) == 0 {
		return r.fallback.Rerank(ctx, query, hits, limit)
	}
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

type fallbackReranker struct{}

func (fallbackReranker) Rerank(_ context.Context, query string, hits []retrievalHit, limit int) ([]retrievalHit, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	queryLanguage := retrieval.DetectLanguage(query)
	queryTokens := retrieval.Tokenize(query, queryLanguage)
	scored := make([]retrievalHit, 0, len(hits))
	for _, hit := range hits {
		contentTokens := retrieval.Tokenize(hit.content, queryLanguage)
		overlap := tokenOverlap(queryTokens, contentTokens)
		hit.score = hit.score + overlap
		scored = append(scored, hit)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].chunk.ID < scored[j].chunk.ID
		}
		return scored[i].score > scored[j].score
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	for i := range scored {
		scored[i].stage = "rerank"
	}
	return scored, nil
}

type groundingValidator struct {
	generator ai.TextGenerator
}

func newGroundingValidator(generator ai.TextGenerator) GroundingValidator {
	return &groundingValidator{generator: generator}
}

func (v *groundingValidator) Validate(question, answer string, citations []domain.Source) bool {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return false
	}
	if strings.Contains(answer, "证据不足") || strings.Contains(strings.ToLower(answer), "insufficient") {
		return true
	}
	if len(citations) == 0 {
		return false
	}
	queryLanguage := retrieval.DetectLanguage(question + "\n" + answer)
	answerTokens := retrieval.Tokenize(answer, queryLanguage)
	citationTokens := make([]string, 0, len(citations)*8)
	for _, citation := range citations {
		citationTokens = append(citationTokens, retrieval.Tokenize(citation.Snippet, queryLanguage)...)
	}
	return tokenOverlap(answerTokens, citationTokens) >= 0.08
}

func tokenOverlap(left, right []string) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	rightSet := make(map[string]struct{}, len(right))
	for _, token := range right {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		rightSet[token] = struct{}{}
	}
	if len(rightSet) == 0 {
		return 0
	}
	matches := 0
	seen := map[string]struct{}{}
	for _, token := range left {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		if _, ok := rightSet[token]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(seen))
}

func (a *App) retrieveEvidence(ctx context.Context, book domain.Book, question string) ([]retrievalHit, *domain.RetrievalDebug, error) {
	query := retrieval.NormalizeText(question)
	language := retrieval.DetectLanguage(query)
	queries, err := a.rewriter.Rewrite(ctx, query, language)
	if err != nil || len(queries) == 0 {
		queries = retrieval.BuildQueryVariants(query)
	}
	if len(queries) == 0 {
		queries = []string{query}
	}
	denseDebug := make([]domain.RetrievalHit, 0)
	lexicalDebug := make([]domain.RetrievalHit, 0)
	denseHits := make(map[string]retrievalHit)
	lexicalHits := make(map[string]retrievalHit)
	for _, item := range queries {
		if a.retrievalMode != "lexical_only" {
			vector, err := a.embedder.EmbedText(ctx, item, "RETRIEVAL_QUERY")
			if err == nil {
				points, err := a.search.QueryDense(ctx, book.ID, vector, a.denseRecallTopK)
				if err == nil {
					for rank, point := range points {
						hit := pointToRetrievalHit(point, "dense")
						hit.score += reciprocalRank(rank)
						mergeRetrievalHit(denseHits, hit)
						denseDebug = append(denseDebug, debugHit(hit, "dense"))
					}
				}
			}
		}
		if a.retrievalMode == "dense_only" {
			continue
		}
		terms := strings.Join(retrieval.Tokenize(item, language), " ")
		points, err := a.lexical.QueryBM25(ctx, book.ID, terms, a.lexicalRecallTopK)
		if err == nil {
			for rank, point := range points {
				hit := pointToRetrievalHit(point, "lexical")
				hit.score += reciprocalRank(rank)
				mergeRetrievalHit(lexicalHits, hit)
				lexicalDebug = append(lexicalDebug, debugHit(hit, "lexical"))
			}
		}
	}
	if len(denseHits) == 0 && len(lexicalHits) == 0 {
		return nil, &domain.RetrievalDebug{Language: language, Queries: queries}, nil
	}
	fused := fuseHits(denseHits, lexicalHits, a.fusionTopK)
	finalHits := fused
	if a.retrievalMode == "dense_only" {
		finalHits = orderedHits(denseHits, a.fusionTopK)
	} else if a.retrievalMode == "lexical_only" {
		finalHits = orderedHits(lexicalHits, a.fusionTopK)
	} else if a.retrievalMode == "hybrid_best" {
		reranked, err := a.reranker.Rerank(ctx, query, fused, a.rerankTopN)
		if err == nil && len(reranked) > 0 {
			finalHits = reranked
		}
	}
	packed := packContext(finalHits, a.topK, a.contextBudget)
	debugInfo := &domain.RetrievalDebug{
		Language: language,
		Queries:  queries,
		Dense:    limitDebugHits(denseDebug, a.rerankTopN),
		Lexical:  limitDebugHits(lexicalDebug, a.rerankTopN),
		Fused:    hitsToDebug(fused, "fusion", a.rerankTopN),
		Reranked: hitsToDebug(finalHits, "rerank", a.topK),
	}
	return packed, debugInfo, nil
}

func pointToRetrievalHit(point retrieval.Point, stage string) retrievalHit {
	payload := point.Payload
	meta := map[string]string{
		"source_type":    strings.TrimSpace(anyString(payload["source_type"])),
		"source_ref":     strings.TrimSpace(anyString(payload["source_ref"])),
		"page":           strings.TrimSpace(anyString(payload["page"])),
		"section_path":   strings.TrimSpace(anyString(payload["section_path"])),
		"chunk_index":    strings.TrimSpace(anyString(payload["chunk_index"])),
		"chunk_family":   strings.TrimSpace(anyString(payload["chunk_family"])),
		"retrieval_tier": strings.TrimSpace(anyString(payload["retrieval_tier"])),
		"content_sha256": strings.TrimSpace(anyString(payload["content_sha256"])),
		"language":       strings.TrimSpace(anyString(payload["language"])),
	}
	return retrievalHit{
		chunk: domain.Chunk{
			ID:       point.ID,
			BookID:   strings.TrimSpace(anyString(payload["book_id"])),
			Content:  strings.TrimSpace(point.Content),
			Metadata: meta,
		},
		score:   point.Score,
		stage:   stage,
		content: strings.TrimSpace(point.Content),
	}
}

func mergeRetrievalHit(target map[string]retrievalHit, hit retrievalHit) {
	key := fuseKey(hit)
	existing, ok := target[key]
	if !ok || hit.score > existing.score {
		target[key] = hit
	}
}

func fuseHits(denseHits, lexicalHits map[string]retrievalHit, topK int) []retrievalHit {
	fused := make(map[string]retrievalHit, len(denseHits)+len(lexicalHits))
	for id, hit := range denseHits {
		copy := hit
		copy.stage = "fusion"
		fused[id] = copy
	}
	for id, hit := range lexicalHits {
		current, ok := fused[id]
		if !ok {
			hit.stage = "fusion"
			fused[id] = hit
			continue
		}
		current.score += hit.score
		fused[id] = current
	}
	out := make([]retrievalHit, 0, len(fused))
	for _, hit := range fused {
		out = append(out, hit)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if math.Abs(out[i].score-out[j].score) < 1e-9 {
			return out[i].chunk.ID < out[j].chunk.ID
		}
		return out[i].score > out[j].score
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

func orderedHits(items map[string]retrievalHit, topK int) []retrievalHit {
	out := make([]retrievalHit, 0, len(items))
	for _, hit := range items {
		out = append(out, hit)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].chunk.ID < out[j].chunk.ID
		}
		return out[i].score > out[j].score
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

func fuseKey(hit retrievalHit) string {
	if family := strings.TrimSpace(hit.chunk.Metadata["chunk_family"]); family != "" {
		return family
	}
	return hit.chunk.ID
}

func packContext(hits []retrievalHit, topK int, budget int) []retrievalHit {
	if topK <= 0 {
		topK = 4
	}
	if budget <= 0 {
		budget = 2200
	}
	out := make([]retrievalHit, 0, minInt(topK, len(hits)))
	seenHashes := map[string]struct{}{}
	usedBudget := 0
	for _, hit := range hits {
		hash := strings.TrimSpace(hit.chunk.Metadata["content_sha256"])
		if hash != "" {
			if _, ok := seenHashes[hash]; ok {
				continue
			}
			seenHashes[hash] = struct{}{}
		}
		contentLen := len([]rune(hit.content))
		if contentLen <= 0 {
			contentLen = 1
		}
		if usedBudget+contentLen > budget && len(out) > 0 {
			continue
		}
		usedBudget += contentLen
		out = append(out, hit)
		if len(out) >= topK {
			break
		}
	}
	return out
}

func hitsToDebug(hits []retrievalHit, stage string, limit int) []domain.RetrievalHit {
	if limit <= 0 || len(hits) < limit {
		limit = len(hits)
	}
	out := make([]domain.RetrievalHit, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, debugHit(hits[i], stage))
	}
	return out
}

func limitDebugHits(hits []domain.RetrievalHit, limit int) []domain.RetrievalHit {
	if limit <= 0 || len(hits) <= limit {
		return hits
	}
	return hits[:limit]
}

func debugHit(hit retrievalHit, stage string) domain.RetrievalHit {
	return domain.RetrievalHit{
		ChunkID:   hit.chunk.ID,
		SourceRef: strings.TrimSpace(hit.chunk.Metadata["source_ref"]),
		Score:     hit.score,
		Stage:     stage,
		Snippet:   truncateRunes(hit.content, 180),
	}
}

func reciprocalRank(rank int) float64 {
	return 1.0 / float64(rank+61)
}

func truncateRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if limit <= 0 || len(runes) <= limit {
		return strings.TrimSpace(text)
	}
	return string(runes[:limit]) + "…"
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func anyString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return ""
	}
}
