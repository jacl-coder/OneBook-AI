package ai

import "context"

// TextGenerator generates text from a system prompt and user prompt.
// All LLM providers (Gemini, Ollama, OpenAI-compatible) implement this interface.
type TextGenerator interface {
	GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// StreamingTextGenerator emits incremental text chunks while also returning the
// final aggregated response once generation completes.
type StreamingTextGenerator interface {
	GenerateTextStream(ctx context.Context, systemPrompt, userPrompt string, onChunk func(string) error) (string, error)
}
