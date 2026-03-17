package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
)

const defaultAbstainAnswer = "证据不足，当前无法基于已上传内容给出可靠回答。"

type QueryRewriter interface {
	Rewrite(ctx context.Context, query, language string) ([]string, error)
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
	if r.generator == nil {
		return nil, nil
	}
	prompt := fmt.Sprintf("Language: %s\nQuestion: %s\nReturn a JSON array with up to 3 short search rewrites.", language, query)
	out, err := r.generator.GenerateText(ctx, "You rewrite search queries. Output valid JSON only.", prompt)
	if err != nil {
		return nil, err
	}
	var rewrites []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rewrites); err != nil {
		return nil, err
	}
	return rewrites, nil
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
	return validationTokenOverlap(answerTokens, citationTokens) >= 0.08
}

func validationTokenOverlap(left, right []string) float64 {
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
	if len(seen) == 0 {
		return 0
	}
	return float64(matches) / float64(len(seen))
}

func (a *App) buildRetrievalQueries(ctx context.Context, question string) []string {
	normalized := retrieval.NormalizeText(question)
	if normalized == "" {
		return nil
	}
	queries := []string{normalized}
	if a.multiQueryEnabled {
		queries = retrieval.BuildQueryVariants(normalized)
	}
	if a.queryRewriteEnabled && a.rewriter != nil {
		language := retrieval.DetectLanguage(normalized)
		rewrites, err := a.rewriter.Rewrite(ctx, normalized, language)
		if err == nil {
			rewrites = normalizeRetrievalQueries(rewrites)
			if len(rewrites) > 0 {
				if a.multiQueryEnabled {
					queries = uniqueRetrievalQueries(append(queries, rewrites...))
				} else {
					queries = []string{rewrites[0]}
				}
			}
		}
	}
	return uniqueRetrievalQueries(queries)
}

func (a *App) retrieveEvidence(ctx context.Context, book domain.Book, question string) ([]retrieval.StageHit, *domain.RetrievalDebug, error) {
	queries := a.buildRetrievalQueries(ctx, question)
	pipeline := retrieval.Pipeline{
		Dense: func(ctx context.Context, query, _ string, topK int) ([]retrieval.StageHit, error) {
			vector, err := a.embedder.EmbedText(ctx, query, "RETRIEVAL_QUERY")
			if err != nil {
				return nil, err
			}
			points, err := a.search.QueryDense(ctx, book.ID, vector, topK)
			if err != nil {
				return nil, err
			}
			return pointsToStageHits(points, "dense"), nil
		},
		Lexical: func(ctx context.Context, query, language string, topK int) ([]retrieval.StageHit, error) {
			terms := strings.Join(retrieval.Tokenize(query, language), " ")
			points, err := a.lexical.QueryBM25(ctx, book.ID, terms, topK)
			if err != nil {
				return nil, err
			}
			return pointsToStageHits(points, "lexical"), nil
		},
		ChunkLoader: func(ctx context.Context, ids []string) (map[string]domain.Chunk, error) {
			chunks, err := a.store.GetChunksByIDs(ids)
			if err != nil {
				return nil, err
			}
			index := make(map[string]domain.Chunk, len(chunks))
			for _, chunk := range chunks {
				index[chunk.ID] = chunk
			}
			return index, nil
		},
		Reranker: a.reranker,
	}
	result, err := pipeline.Run(ctx, retrieval.PipelineOptions{
		Query:         question,
		Queries:       queries,
		BookID:        book.ID,
		RetrievalMode: a.retrievalMode,
		TopK:          a.topK,
		DenseTopK:     a.denseRecallTopK,
		LexicalTopK:   a.lexicalRecallTopK,
		DenseWeight:   a.denseWeight,
		LexicalWeight: a.lexicalWeight,
		FusionTopK:    a.fusionTopK,
		RerankTopN:    a.rerankTopN,
		ContextBudget: a.contextBudget,
	})
	if err != nil {
		return nil, nil, err
	}
	debugInfo := &domain.RetrievalDebug{
		Language: result.Language,
		Queries:  result.Queries,
		Dense:    stageHitsToDebug(result.Dense, "dense", a.rerankTopN),
		Lexical:  stageHitsToDebug(result.Lexical, "lexical", a.rerankTopN),
		Fused:    stageHitsToDebug(result.Fused, "fusion", a.rerankTopN),
		Reranked: stageHitsToDebug(result.Reranked, "rerank", a.topK),
	}
	return result.Final, debugInfo, nil
}

func pointsToStageHits(points []retrieval.Point, stage string) []retrieval.StageHit {
	out := make([]retrieval.StageHit, 0, len(points))
	for _, point := range points {
		meta := pointMetadata(point.Payload)
		out = append(out, retrieval.StageHit{
			ChunkID:  strings.TrimSpace(point.ID),
			BookID:   strings.TrimSpace(anyString(point.Payload["book_id"])),
			Score:    point.Score,
			Stage:    stage,
			Content:  strings.TrimSpace(anyString(point.Payload["content_text"])),
			Metadata: meta,
		})
	}
	return out
}

func pointMetadata(payload map[string]any) map[string]string {
	return map[string]string{
		"source_type":    strings.TrimSpace(anyString(payload["source_type"])),
		"source_ref":     strings.TrimSpace(anyString(payload["source_ref"])),
		"page":           strings.TrimSpace(anyString(payload["page"])),
		"section_path":   strings.TrimSpace(anyString(payload["section_path"])),
		"section_id":     strings.TrimSpace(anyString(payload["section_id"])),
		"chunk_family":   strings.TrimSpace(anyString(payload["chunk_family"])),
		"retrieval_tier": strings.TrimSpace(anyString(payload["retrieval_tier"])),
		"content_sha256": strings.TrimSpace(anyString(payload["content_sha256"])),
		"language":       strings.TrimSpace(anyString(payload["language"])),
		"title":          strings.TrimSpace(anyString(payload["title"])),
		"section_title":  strings.TrimSpace(anyString(payload["section_title"])),
		"block_type":     strings.TrimSpace(anyString(payload["block_type"])),
	}
}

func stageHitsToDebug(hits []retrieval.StageHit, stage string, limit int) []domain.RetrievalHit {
	if limit <= 0 || len(hits) < limit {
		limit = len(hits)
	}
	out := make([]domain.RetrievalHit, 0, limit)
	for i := 0; i < limit; i++ {
		hit := hits[i]
		sourceRef := ""
		if hit.Metadata != nil {
			sourceRef = strings.TrimSpace(hit.Metadata["source_ref"])
		}
		snippet := strings.TrimSpace(hit.Content)
		if snippet == "" {
			snippet = strings.TrimSpace(hit.Chunk.Content)
		}
		out = append(out, domain.RetrievalHit{
			ChunkID:   hit.ChunkID,
			SourceRef: sourceRef,
			Score:     hit.Score,
			Stage:     stage,
			Snippet:   truncateRunes(snippet, 180),
		})
	}
	return out
}

func truncateRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if limit <= 0 || len(runes) <= limit {
		return strings.TrimSpace(text)
	}
	return string(runes[:limit]) + "…"
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

func normalizeRetrievalQueries(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = retrieval.NormalizeText(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return uniqueRetrievalQueries(out)
}

func uniqueRetrievalQueries(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
