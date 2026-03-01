package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/store"
)

const defaultConversationTitle = "新对话"

// Config holds runtime configuration for the core application.
type Config struct {
	DatabaseURL       string
	Store             store.Store
	GeminiAPIKey      string
	GenerationModel   string
	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingModel    string
	EmbeddingDim      int
	TopK              int
	HistoryLimit      int
}

// App is the core application service wiring together storage and chat logic.
type App struct {
	store           store.Store
	gemini          *ai.GeminiClient
	embedder        ai.Embedder
	generationModel string
	topK            int
	historyLimit    int
}

// New constructs the application with database-backed storage for messages.
func New(cfg Config) (*App, error) {
	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required")
		}
		var err error
		dataStore, err = store.NewGormStore(cfg.DatabaseURL, store.WithEmbeddingDim(cfg.EmbeddingDim))
		if err != nil {
			return nil, fmt.Errorf("init postgres store: %w", err)
		}
	}

	if cfg.GenerationModel == "" {
		return nil, fmt.Errorf("generation model required")
	}
	if cfg.EmbeddingModel == "" {
		return nil, fmt.Errorf("embedding model required")
	}
	gemini, err := ai.NewGeminiClient(cfg.GeminiAPIKey)
	if err != nil {
		return nil, err
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.EmbeddingProvider))
	if provider == "" {
		provider = "ollama"
	}
	var embedder ai.Embedder
	switch provider {
	case "ollama":
		if cfg.EmbeddingDim <= 0 {
			return nil, fmt.Errorf("embedding dim required for ollama")
		}
		ollama := ai.NewOllamaClient(cfg.EmbeddingBaseURL)
		embedder = ai.NewOllamaEmbedder(ollama, cfg.EmbeddingModel, cfg.EmbeddingDim)
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", provider)
	}
	topK := cfg.TopK
	if topK <= 0 {
		topK = 4
	}
	historyLimit := cfg.HistoryLimit
	if historyLimit < 0 {
		historyLimit = 0
	}

	return &App{
		store:           dataStore,
		gemini:          gemini,
		embedder:        embedder,
		generationModel: cfg.GenerationModel,
		topK:            topK,
		historyLimit:    historyLimit,
	}, nil
}

// AskQuestion performs a placeholder question/answer flow bound to a book and conversation.
func (a *App) AskQuestion(user domain.User, book domain.Book, question string, conversationID string) (domain.Answer, error) {
	if book.OwnerID != user.ID && user.Role != domain.RoleAdmin {
		return domain.Answer{}, fmt.Errorf("forbidden")
	}
	if book.Status != domain.StatusReady {
		return domain.Answer{}, ErrBookNotReady
	}
	if strings.TrimSpace(question) == "" {
		return domain.Answer{}, fmt.Errorf("question required")
	}
	conversation, err := a.ensureConversation(user, book, question, conversationID)
	if err != nil {
		return domain.Answer{}, err
	}
	ctx := context.Background()
	queryEmbedding, err := a.embedder.EmbedText(ctx, question, "RETRIEVAL_QUERY")
	if err != nil {
		return domain.Answer{}, fmt.Errorf("embed question: %w", err)
	}
	chunks, err := a.store.SearchChunks(book.ID, queryEmbedding, a.topK)
	if err != nil {
		return domain.Answer{}, fmt.Errorf("search chunks: %w", err)
	}
	if len(chunks) == 0 {
		return domain.Answer{}, ErrBookNotReady
	}
	var history []domain.Message
	if a.historyLimit > 0 {
		historyLimit := a.historyLimit * 2
		if historyLimit < a.historyLimit {
			historyLimit = a.historyLimit
		}
		history, err = a.store.ListConversationMessages(conversation.ID, historyLimit)
		if err != nil {
			return domain.Answer{}, fmt.Errorf("load history: %w", err)
		}
	}
	historyText := buildHistory(history)
	contextText, sources := buildContext(chunks)
	var userPrompt string
	if historyText != "" {
		userPrompt = fmt.Sprintf("书名：%s\n对话历史：\n%s\n\n当前问题：%s\n\n已知资料：\n%s\n\n请基于已知资料回答问题，如果资料不足请说明。", book.Title, historyText, question, contextText)
	} else {
		userPrompt = fmt.Sprintf("书名：%s\n问题：%s\n\n已知资料：\n%s\n\n请基于已知资料回答问题，如果资料不足请说明。", book.Title, question, contextText)
	}
	systemPrompt := "你是一个可靠的读书助手，必须基于提供的资料回答，并在回答中标注引用编号。"
	response, err := a.gemini.GenerateText(ctx, a.generationModel, systemPrompt, userPrompt)
	if err != nil {
		return domain.Answer{}, fmt.Errorf("generate answer: %w", err)
	}
	answer := domain.Answer{
		ConversationID: conversation.ID,
		BookID:         book.ID,
		Question:       question,
		Answer:         response,
		Sources:        sources,
		CreatedAt:      time.Now().UTC(),
	}
	userMessageTime := time.Now().UTC()
	if err := a.store.AppendConversationMessage(conversation.ID, domain.Message{
		ID:             util.NewID(),
		ConversationID: conversation.ID,
		UserID:         user.ID,
		BookID:         book.ID,
		Role:           "user",
		Content:        question,
		CreatedAt:      userMessageTime,
	}); err != nil {
		return domain.Answer{}, fmt.Errorf("save user message: %w", err)
	}
	assistantMessageTime := time.Now().UTC()
	if err := a.store.AppendConversationMessage(conversation.ID, domain.Message{
		ID:             util.NewID(),
		ConversationID: conversation.ID,
		UserID:         user.ID,
		BookID:         book.ID,
		Role:           "assistant",
		Content:        answer.Answer,
		Sources:        answer.Sources,
		CreatedAt:      assistantMessageTime,
	}); err != nil {
		return domain.Answer{}, fmt.Errorf("save answer message: %w", err)
	}
	if err := a.store.UpdateConversation(conversation.ID, "", assistantMessageTime); err != nil {
		return domain.Answer{}, fmt.Errorf("update conversation: %w", err)
	}
	return answer, nil
}

