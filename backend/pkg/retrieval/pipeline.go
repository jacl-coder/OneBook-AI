package retrieval

import (
	"context"
	"math"
	"sort"
	"strings"

	"onebookai/pkg/domain"
)

type StageHit struct {
	ChunkID  string
	BookID   string
	Score    float64
	Stage    string
	Content  string
	Metadata map[string]string
	Chunk    domain.Chunk
}

type RewriteFunc func(ctx context.Context, query, language string) ([]string, error)
type SearchFunc func(ctx context.Context, query, language string, topK int) ([]StageHit, error)
type ChunkLoadFunc func(ctx context.Context, ids []string) (map[string]domain.Chunk, error)

type PipelineOptions struct {
	Query         string
	BookID        string
	RetrievalMode string
	TopK          int
	DenseTopK     int
	LexicalTopK   int
	FusionTopK    int
	RerankTopN    int
	ContextBudget int
}

type PipelineResult struct {
	Language string
	Queries  []string
	Dense    []StageHit
	Lexical  []StageHit
	Fused    []StageHit
	Reranked []StageHit
	Final    []StageHit
	Warnings []string
}

type Pipeline struct {
	Rewrite     RewriteFunc
	Dense       SearchFunc
	Lexical     SearchFunc
	ChunkLoader ChunkLoadFunc
	Reranker    Reranker
}

func (p Pipeline) Run(ctx context.Context, opts PipelineOptions) (PipelineResult, error) {
	query := NormalizeText(opts.Query)
	language := DetectLanguage(query)
	queries := BuildQueryVariants(query)
	if p.Rewrite != nil {
		if rewritten, err := p.Rewrite(ctx, query, language); err == nil && len(rewritten) > 0 {
			queries = rewritten
		}
	}
	if len(queries) == 0 {
		queries = []string{query}
	}

	denseHits := make(map[string]StageHit)
	lexicalHits := make(map[string]StageHit)
	warnings := make([]string, 0, 2)

	for _, item := range queries {
		if strings.TrimSpace(item) == "" {
			continue
		}
		if opts.RetrievalMode != "lexical_only" && p.Dense != nil {
			hits, err := p.Dense(ctx, item, language, opts.DenseTopK)
			if err != nil {
				warnings = append(warnings, "dense retrieval unavailable")
			} else {
				accumulateStageHits(denseHits, hits, "dense")
			}
		}
		if opts.RetrievalMode != "dense_only" && p.Lexical != nil {
			hits, err := p.Lexical(ctx, item, language, opts.LexicalTopK)
			if err != nil {
				warnings = append(warnings, "lexical retrieval unavailable")
			} else {
				accumulateStageHits(lexicalHits, hits, "lexical")
			}
		}
	}

	result := PipelineResult{
		Language: language,
		Queries:  queries,
		Warnings: uniquePipelineStrings(warnings),
	}
	if len(denseHits) == 0 && len(lexicalHits) == 0 {
		return result, nil
	}

	result.Dense = orderedStageHits(denseHits, opts.DenseTopK)
	result.Lexical = orderedStageHits(lexicalHits, opts.LexicalTopK)
	result.Fused = fuseStageHits(denseHits, lexicalHits, opts.FusionTopK)

	if p.ChunkLoader != nil {
		loaded, err := p.ChunkLoader(ctx, uniqueChunkIDs(result.Dense, result.Lexical, result.Fused))
		if err != nil {
			result.Warnings = uniquePipelineStrings(append(result.Warnings, "chunk hydration unavailable"))
		} else {
			result.Dense = hydrateStageHits(result.Dense, loaded)
			result.Lexical = hydrateStageHits(result.Lexical, loaded)
			result.Fused = hydrateStageHits(result.Fused, loaded)
		}
	}

	switch strings.TrimSpace(opts.RetrievalMode) {
	case "dense_only":
		result.Final = result.Dense
	case "lexical_only":
		result.Final = result.Lexical
	case "hybrid_best":
		result.Reranked = result.Fused
		if p.Reranker != nil && len(result.Fused) > 0 {
			reranked, err := p.Reranker.Rerank(ctx, query, result.Fused, opts.RerankTopN)
			if err != nil {
				result.Warnings = uniquePipelineStrings(append(result.Warnings, "reranker unavailable"))
			} else if len(reranked) > 0 {
				result.Reranked = reranked
			}
		}
		result.Final = result.Reranked
	default:
		result.Final = result.Fused
	}
	if len(result.Reranked) == 0 && strings.TrimSpace(opts.RetrievalMode) != "hybrid_best" {
		result.Reranked = result.Final
	}
	result.Final = packStageHits(result.Final, opts.TopK, opts.ContextBudget)
	return result, nil
}

