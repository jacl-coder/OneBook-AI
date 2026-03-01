package ai

import (
	"context"
	"fmt"
	"strings"
)

// OllamaGenerator wraps OllamaClient with a fixed model for text generation
// using the Ollama /api/chat endpoint.
type OllamaGenerator struct {
	client *OllamaClient
	model  string
}

// NewOllamaGenerator builds an Ollama-based TextGenerator.
func NewOllamaGenerator(client *OllamaClient, model string) *OllamaGenerator {
	return &OllamaGenerator{client: client, model: model}
}

// GenerateText implements TextGenerator using Ollama /api/chat.
func (g *OllamaGenerator) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	model := strings.TrimSpace(g.model)
	if model == "" {
		return "", fmt.Errorf("ollama generation model required")
	}

	messages := make([]ollamaChatMessage, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, ollamaChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ollamaChatMessage{Role: "user", Content: userPrompt})

	reqBody := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	var resp ollamaChatResponse
	if _, err := g.client.doJSON(ctx, "/api/chat", reqBody, &resp); err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	if strings.TrimSpace(resp.Message.Content) == "" {
		return "", fmt.Errorf("empty response from ollama")
	}
	return resp.Message.Content, nil
}

// Ollama /api/chat request/response types.

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
}
