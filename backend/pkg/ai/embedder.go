package ai

import "context"

// Embedder provides embeddings for text.
type Embedder interface {
	EmbedText(ctx context.Context, text, taskType string) ([]float32, error)
}

// BatchEmbedder optionally supports embedding multiple texts at once.
type BatchEmbedder interface {
	EmbedTexts(ctx context.Context, texts []string, taskType string) ([][]float32, error)
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

// EmbedTexts returns embeddings for multiple texts using Ollama.
func (e *OllamaEmbedder) EmbedTexts(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	return e.client.EmbedTexts(ctx, e.model, texts, e.dimensions)
}
