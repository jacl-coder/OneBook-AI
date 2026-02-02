package ai

import "context"

// Embedder provides embeddings for text.
type Embedder interface {
	EmbedText(ctx context.Context, text, taskType string) ([]float32, error)
}

// GeminiEmbedder wraps Gemini embedding calls with a fixed model.
type GeminiEmbedder struct {
	client *GeminiClient
	model  string
}

// NewGeminiEmbedder builds a Gemini-based embedder.
func NewGeminiEmbedder(client *GeminiClient, model string) *GeminiEmbedder {
	return &GeminiEmbedder{client: client, model: model}
}

// EmbedText returns embeddings for text using Gemini.
func (e *GeminiEmbedder) EmbedText(ctx context.Context, text, taskType string) ([]float32, error) {
	return e.client.EmbedText(ctx, e.model, text, taskType)
}

// OllamaEmbedder wraps Ollama embedding calls with a fixed model and dimension.
type OllamaEmbedder struct {
	client     *OllamaClient
	model      string
	dimensions int
}

// NewOllamaEmbedder builds an Ollama-based embedder.
func NewOllamaEmbedder(client *OllamaClient, model string, dimensions int) *OllamaEmbedder {
	return &OllamaEmbedder{client: client, model: model, dimensions: dimensions}
}

// EmbedText returns embeddings for text using Ollama.
func (e *OllamaEmbedder) EmbedText(ctx context.Context, text, taskType string) ([]float32, error) {
	return e.client.EmbedText(ctx, e.model, text, e.dimensions)
}