// ListConversations lists recent conversations for current user.
func (a *App) ListConversations(user domain.User, limit int) ([]domain.Conversation, error) {
	if strings.TrimSpace(user.ID) == "" {
		return nil, fmt.Errorf("user id required")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	items, err := a.store.ListConversationsByUser(user.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	return items, nil
}

// ListConversationMessages lists conversation messages in chronological order.
func (a *App) ListConversationMessages(user domain.User, conversationID string, limit int) ([]domain.Message, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("conversation id required")
	}
	conversation, ok, err := a.store.GetConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("load conversation: %w", err)
	}
	if !ok {
		return nil, ErrConversationNotFound
	}
	if conversation.UserID != user.ID && user.Role != domain.RoleAdmin {
		return nil, ErrConversationForbidden
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	items, err := a.store.ListConversationMessages(conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversation messages: %w", err)
	}
	return items, nil
}

func (a *App) ensureConversation(user domain.User, book domain.Book, question string, conversationID string) (domain.Conversation, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		conversation, ok, err := a.store.GetConversation(conversationID)
		if err != nil {
			return domain.Conversation{}, fmt.Errorf("load conversation: %w", err)
		}
		if !ok {
			return domain.Conversation{}, ErrConversationNotFound
		}
		if conversation.UserID != user.ID && user.Role != domain.RoleAdmin {
			return domain.Conversation{}, ErrConversationForbidden
		}
		if conversation.BookID != "" && conversation.BookID != book.ID {
			return domain.Conversation{}, fmt.Errorf("conversation book mismatch")
		}
		return conversation, nil
	}

	now := time.Now().UTC()
	conversation := domain.Conversation{
		ID:            util.NewID(),
		UserID:        user.ID,
		BookID:        book.ID,
		Title:         generateConversationTitle(question),
		LastMessageAt: &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := a.store.CreateConversation(conversation); err != nil {
		return domain.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return conversation, nil
}

func generateConversationTitle(question string) string {
	text := strings.TrimSpace(strings.ReplaceAll(question, "\n", " "))
	if text == "" {
		return defaultConversationTitle
	}
	for _, prefix := range []string{"请问一下", "请问", "麻烦你", "麻烦", "帮我", "可以帮我", "能帮我", "我想问一下", "我想问", "我想了解一下", "我想了解", "请你", "请", "关于", "有关"} {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
			break
		}
	}
	text = strings.TrimSuffix(text, "吗")
	text = strings.TrimSuffix(text, "呢")
	text = strings.TrimSuffix(text, "？")
	text = strings.TrimSuffix(text, "?")
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultConversationTitle
	}
	runes := []rune(text)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	return text
}

func buildContext(chunks []domain.Chunk) (string, []domain.Source) {
	var sb strings.Builder
	sources := make([]domain.Source, 0, len(chunks))
	for i, chunk := range chunks {
		label := fmt.Sprintf("[%d]", i+1)
		location := chunkLocation(chunk.Metadata)
		snippet := chunk.Content
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		sb.WriteString(label)
		if location != "" {
			sb.WriteString(" (" + location + ")")
		}
		sb.WriteString(" ")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n\n")
		sources = append(sources, domain.Source{
			Label:    label,
			Location: location,
			Snippet:  snippet,
		})
	}
	return sb.String(), sources
}

func buildHistory(messages []domain.Message) string {
	if len(messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "user":
			role = "用户"
		case "assistant":
			role = "助手"
		default:
			if role == "" {
				role = "消息"
			}
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func chunkLocation(meta map[string]string) string {
	if meta == nil {
		return ""
	}
	if ref := strings.TrimSpace(meta["source_ref"]); ref != "" {
		parts := strings.SplitN(ref, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0] + " " + parts[1])
		}
		return ref
	}
	if page := strings.TrimSpace(meta["page"]); page != "" {
		return "page " + page
	}
	if section := strings.TrimSpace(meta["section"]); section != "" {
		return section
	}
	if idx := strings.TrimSpace(meta["chunk"]); idx != "" {
		return "chunk " + idx
	}
	return ""
}
