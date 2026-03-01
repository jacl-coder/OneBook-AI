package ai

import "context"

// GeminiGenerator wraps GeminiClient with a fixed model for text generation.
type GeminiGenerator struct {
	client *GeminiClient
	model  string
}

// NewGeminiGenerator builds a Gemini-based TextGenerator.
func NewGeminiGenerator(client *GeminiClient, model string) *GeminiGenerator {
	return &GeminiGenerator{client: client, model: model}
}

// GenerateText implements TextGenerator using Gemini.
func (g *GeminiGenerator) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return g.client.GenerateText(ctx, g.model, systemPrompt, userPrompt)
}
