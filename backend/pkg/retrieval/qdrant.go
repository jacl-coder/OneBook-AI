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

	"github.com/google/uuid"
)

type SparseVector struct {
	Indices []uint32  `json:"indices"`
	Values  []float32 `json:"values"`
}

type Point struct {
	ID      string         `json:"id"`
	Payload map[string]any `json:"payload"`
	Score   float64        `json:"score"`
}

type UpsertPoint struct {
	ID      string         `json:"id"`
	Dense   []float32      `json:"dense"`
	Sparse  SparseVector   `json:"sparse"`
	Payload map[string]any `json:"payload"`
}

type Client struct {
	baseURL    string
	apiKey     string
	collection string
	denseSize  int
	httpClient *http.Client
}

var qdrantPointNamespace = uuid.MustParse("2b6d13ed-63bb-4d0d-9a8e-2e6bdb5d2f15")

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	msg := strings.TrimSpace(e.Body)
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return fmt.Sprintf("qdrant error (%d): %s", e.Status, msg)
}

func NewQdrantClient(baseURL, apiKey, collection string, denseSize int) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	collection = strings.TrimSpace(collection)
	if baseURL == "" {
		return nil, fmt.Errorf("qdrant base url required")
	}
	if collection == "" {
		return nil, fmt.Errorf("qdrant collection required")
	}
	if denseSize <= 0 {
		return nil, fmt.Errorf("qdrant dense size required")
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(apiKey),
		collection: collection,
		denseSize:  denseSize,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) EnsureCollection(ctx context.Context) error {
	reqBody := map[string]any{
		"vectors": map[string]any{
			"dense": map[string]any{
				"size":     c.denseSize,
				"distance": "Cosine",
			},
		},
		"sparse_vectors": map[string]any{
			"sparse": map[string]any{},
		},
	}
	err := c.do(ctx, http.MethodPut, "/collections/"+url.PathEscape(c.collection), reqBody, nil)
	var apiErr *apiError
	if err != nil && errorAs(err, &apiErr) && apiErr.Status == http.StatusConflict {
		body := strings.ToLower(apiErr.Body)
		if strings.Contains(body, "already exists") || strings.Contains(body, "collection") {
			return nil
		}
	}
	return err
}

func (c *Client) UpsertPoints(ctx context.Context, points []UpsertPoint) error {
	if len(points) == 0 {
		return nil
	}
	payloadPoints := make([]map[string]any, 0, len(points))
	for _, point := range points {
		payloadPoints = append(payloadPoints, map[string]any{
			"id": qdrantPointID(point.ID, point.Payload),
			"vector": map[string]any{
				"dense":  point.Dense,
				"sparse": point.Sparse,
			},
			"payload": point.Payload,
		})
	}
	return c.do(ctx, http.MethodPut, "/collections/"+url.PathEscape(c.collection)+"/points?wait=true", map[string]any{
		"points": payloadPoints,
	}, nil)
}

func (c *Client) DeleteByBook(ctx context.Context, bookID string) error {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil
	}
	err := c.do(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/delete?wait=true", map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "book_id",
					"match": map[string]any{
						"value": bookID,
					},
				},
			},
		},
	}, nil)
	var apiErr *apiError
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	if err != nil && errorAs(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *Client) QueryDense(ctx context.Context, bookID string, vector []float32, limit int) ([]Point, error) {
	if len(vector) == 0 || limit <= 0 {
		return nil, nil
	}
	return c.query(ctx, bookID, map[string]any{
		"using":        "dense",
		"query":        vector,
		"limit":        limit,
		"with_payload": true,
	})
}

func (c *Client) QuerySparse(ctx context.Context, bookID string, vector SparseVector, limit int) ([]Point, error) {
	if len(vector.Indices) == 0 || len(vector.Values) == 0 || limit <= 0 {
		return nil, nil
	}
	return c.query(ctx, bookID, map[string]any{
		"using":        "sparse",
		"query":        vector,
		"limit":        limit,
		"with_payload": true,
	})
}

func (c *Client) query(ctx context.Context, bookID string, payload map[string]any) ([]Point, error) {
	payload["filter"] = map[string]any{
		"must": []map[string]any{
			{
				"key": "book_id",
				"match": map[string]any{
					"value": strings.TrimSpace(bookID),
				},
			},
		},
	}
	var resp qdrantQueryResponse
	if err := c.do(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/query", payload, &resp); err != nil {
		var apiErr *apiError
		if errorAs(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return resp.Points(), nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
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
	if c.apiKey != "" {
		req.Header.Set("api-key", c.apiKey)
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

func errorAs(err error, target **apiError) bool {
	if err == nil || target == nil {
		return false
	}
	apiErr, ok := err.(*apiError)
	if !ok {
		return false
	}
	*target = apiErr
	return true
}

type qdrantQueryResponse struct {
	Result any `json:"result"`
}

func (r qdrantQueryResponse) Points() []Point {
	switch result := r.Result.(type) {
	case []any:
		return parseQdrantPoints(result)
	case map[string]any:
		if points, ok := result["points"].([]any); ok {
			return parseQdrantPoints(points)
		}
		if points, ok := result["result"].([]any); ok {
			return parseQdrantPoints(points)
		}
	}
	return nil
}

func parseQdrantPoints(items []any) []Point {
	points := make([]Point, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		point := Point{
			ID:      anyString(row["id"]),
			Score:   anyFloat64(row["score"]),
			Payload: map[string]any{},
		}
		if payload, ok := row["payload"].(map[string]any); ok {
			point.Payload = payload
			if chunkID := anyString(payload["chunk_id"]); chunkID != "" {
				point.ID = chunkID
			}
		}
		points = append(points, point)
	}
	return points
}

func qdrantPointID(sourceID string, payload map[string]any) string {
	bookID := strings.TrimSpace(anyString(payload["book_id"]))
	chunkID := strings.TrimSpace(anyString(payload["chunk_id"]))
	baseID := strings.TrimSpace(sourceID)
	if chunkID != "" {
		baseID = chunkID
	}
	if baseID == "" {
		baseID = strings.TrimSpace(anyString(payload["content_sha256"]))
	}
	if bookID != "" {
		baseID = bookID + ":" + baseID
	}
	if baseID == "" {
		baseID = "onebookai-qdrant-point"
	}
	return uuid.NewSHA1(qdrantPointNamespace, []byte(baseID)).String()
}

func anyString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", v))
	default:
		return ""
	}
}

func anyFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
