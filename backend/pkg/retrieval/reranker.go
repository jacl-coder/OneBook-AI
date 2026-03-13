package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Reranker interface {
	Rerank(ctx context.Context, query string, hits []StageHit, limit int) ([]StageHit, error)
}

type ServiceReranker struct {
	url      string
	maxDocs  int
	maxChars int
	client   *http.Client
}

func NewServiceReranker(url string, timeout time.Duration, maxDocs, maxChars int) *ServiceReranker {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if maxDocs <= 0 {
		maxDocs = 50
	}
	if maxChars <= 0 {
		maxChars = 2400
	}
	return &ServiceReranker{
		url:      strings.TrimSpace(url),
		maxDocs:  maxDocs,
		maxChars: maxChars,
		client:   &http.Client{Timeout: timeout},
	}
}

func (r *ServiceReranker) Rerank(ctx context.Context, query string, hits []StageHit, limit int) ([]StageHit, error) {
	if r == nil || strings.TrimSpace(r.url) == "" {
		return nil, fmt.Errorf("reranker url required")
	}
	if len(hits) == 0 {
		return nil, nil
	}
	if len(hits) > r.maxDocs {
		return nil, fmt.Errorf("reranker max docs exceeded: %d > %d", len(hits), r.maxDocs)
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
	index := make(map[string]StageHit, len(hits))
	reqBody.Documents = make([]struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}, 0, len(hits))
	for _, hit := range hits {
		text := strings.TrimSpace(hit.Content)
		if text == "" {
			text = strings.TrimSpace(hit.Chunk.Content)
		}
		if text == "" {
			continue
		}
		runes := []rune(text)
		if len(runes) > r.maxChars {
			text = string(runes[:r.maxChars])
		}
		id := stageHitKey(hit)
		index[id] = hit
		reqBody.Documents = append(reqBody.Documents, struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		}{ID: id, Text: text})
	}
	if len(reqBody.Documents) == 0 {
		return nil, fmt.Errorf("no rerankable documents")
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("reranker error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Results []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	scored := make([]StageHit, 0, len(out.Results))
	for _, result := range out.Results {
		hit, ok := index[result.ID]
		if !ok {
			continue
		}
		hit.Score = result.Score
		hit.Stage = "rerank"
		scored = append(scored, hit)
	}
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

func (r *ServiceReranker) Health(ctx context.Context) error {
	if r == nil || strings.TrimSpace(r.url) == "" {
		return fmt.Errorf("reranker url required")
	}
	healthURL := strings.TrimSuffix(r.url, "/rerank") + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("reranker health error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type FallbackReranker struct{}

func (FallbackReranker) Rerank(_ context.Context, query string, hits []StageHit, limit int) ([]StageHit, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	language := DetectLanguage(query)
	queryTokens := Tokenize(query, language)
	scored := make([]StageHit, 0, len(hits))
	for _, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			content = strings.TrimSpace(hit.Chunk.Content)
		}
		contentTokens := Tokenize(content, language)
		hit.Score += tokenOverlap(queryTokens, contentTokens)
		hit.Stage = "rerank"
		scored = append(scored, hit)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].ChunkID < scored[j].ChunkID
		}
		return scored[i].Score > scored[j].Score
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored, nil
}

type ChainReranker struct {
	Primary  Reranker
	Fallback Reranker
}

func (r ChainReranker) Rerank(ctx context.Context, query string, hits []StageHit, limit int) ([]StageHit, error) {
	if r.Primary != nil {
		scored, err := r.Primary.Rerank(ctx, query, hits, limit)
		if err == nil && len(scored) > 0 {
			return scored, nil
		}
	}
	if r.Fallback != nil {
		return r.Fallback.Rerank(ctx, query, hits, limit)
	}
	return nil, fmt.Errorf("no reranker available")
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
	if len(seen) == 0 {
		return 0
	}
	return float64(matches) / float64(len(seen))
}
