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

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// GeminiClient calls the Google AI Studio (Gemini) API.
type GeminiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewGeminiClient constructs a client with the provided API key.
func NewGeminiClient(apiKey string) (*GeminiClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("gemini api key required")
	}
	return &GeminiClient{
		apiKey:     apiKey,
		baseURL:    defaultGeminiBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// EmbedText generates an embedding for the input text.
func (c *GeminiClient) EmbedText(ctx context.Context, model string, text string, taskType string) ([]float32, error) {
	reqBody := embedRequest{
		Content: content{
			Parts: []part{{Text: text}},
		},
	}
	if taskType != "" {
		reqBody.TaskType = taskType
	}
	var resp embedResponse
	if err := c.doJSON(ctx, fmt.Sprintf("%s/models/%s:embedContent?key=%s", c.baseURL, normalizeModel(model), c.apiKey), reqBody, &resp); err != nil {
		return nil, err
	}
	return resp.Embedding.Values, nil
}

// GenerateText returns the generated response for a prompt.
func (c *GeminiClient) GenerateText(ctx context.Context, model, systemPrompt, userPrompt string) (string, error) {
	reqBody := generateRequest{
		Contents: []content{
			{
				Role:  "user",
				Parts: []part{{Text: userPrompt}},
			},
		},
	}
	if strings.TrimSpace(systemPrompt) != "" {
		reqBody.SystemInstruction = &content{
			Parts: []part{{Text: systemPrompt}},
		}
	}
	var resp generateResponse
	if err := c.doJSON(ctx, fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, normalizeModel(model), c.apiKey), reqBody, &resp); err != nil {
		return "", err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}

func normalizeModel(model string) string {
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "models/")
	return model
}

func (c *GeminiClient) doJSON(ctx context.Context, url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var errResp errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return fmt.Errorf("gemini api error: %s", errResp.Error.Message)
		}
		return fmt.Errorf("gemini api error: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type embedRequest struct {
	Content  content `json:"content"`
	TaskType string  `json:"taskType,omitempty"`
}

type embedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

type generateRequest struct {
	Contents          []content `json:"contents"`
	SystemInstruction *content  `json:"systemInstruction,omitempty"`
}

type generateResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
