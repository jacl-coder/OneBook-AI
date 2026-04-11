package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/util"
)

// OpenAICompatGenerator calls any OpenAI-compatible /v1/chat/completions endpoint.
// Works with vLLM, LiteLLM, LocalAI, Deepseek, OpenRouter, self-hosted models, etc.
type OpenAICompatGenerator struct {
	baseURL          string
	apiKey           string
	model            string
	enableThinking   *bool
	httpClient       *http.Client
	streamHTTPClient *http.Client
}

type OpenAICompatConfig struct {
	EnableThinking *bool
}

// NewOpenAICompatGenerator builds an OpenAI-compatible TextGenerator.
// baseURL should include the /v1 prefix, e.g. "http://localhost:8000/v1".
// apiKey can be empty for local models that do not require authentication.
func NewOpenAICompatGenerator(baseURL, apiKey, model string, cfg OpenAICompatConfig) *OpenAICompatGenerator {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &OpenAICompatGenerator{
		baseURL:        baseURL,
		apiKey:         strings.TrimSpace(apiKey),
		model:          strings.TrimSpace(model),
		enableThinking: cfg.EnableThinking,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		streamHTTPClient: &http.Client{},
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
		Model:          g.model,
		Messages:       messages,
		EnableThinking: g.enableThinking,
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
	text := strings.TrimSpace(decodeOAIContent(chatResp.Choices[0].Message.Content))
	if text == "" {
		log := util.LoggerFromContext(ctx)
		if reasoning := strings.TrimSpace(decodeOAIContent(chatResp.Choices[0].Message.ReasoningContent)); reasoning != "" {
			log.Warn("openai_compat_nonstream_reasoning_without_content", "model", g.model, "reasoning_len", len(reasoning))
		}
		return "", fmt.Errorf("empty response from openai-compat api")
	}
	return text, nil
}

// GenerateTextStream implements StreamingTextGenerator using streaming chat
// completions when the upstream provider supports OpenAI-compatible SSE.
func (g *OpenAICompatGenerator) GenerateTextStream(ctx context.Context, systemPrompt, userPrompt string, onChunk func(string) error) (string, error) {
	if g.model == "" {
		return "", fmt.Errorf("openai-compat generation model required")
	}
	log := util.LoggerFromContext(ctx)
	messages := make([]oaiMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, oaiMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, oaiMessage{Role: "user", Content: userPrompt})

	reqBody := oaiChatRequest{
		Model:          g.model,
		Messages:       messages,
		Stream:         true,
		EnableThinking: g.enableThinking,
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
	req.Header.Set("Accept", "text/event-stream")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.streamHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai-compat stream request: %w", err)
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

	text, err := consumeOAIStream(ctx, g.model, resp.Body, onChunk)
	if err != nil {
		return "", err
	}
	if text == "" {
		log.Warn("openai_compat_stream_empty_response", "model", g.model, "raw_text_len", len(text))
		log.Info("openai_compat_stream_fallback_to_nonstream", "model", g.model)
		return g.GenerateText(ctx, systemPrompt, userPrompt)
	}
	return text, nil
}

func consumeOAIStream(ctx context.Context, model string, body io.Reader, onChunk func(string) error) (string, error) {
	log := util.LoggerFromContext(ctx)
	var full strings.Builder
	reader := bufio.NewReader(body)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errorsIsEOF(readErr) {
			return "", fmt.Errorf("openai-compat stream read: %w", readErr)
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				log.Info("openai_compat_stream_payload", "model", model, "payload", payload)
				break
			}
			if payload != "" {
				log.Info("openai_compat_stream_payload", "model", model, "payload", payload)
				var chunkResp oaiChatStreamResponse
				if err := json.Unmarshal([]byte(payload), &chunkResp); err != nil {
					return "", fmt.Errorf("openai-compat stream decode: %w", err)
				}
				if chunkResp.Error.Message != "" {
					return "", fmt.Errorf("openai-compat api error: %s", chunkResp.Error.Message)
				}
				for _, choice := range chunkResp.Choices {
					delta := decodeOAIContent(choice.Delta.Content)
					if delta == "" {
						continue
					}
					full.WriteString(delta)
					if onChunk != nil {
						if err := onChunk(delta); err != nil {
							return full.String(), err
						}
					}
				}
			}
		}
		if errorsIsEOF(readErr) {
			break
		}
	}
	return strings.TrimSpace(full.String()), nil
}

// OpenAI-compatible request/response types.

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiChatRequest struct {
	Model          string       `json:"model"`
	Messages       []oaiMessage `json:"messages"`
	Stream         bool         `json:"stream,omitempty"`
	EnableThinking *bool        `json:"enable_thinking,omitempty"`
}

type oaiChatResponse struct {
	Choices []struct {
		Message struct {
			Role             string          `json:"role"`
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

type oaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type oaiChatStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func decodeOAIContent(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		return text
	}

	var parts []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(trimmed, &parts); err == nil {
		var out strings.Builder
		for _, part := range parts {
			out.WriteString(part.Text)
			out.WriteString(part.Content)
		}
		return out.String()
	}

	var partObjects []map[string]any
	if err := json.Unmarshal(trimmed, &partObjects); err == nil {
		var out strings.Builder
		for _, part := range partObjects {
			if text, ok := part["text"].(string); ok {
				out.WriteString(text)
				continue
			}
			if text, ok := part["content"].(string); ok {
				out.WriteString(text)
			}
		}
		return out.String()
	}

	return ""
}

func errorsIsEOF(err error) bool {
	return errors.Is(err, io.EOF)
}
