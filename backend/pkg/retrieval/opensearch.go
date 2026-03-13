package retrieval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LexicalDocument is the canonical lexical record indexed into OpenSearch.
type LexicalDocument struct {
	ID      string         `json:"id"`
	Content string         `json:"content_text"`
	Terms   string         `json:"content_terms"`
	Payload map[string]any `json:"payload"`
}

// OpenSearchClient stores and searches lexical documents with BM25-style ranking.
type OpenSearchClient struct {
	baseURL    string
	index      string
	username   string
	password   string
	httpClient *http.Client
}

// NewOpenSearchClient builds a lexical client.
func NewOpenSearchClient(baseURL, index, username, password string) (*OpenSearchClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	index = strings.TrimSpace(index)
	if baseURL == "" {
		return nil, fmt.Errorf("opensearch base url required")
	}
	if index == "" {
		return nil, fmt.Errorf("opensearch index required")
	}
	return &OpenSearchClient{
		baseURL:    baseURL,
		index:      index,
		username:   strings.TrimSpace(username),
		password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// EnsureIndex creates the lexical index if missing.
func (c *OpenSearchClient) EnsureIndex(ctx context.Context) error {
	body := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"chunk_id":      map[string]any{"type": "keyword"},
				"book_id":       map[string]any{"type": "keyword"},
				"chunk_family":  map[string]any{"type": "keyword"},
				"section_id":    map[string]any{"type": "keyword"},
				"title":         map[string]any{"type": "text"},
				"section_title": map[string]any{"type": "text"},
				"keywords":      map[string]any{"type": "text"},
				"tags":          map[string]any{"type": "keyword"},
				"block_type":    map[string]any{"type": "keyword"},
				"language":      map[string]any{"type": "keyword"},
				"content_text":  map[string]any{"type": "text"},
				"content_terms": map[string]any{"type": "text"},
			},
		},
	}
	err := c.do(ctx, http.MethodPut, "/"+url.PathEscape(c.index), body, nil)
	var apiErr *apiError
	if err != nil && errorAs(err, &apiErr) {
		bodyLower := strings.ToLower(apiErr.Body)
		if apiErr.Status == http.StatusBadRequest && strings.Contains(bodyLower, "already_exists") {
			return nil
		}
	}
	return err
}

// DeleteByBook removes all lexical docs for a book.
func (c *OpenSearchClient) DeleteByBook(ctx context.Context, bookID string) error {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil
	}
	body := map[string]any{
		"query": map[string]any{
			"term": map[string]any{
				"book_id": bookID,
			},
		},
	}
	err := c.do(ctx, http.MethodPost, "/"+url.PathEscape(c.index)+"/_delete_by_query?refresh=true", body, nil)
	var apiErr *apiError
	if err != nil && errorAs(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

// IndexDocuments bulk indexes lexical docs.
func (c *OpenSearchClient) IndexDocuments(ctx context.Context, docs []LexicalDocument) error {
	if len(docs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, doc := range docs {
		meta := map[string]any{"index": map[string]any{"_index": c.index, "_id": doc.ID}}
		if err := enc.Encode(meta); err != nil {
			return err
		}
		source := map[string]any{
			"chunk_id":      doc.ID,
			"content_text":  doc.Content,
			"content_terms": doc.Terms,
			"book_id":       strings.TrimSpace(anyString(doc.Payload["book_id"])),
			"chunk_family":  strings.TrimSpace(anyString(doc.Payload["chunk_family"])),
			"section_id":    strings.TrimSpace(anyString(doc.Payload["section_id"])),
			"title":         strings.TrimSpace(anyString(doc.Payload["title"])),
			"section_title": strings.TrimSpace(anyString(doc.Payload["section_title"])),
			"keywords":      strings.TrimSpace(anyString(doc.Payload["keywords"])),
			"tags":          strings.TrimSpace(anyString(doc.Payload["tags"])),
			"block_type":    strings.TrimSpace(anyString(doc.Payload["block_type"])),
			"language":      strings.TrimSpace(anyString(doc.Payload["language"])),
		}
		if err := enc.Encode(source); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/_bulk?refresh=true", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &apiError{Status: resp.StatusCode, Body: string(data)}
	}
	var result struct {
		Errors bool `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Errors {
		return fmt.Errorf("opensearch bulk index reported partial errors")
	}
	return nil
}

// QueryBM25 runs lexical retrieval against tokenized content.
func (c *OpenSearchClient) QueryBM25(ctx context.Context, bookID, terms string, limit int) ([]Point, error) {
	bookID = strings.TrimSpace(bookID)
	terms = strings.TrimSpace(terms)
	if terms == "" || limit <= 0 {
		return nil, nil
	}
	must := []any{
		map[string]any{
			"match": map[string]any{
				"content_terms": map[string]any{
					"query":    terms,
					"operator": "or",
				},
			},
		},
	}
	if bookID != "" {
		must = append([]any{
			map[string]any{
				"term": map[string]any{
					"book_id": bookID,
				},
			},
		}, must...)
	}
	body := map[string]any{
		"size": limit,
		"query": map[string]any{
			"bool": map[string]any{
				"must": must,
			},
		},
	}
	var resp struct {
		Hits struct {
			Hits []struct {
				ID     string         `json:"_id"`
				Score  float64        `json:"_score"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := c.do(ctx, http.MethodPost, "/"+url.PathEscape(c.index)+"/_search", body, &resp); err != nil {
		var apiErr *apiError
		if errorAs(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	points := make([]Point, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		payload := map[string]any{
			"chunk_id":      strings.TrimSpace(anyString(hit.Source["chunk_id"])),
			"book_id":       strings.TrimSpace(anyString(hit.Source["book_id"])),
			"chunk_family":  strings.TrimSpace(anyString(hit.Source["chunk_family"])),
			"section_id":    strings.TrimSpace(anyString(hit.Source["section_id"])),
			"title":         strings.TrimSpace(anyString(hit.Source["title"])),
			"section_title": strings.TrimSpace(anyString(hit.Source["section_title"])),
			"keywords":      strings.TrimSpace(anyString(hit.Source["keywords"])),
			"tags":          strings.TrimSpace(anyString(hit.Source["tags"])),
			"block_type":    strings.TrimSpace(anyString(hit.Source["block_type"])),
			"language":      strings.TrimSpace(anyString(hit.Source["language"])),
		}
		points = append(points, Point{
			ID:      strings.TrimSpace(anyString(payload["chunk_id"])),
			Payload: payload,
			Score:   hit.Score,
		})
	}
	return points, nil
}

// DeleteIndex removes the lexical index.
func (c *OpenSearchClient) DeleteIndex(ctx context.Context) error {
	err := c.do(ctx, http.MethodDelete, "/"+url.PathEscape(c.index), nil, nil)
	var apiErr *apiError
	if err != nil && errorAs(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *OpenSearchClient) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &apiError{Status: resp.StatusCode, Body: string(data)}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