func accumulateStageHits(target map[string]StageHit, hits []StageHit, stage string) {
	for rank, hit := range hits {
		hit.Stage = stage
		hit.Score += reciprocalRank(rank)
		mergeStageHit(target, hit)
	}
}

func mergeStageHit(target map[string]StageHit, hit StageHit) {
	key := stageHitKey(hit)
	existing, ok := target[key]
	if !ok || hit.Score > existing.Score {
		target[key] = hit
	}
}

func stageHitKey(hit StageHit) string {
	if hit.Metadata != nil {
		if family := strings.TrimSpace(hit.Metadata["chunk_family"]); family != "" {
			return family
		}
	}
	if chunkID := strings.TrimSpace(hit.ChunkID); chunkID != "" {
		return chunkID
	}
	return strings.TrimSpace(hit.Chunk.ID)
}

func orderedStageHits(items map[string]StageHit, topK int) []StageHit {
	out := make([]StageHit, 0, len(items))
	for _, hit := range items {
		out = append(out, hit)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if math.Abs(out[i].Score-out[j].Score) < 1e-9 {
			return out[i].ChunkID < out[j].ChunkID
		}
		return out[i].Score > out[j].Score
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

func fuseStageHits(denseHits, lexicalHits map[string]StageHit, topK int) []StageHit {
	fused := make(map[string]StageHit, len(denseHits)+len(lexicalHits))
	for id, hit := range denseHits {
		copy := hit
		copy.Stage = "fusion"
		fused[id] = copy
	}
	for id, hit := range lexicalHits {
		current, ok := fused[id]
		if !ok {
			hit.Stage = "fusion"
			fused[id] = hit
			continue
		}
		if current.ChunkID == "" {
			current.ChunkID = hit.ChunkID
		}
		if current.BookID == "" {
			current.BookID = hit.BookID
		}
		if strings.TrimSpace(current.Content) == "" {
			current.Content = hit.Content
		}
		current.Metadata = mergeMetadata(current.Metadata, hit.Metadata)
		current.Score += hit.Score
		fused[id] = current
	}
	return orderedStageHits(fused, topK)
}

func mergeMetadata(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		if strings.TrimSpace(out[key]) == "" {
			out[key] = value
		}
	}
	return out
}

func hydrateStageHits(hits []StageHit, chunks map[string]domain.Chunk) []StageHit {
	if len(hits) == 0 || len(chunks) == 0 {
		return hits
	}
	out := make([]StageHit, 0, len(hits))
	for _, hit := range hits {
		chunk, ok := chunks[hit.ChunkID]
		if !ok {
			out = append(out, hit)
			continue
		}
		meta := mergeMetadata(chunk.Metadata, hit.Metadata)
		chunk.Metadata = meta
		hit.Chunk = chunk
		hit.BookID = chunk.BookID
		hit.Content = chunk.Content
		hit.Metadata = meta
		out = append(out, hit)
	}
	return out
}

func uniqueChunkIDs(groups ...[]StageHit) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 32)
	for _, group := range groups {
		for _, hit := range group {
			id := strings.TrimSpace(hit.ChunkID)
			if id == "" {
				id = strings.TrimSpace(hit.Chunk.ID)
			}
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func packStageHits(hits []StageHit, topK int, budget int) []StageHit {
	if topK <= 0 {
		topK = 4
	}
	if budget <= 0 {
		budget = 2200
	}
	out := make([]StageHit, 0, minInt(topK, len(hits)))
	seenHashes := map[string]struct{}{}
	usedBudget := 0
	for _, hit := range hits {
		hash := ""
		if hit.Metadata != nil {
			hash = strings.TrimSpace(hit.Metadata["content_sha256"])
		}
		if hash != "" {
			if _, ok := seenHashes[hash]; ok {
				continue
			}
			seenHashes[hash] = struct{}{}
		}
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			content = strings.TrimSpace(hit.Chunk.Content)
		}
		if content == "" {
			continue
		}
		length := len([]rune(content))
		if length <= 0 {
			length = 1
		}
		if usedBudget+length > budget && len(out) > 0 {
			continue
		}
		usedBudget += length
		out = append(out, hit)
		if len(out) >= topK {
			break
		}
	}
	return out
}

func reciprocalRank(rank int) float64 {
	return 1.0 / float64(rank+61)
}

func uniquePipelineStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
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

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
