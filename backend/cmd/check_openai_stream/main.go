package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string    `json:"model"`
	Messages       []message `json:"messages"`
	Stream         bool      `json:"stream,omitempty"`
	EnableThinking *bool     `json:"enable_thinking,omitempty"`
}

type streamResponse struct {
	Choices []struct {
		Delta struct {
			Role             string          `json:"role"`
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
		Index        int     `json:"index"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type nonStreamResponse struct {
	Choices []struct {
		Message struct {
			Role             string          `json:"role"`
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
		} `json:"message"`
		FinishReason *string `json:"finish_reason"`
		Index        int     `json:"index"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type summary struct {
	Events           int
	ContentChunks    int
	ReasoningChunks  int
	SawDone          bool
	ContentText      strings.Builder
	ReasoningText    strings.Builder
	LastFinishReason string
}

func main() {
	var (
		baseURL      = flag.String("base-url", envOr("GENERATION_BASE_URL", ""), "OpenAI-compatible base URL including /v1")
		apiKey       = flag.String("api-key", envOr("GENERATION_API_KEY", ""), "API key")
		model        = flag.String("model", envOr("GENERATION_MODEL", ""), "Model name")
		systemPrompt = flag.String("system", "You are a concise assistant.", "System prompt")
		userPrompt   = flag.String("prompt", "什么是市场微观结构", "User prompt")
		timeout      = flag.Duration("timeout", 3*time.Minute, "Request timeout")
	)
	flag.Parse()

	if strings.TrimSpace(*baseURL) == "" || strings.TrimSpace(*model) == "" {
		fmt.Fprintln(os.Stderr, "base-url and model are required; set flags or export GENERATION_BASE_URL / GENERATION_MODEL")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("== Config ==")
	fmt.Printf("base_url: %s\n", *baseURL)
	fmt.Printf("model: %s\n", *model)
	fmt.Printf("prompt: %s\n", *userPrompt)
	fmt.Println()

	reqBody := chatRequest{
		Model: *model,
		Messages: []message{
			{Role: "system", Content: *systemPrompt},
			{Role: "user", Content: *userPrompt},
		},
		Stream:         true,
		EnableThinking: parseOptionalBoolEnv("GENERATION_ENABLE_THINKING"),
	}

	streamSummary, streamErr := runStream(ctx, strings.TrimRight(*baseURL, "/")+"/chat/completions", *apiKey, reqBody)
	if streamErr != nil {
		fmt.Printf("\nstream_error: %v\n", streamErr)
	}

	fmt.Println()
	fmt.Println("== Stream Summary ==")
	fmt.Printf("events: %d\n", streamSummary.Events)
	fmt.Printf("content_chunks: %d\n", streamSummary.ContentChunks)
	fmt.Printf("reasoning_chunks: %d\n", streamSummary.ReasoningChunks)
	fmt.Printf("saw_done: %t\n", streamSummary.SawDone)
	if streamSummary.LastFinishReason != "" {
		fmt.Printf("last_finish_reason: %s\n", streamSummary.LastFinishReason)
	}
	fmt.Printf("content_text_len: %d\n", streamSummary.ContentText.Len())
	fmt.Printf("reasoning_text_len: %d\n", streamSummary.ReasoningText.Len())
	if streamSummary.ContentText.Len() > 0 {
		fmt.Printf("content_preview: %q\n", preview(streamSummary.ContentText.String(), 240))
	}
	if streamSummary.ReasoningText.Len() > 0 {
		fmt.Printf("reasoning_preview: %q\n", preview(streamSummary.ReasoningText.String(), 240))
	}
	fmt.Printf("analysis: %s\n", analyzeStream(streamSummary))

	fmt.Println()
	fmt.Println("== Non-Stream Check ==")
	nonStreamText, nonStreamReasoning, nonStreamErr := runNonStream(ctx, strings.TrimRight(*baseURL, "/")+"/chat/completions", *apiKey, reqBody)
	if nonStreamErr != nil {
		fmt.Printf("non_stream_error: %v\n", nonStreamErr)
		os.Exit(1)
	}
	fmt.Printf("non_stream_content_len: %d\n", len(nonStreamText))
	if nonStreamText != "" {
		fmt.Printf("non_stream_content_preview: %q\n", preview(nonStreamText, 240))
	}
	fmt.Printf("non_stream_reasoning_len: %d\n", len(nonStreamReasoning))
	if nonStreamReasoning != "" {
		fmt.Printf("non_stream_reasoning_preview: %q\n", preview(nonStreamReasoning, 240))
	}
}

func runStream(ctx context.Context, url, apiKey string, reqBody chatRequest) (summary, error) {
	var out summary

	body, err := json.Marshal(reqBody)
	if err != nil {
		return out, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	fmt.Println("== Stream Response ==")
	fmt.Printf("status: %s\n", resp.Status)

	reader := bufio.NewReader(resp.Body)
	var dataLines []string
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return out, fmt.Errorf("read stream: %w", readErr)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) > 0 {
				processEvent(strings.Join(dataLines, "\n"), &out)
				dataLines = dataLines[:0]
			}
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}

		if readErr == io.EOF {
			if len(dataLines) > 0 {
				processEvent(strings.Join(dataLines, "\n"), &out)
			}
			break
		}
	}
	return out, nil
}

func processEvent(payload string, out *summary) {
	if payload == "" {
		return
	}

	out.Events++
	fmt.Printf("\n[event %d raw]\n%s\n", out.Events, payload)

	if payload == "[DONE]" {
		out.SawDone = true
		return
	}

	var event streamResponse
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		fmt.Printf("parse_error: %v\n", err)
		return
	}
	if event.Error.Message != "" {
		fmt.Printf("upstream_error: %s\n", event.Error.Message)
		return
	}

	for _, choice := range event.Choices {
		content := decodeContent(choice.Delta.Content)
		reasoning := decodeContent(choice.Delta.ReasoningContent)
		if choice.FinishReason != nil {
			out.LastFinishReason = *choice.FinishReason
		}
		fmt.Printf("choice[%d].role=%q content=%q reasoning=%q finish_reason=%q\n",
			choice.Index, choice.Delta.Role, content, reasoning, out.LastFinishReason)
		if content != "" {
			out.ContentChunks++
			out.ContentText.WriteString(content)
		}
		if reasoning != "" {
			out.ReasoningChunks++
			out.ReasoningText.WriteString(reasoning)
		}
	}
}

func runNonStream(ctx context.Context, url, apiKey string, reqBody chatRequest) (string, string, error) {
	reqBody.Stream = false
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	fmt.Printf("status: %s\n", resp.Status)

	var bodyResp nonStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&bodyResp); err != nil {
		return "", "", fmt.Errorf("decode non-stream: %w", err)
	}
	if bodyResp.Error.Message != "" {
		return "", "", fmt.Errorf("upstream error: %s", bodyResp.Error.Message)
	}
	if len(bodyResp.Choices) == 0 {
		return "", "", fmt.Errorf("non-stream returned no choices")
	}

	content := strings.TrimSpace(decodeContent(bodyResp.Choices[0].Message.Content))
	reasoning := strings.TrimSpace(decodeContent(bodyResp.Choices[0].Message.ReasoningContent))
	return content, reasoning, nil
}

func decodeContent(raw json.RawMessage) string {
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
			}
			if text, ok := part["content"].(string); ok {
				out.WriteString(text)
			}
		}
		return out.String()
	}

	return string(trimmed)
}

func analyzeStream(s summary) string {
	switch {
	case s.ContentChunks > 0:
		return "upstream emitted usable streaming content"
	case s.ReasoningChunks > 0:
		return "upstream emitted reasoning-only chunks without answer content; stream is connected but not usable for visible token output"
	case s.Events == 0:
		return "no SSE events were received"
	default:
		return "SSE events arrived but no recognizable content fields were present"
	}
}

func preview(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseOptionalBoolEnv(key string) *bool {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &parsed
}
