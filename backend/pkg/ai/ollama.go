package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultOllamaBaseURL = "http://127.0.0.1:11434"

// OllamaClient calls the Ollama HTTP API.
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOllamaClient constructs a client with the provided base URL.
func NewOllamaClient(baseURL string) *OllamaClient {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OllamaClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// EmbedText generates an embedding for the input text.
func (c *OllamaClient) EmbedText(ctx context.Context, model string, text string, dimensions int) ([]float32, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("ollama embedding model required")
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("embedding text required")
	}

	reqBody := ollamaEmbedRequest{
		Model: model,
		Input: text,
	}
	if dimensions > 0 {
		reqBody.Dimensions = dimensions
	}

	var resp ollamaEmbedResponse
	status, err := c.doJSON(ctx, "/api/embed", reqBody, &resp)
	if err != nil {
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			return c.embedLegacy(ctx, model, text)
		}
		return nil, err
	}

	if len(resp.Embeddings) > 0 {
		return resp.Embeddings[0], nil
	}
	if len(resp.Embedding) > 0 {
		return resp.Embedding, nil
	}
	return nil, fmt.Errorf("ollama embed response missing embeddings")
}

func (c *OllamaClient) embedLegacy(ctx context.Context, model, text string) ([]float32, error) {
	reqBody := ollamaLegacyEmbedRequest{
		Model:  model,
		Prompt: text,
	}
	var resp ollamaLegacyEmbedResponse
	if _, err := c.doJSON(ctx, "/api/embeddings", reqBody, &resp); err != nil {
		return nil, err
	}
	if len(resp.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embedding response missing embedding")
	}
	return resp.Embedding, nil
}

func (c *OllamaClient) doJSON(ctx context.Context, path string, payload any, out any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp ollamaErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return resp.StatusCode, fmt.Errorf("ollama api error: %s", errResp.Error)
		}
		return resp.StatusCode, fmt.Errorf("ollama api error: %s", resp.Status)
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

type ollamaEmbedRequest struct {
	Model      string `json:"model"`
	Input      any    `json:"input"`
	Dimensions int    `json:"dimensions,omitempty"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Embedding  []float32   `json:"embedding"`
}

type ollamaLegacyEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaLegacyEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type ollamaErrorResponse struct {
	Error string `json:"error"`
}
