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

// OpenAICompatGenerator calls any OpenAI-compatible /v1/chat/completions endpoint.
// Works with vLLM, LiteLLM, LocalAI, Deepseek, OpenRouter, self-hosted models, etc.
type OpenAICompatGenerator struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAICompatGenerator builds an OpenAI-compatible TextGenerator.
// baseURL should include the /v1 prefix, e.g. "http://localhost:8000/v1".
// apiKey can be empty for local models that do not require authentication.
func NewOpenAICompatGenerator(baseURL, apiKey, model string) *OpenAICompatGenerator {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &OpenAICompatGenerator{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// GenerateText implements TextGenerator using the OpenAI chat completions API.
func (g *OpenAICompatGenerator) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if g.model == "" {
		return "", fmt.Errorf("openai-compat generation model required")
	}
	messages := make([]oaiMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, oaiMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, oaiMessage{Role: "user", Content: userPrompt})

	reqBody := oaiChatRequest{
		Model:    g.model,
		Messages: messages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := g.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai-compat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp oaiErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("openai-compat api error: %s", errResp.Error.Message)
		}
		return "", fmt.Errorf("openai-compat api error: %s", resp.Status)
	}

	var chatResp oaiChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("openai-compat decode: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from openai-compat api")
	}
	text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("empty response from openai-compat api")
	}
	return text, nil
}

// OpenAI-compatible request/response types.

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiChatRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
}

type oaiChatResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
}

type oaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
