package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
	"onebookai/pkg/store"
)

const defaultConversationTitle = "新对话"

// Config holds runtime configuration for the core application.
type Config struct {
	DatabaseURL              string
	Store                    store.Store
	GenerationProvider       string
	GenerationBaseURL        string
	GenerationAPIKey         string
	GenerationModel          string
	GenerationEnableThinking *bool
	EmbeddingProvider        string
	EmbeddingBaseURL         string
	EmbeddingModel           string
	EmbeddingDim             int
	TopK                     int
	DenseRecallTopK          int
	LexicalRecallTopK        int
	DenseWeight              float64
	LexicalWeight            float64
	FusionTopK               int
	HistoryLimit             int
	QdrantURL                string
	QdrantAPIKey             string
	QdrantCollection         string
	OpenSearchURL            string
	OpenSearchIndex          string
	OpenSearchUsername       string
	OpenSearchPassword       string
	RerankTopN               int
	RetrievalMode            string
	RerankerURL              string
	ContextBudget            int
	MinEvidenceCount         int
	QueryRewriteEnabled      bool
	MultiQueryEnabled        bool
	AbstainEnabled           bool
}

// App is the core application service wiring together storage and chat logic.
type App struct {
	store               store.Store
	generator           ai.TextGenerator
	embedder            ai.Embedder
	search              *retrieval.Client
	lexical             *retrieval.OpenSearchClient
	rewriter            QueryRewriter
	reranker            retrieval.Reranker
	validator           GroundingValidator
	topK                int
	denseRecallTopK     int
	lexicalRecallTopK   int
	denseWeight         float64
	lexicalWeight       float64
	fusionTopK          int
	historyLimit        int
	rerankTopN          int
	retrievalMode       string
	contextBudget       int
	minEvidenceCount    int
	queryRewriteEnabled bool
	multiQueryEnabled   bool
	abstainEnabled      bool
}

// New constructs the application with database-backed storage for messages.
func New(cfg Config) (*App, error) {
	dataStore := cfg.Store
	if dataStore == nil {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("database URL required")
		}
		var err error
		dataStore, err = store.NewGormStore(cfg.DatabaseURL)
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
	searchClient, err := retrieval.NewQdrantClient(cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantCollection, cfg.EmbeddingDim)
	if err != nil {
		return nil, fmt.Errorf("init qdrant client: %w", err)
	}
	lexicalClient, err := retrieval.NewOpenSearchClient(cfg.OpenSearchURL, cfg.OpenSearchIndex, cfg.OpenSearchUsername, cfg.OpenSearchPassword)
	if err != nil {
		return nil, fmt.Errorf("init opensearch client: %w", err)
	}

	// Build text generator based on provider.
	generator, err := buildGenerator(cfg)
	if err != nil {
		return nil, fmt.Errorf("init text generator: %w", err)
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
		topK = 5
	}
	denseRecallTopK := cfg.DenseRecallTopK
	if denseRecallTopK <= 0 {
		denseRecallTopK = 40
	}
	lexicalRecallTopK := cfg.LexicalRecallTopK
	if lexicalRecallTopK <= 0 {
		lexicalRecallTopK = 60
	}
	fusionTopK := cfg.FusionTopK
	if fusionTopK <= 0 {
		fusionTopK = 30
	}
	denseWeight := cfg.DenseWeight
	lexicalWeight := cfg.LexicalWeight
	if denseWeight <= 0 && lexicalWeight <= 0 {
		denseWeight = 0.45
		lexicalWeight = 0.55
	}
	historyLimit := cfg.HistoryLimit
	if historyLimit < 0 {
		historyLimit = 0
	}
	rerankTopN := cfg.RerankTopN
	if rerankTopN <= 0 {
		rerankTopN = 12
	}
	retrievalMode := strings.TrimSpace(cfg.RetrievalMode)
	if retrievalMode == "" {
		retrievalMode = "hybrid_best"
	}
	contextBudget := cfg.ContextBudget
	if contextBudget <= 0 {
		contextBudget = 2200
	}
	minEvidenceCount := cfg.MinEvidenceCount
	if minEvidenceCount <= 0 {
		minEvidenceCount = 2
	}
	queryRewriteEnabled := cfg.QueryRewriteEnabled
	multiQueryEnabled := cfg.MultiQueryEnabled
	abstainEnabled := cfg.AbstainEnabled

	return &App{
		store:     dataStore,
		generator: generator,
		embedder:  embedder,
		search:    searchClient,
		lexical:   lexicalClient,
		rewriter:  newModelQueryRewriter(generator),
		reranker: retrieval.ChainReranker{
			Primary:  retrieval.NewServiceReranker(cfg.RerankerURL, 8*time.Second, 50, 2400),
			Fallback: retrieval.FallbackReranker{},
		},
		validator:           newGroundingValidator(generator),
		topK:                topK,
		denseRecallTopK:     denseRecallTopK,
		lexicalRecallTopK:   lexicalRecallTopK,
		denseWeight:         denseWeight,
		lexicalWeight:       lexicalWeight,
		fusionTopK:          fusionTopK,
		historyLimit:        historyLimit,
		rerankTopN:          rerankTopN,
		retrievalMode:       retrievalMode,
		contextBudget:       contextBudget,
		minEvidenceCount:    minEvidenceCount,
		queryRewriteEnabled: queryRewriteEnabled,
		multiQueryEnabled:   multiQueryEnabled,
		abstainEnabled:      abstainEnabled,
	}, nil
}

// buildGenerator constructs the TextGenerator for the configured provider.
func buildGenerator(cfg Config) (ai.TextGenerator, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.GenerationProvider))
	if provider == "" {
		provider = "gemini"
	}
	switch provider {
	case "gemini":
		client, err := ai.NewGeminiClient(cfg.GenerationAPIKey)
		if err != nil {
			return nil, err
		}
		return ai.NewGeminiGenerator(client, cfg.GenerationModel), nil
	case "ollama":
		client := ai.NewOllamaClient(cfg.GenerationBaseURL)
		return ai.NewOllamaGenerator(client, cfg.GenerationModel), nil
	case "openai-compat":
		if cfg.GenerationBaseURL == "" {
			return nil, fmt.Errorf("generationBaseURL required for openai-compat provider")
		}
		return ai.NewOpenAICompatGenerator(
			cfg.GenerationBaseURL,
			cfg.GenerationAPIKey,
			cfg.GenerationModel,
			ai.OpenAICompatConfig{EnableThinking: cfg.GenerationEnableThinking},
		), nil
	default:
		return nil, fmt.Errorf("unknown generation provider: %s", provider)
	}
}

// AskQuestion performs an evidence-grounded question/answer flow bound to a book and conversation.
func (a *App) AskQuestion(user domain.User, book domain.Book, question string, conversationID string, idempotencyKey string, includeDebug bool) (domain.Answer, bool, error) {
	return a.askQuestion(context.Background(), user, book, question, conversationID, idempotencyKey, includeDebug, nil)
}

// AskQuestionStream performs the same question/answer flow as AskQuestion but
// emits model output incrementally when the configured generator supports it.
func (a *App) AskQuestionStream(
	ctx context.Context,
	user domain.User,
	book domain.Book,
	question string,
	conversationID string,
	idempotencyKey string,
	includeDebug bool,
	onChunk func(string) error,
) (domain.Answer, bool, error) {
	return a.askQuestion(ctx, user, book, question, conversationID, idempotencyKey, includeDebug, onChunk)
}

func (a *App) askQuestion(
	ctx context.Context,
	user domain.User,
	book domain.Book,
	question string,
	conversationID string,
	idempotencyKey string,
	includeDebug bool,
	onChunk func(string) error,
) (domain.Answer, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if book.OwnerID != user.ID && user.Role != domain.RoleAdmin {
		return domain.Answer{}, false, fmt.Errorf("forbidden")
	}
	if book.Status != domain.StatusReady {
		return domain.Answer{}, false, ErrBookNotReady
	}
	if strings.TrimSpace(question) == "" {
		return domain.Answer{}, false, fmt.Errorf("question required")
	}
	record, replayedAnswer, replayed, err := a.beginChatIdempotency(user.ID, book.ID, conversationID, question, idempotencyKey)
	if err != nil {
		return domain.Answer{}, false, err
	}
	if replayed {
		return replayedAnswer, true, nil
	}
	conversation, createConversation, err := a.ensureConversation(user, book, question, conversationID)
	if err != nil {
		return domain.Answer{}, false, err
	}
	var history []domain.Message
	if a.historyLimit > 0 {
		historyLimit := a.historyLimit * 2
		if historyLimit < a.historyLimit {
			historyLimit = a.historyLimit
		}
		history, err = a.store.ListConversationMessages(conversation.ID, historyLimit)
		if err != nil {
			return domain.Answer{}, false, fmt.Errorf("load history: %w", err)
		}
	}
	historyText := buildHistory(history)
	decision := decideQueryRoute(question, history)

	var (
		answerText string
		abstained  bool
		citations  []domain.Source
		debugInfo  *domain.RetrievalDebug
	)
	switch decision.Route {
	case queryRouteHistoryOnly:
		answerText, citations, abstained, err = a.answerFromHistoryWithChunk(ctx, book, question, historyText, history, onChunk)
		if err != nil {
			return domain.Answer{}, false, err
		}
	case queryRouteDocumentOverview:
		answerText, citations, abstained, err = a.answerDocumentOverview(ctx, book, question, onChunk)
		if err != nil {
			return domain.Answer{}, false, err
		}
	case queryRouteOutOfScopeReject:
		if a.abstainEnabled {
			answerText = outOfScopeAbstainAnswer
			abstained = true
			break
		}
		fallthrough
	default:
		retrieved, routeDebug, routeErr := a.retrieveEvidence(ctx, book, question)
		if routeErr != nil {
			return domain.Answer{}, false, routeErr
		}
		debugInfo = routeDebug
		contextText, routeCitations := buildContext(retrieved)
		citations = routeCitations
		abstained = a.abstainEnabled && len(retrieved) < a.minEvidenceCount
		answerText = defaultAbstainAnswer
		if !abstained {
			var userPrompt string
			promptRequirement := "要求：只基于证据回答；引用相关编号；证据不足则明确拒答。"
			systemPrompt := "你是一个严格基于证据回答的读书助手。不要使用证据外知识。每个结论都必须可由提供证据支持。"
			if !a.abstainEnabled {
				promptRequirement = "要求：优先基于证据回答；引用相关编号；证据不足时可以给出谨慎的最佳努力回答，并明确说明不确定性，但不要编造引用。"
				systemPrompt = "你是一个优先基于证据回答的读书助手。可以在证据不足时给出谨慎的最佳努力回答，但必须明确不确定性，且不要虚构引用或把证据外信息说成确定事实。"
			}
			if historyText != "" {
				userPrompt = fmt.Sprintf("书名：%s\n对话历史：\n%s\n\n当前问题：%s\n\n证据：\n%s\n\n%s", book.Title, historyText, question, contextText, promptRequirement)
			} else {
				userPrompt = fmt.Sprintf("书名：%s\n问题：%s\n\n证据：\n%s\n\n%s", book.Title, question, contextText, promptRequirement)
			}
			response, genErr := a.generateAnswerText(ctx, systemPrompt, userPrompt, onChunk)
			if genErr == nil {
				answerText = strings.TrimSpace(response)
			} else if onChunk != nil {
				return domain.Answer{}, false, genErr
			}
			if strings.TrimSpace(answerText) == "" {
				abstained = true
				answerText = defaultAbstainAnswer
			}
			if !abstained && a.abstainEnabled && !a.validator.Validate(question, answerText, citations) {
				abstained = true
				answerText = defaultAbstainAnswer
				citations = nil
			}
		}
	}
	answer := domain.Answer{
		Conversation: conversation,
		Question:     question,
		Answer:       answerText,
		Citations:    citations,
		Abstained:    abstained,
		CreatedAt:    time.Now().UTC(),
	}
	if includeDebug {
		answer.RetrievalDebug = debugInfo
	}
	userMessageTime := time.Now().UTC()
	userMessage := domain.Message{
		ID:             util.NewID(),
		ConversationID: conversation.ID,
		UserID:         user.ID,
		BookID:         book.ID,
		Role:           "user",
		Content:        question,
		CreatedAt:      userMessageTime,
	}
	assistantMessageTime := time.Now().UTC()
	assistantMessage := domain.Message{
		ID:             util.NewID(),
		ConversationID: conversation.ID,
		UserID:         user.ID,
		BookID:         book.ID,
		Role:           "assistant",
		Content:        answer.Answer,
		Sources:        answer.Citations,
		Abstained:      abstained,
		CreatedAt:      assistantMessageTime,
	}
	answer.Conversation.LastMessageAt = &assistantMessageTime
	answer.Conversation.UpdatedAt = assistantMessageTime
	var completedRecord *domain.IdempotencyRecord
	if strings.TrimSpace(record.ID) != "" {
		record.State = domain.IdempotencyStateCompleted
		record.ResourceType = "conversation"
		record.ResourceID = conversation.ID
		record.StatusCode = 200
		record.UpdatedAt = time.Now().UTC()
		responseJSON, err := marshalAnswerResponse(answer)
		if err != nil {
			return domain.Answer{}, false, fmt.Errorf("marshal chat response: %w", err)
		}
		record.ResponseJSON = responseJSON
		completedRecord = &record
	}
	if err := a.store.SaveConversationExchange(conversation, createConversation, userMessage, assistantMessage, completedRecord); err != nil {
		return domain.Answer{}, false, fmt.Errorf("save conversation exchange: %w", err)
	}
	return answer, false, nil
}

func (a *App) answerFromHistory(ctx context.Context, book domain.Book, question string, historyText string, history []domain.Message) (string, []domain.Source, bool) {
	answer, citations, abstained, err := a.answerFromHistoryWithChunk(ctx, book, question, historyText, history, nil)
	if err != nil {
		return defaultAbstainAnswer, nil, true
	}
	return answer, citations, abstained
}

func (a *App) answerFromHistoryWithChunk(
	ctx context.Context,
	book domain.Book,
	question string,
	historyText string,
	history []domain.Message,
	onChunk func(string) error,
) (string, []domain.Source, bool, error) {
	if strings.TrimSpace(historyText) == "" {
		return defaultAbstainAnswer, nil, true, nil
	}
	requirement := "要求：只基于当前对话历史继续回答，不要引入对话外知识；如果历史不足以支撑回答，就明确说明证据不足。"
	systemPrompt := "你是一个延续当前会话上下文的读书助手。只使用给定对话历史，不要虚构新的事实。"
	if !a.abstainEnabled {
		requirement = "要求：只基于当前对话历史继续回答，不要引入对话外知识；如果历史不足，也可以给出谨慎的最佳努力回答，并明确说明不确定性。"
		systemPrompt = "你是一个延续当前会话上下文的读书助手。只使用给定对话历史，不要虚构新的事实；在信息不足时可以给出谨慎回答，但要明确不确定性。"
	}
	userPrompt := fmt.Sprintf("书名：%s\n对话历史：\n%s\n\n当前问题：%s\n\n%s", book.Title, historyText, question, requirement)
	answerText := defaultAbstainAnswer
	response, err := a.generateAnswerText(ctx, systemPrompt, userPrompt, onChunk)
	if err == nil {
		answerText = strings.TrimSpace(response)
	} else if onChunk != nil {
		return "", nil, false, err
	}
	if strings.TrimSpace(answerText) == "" {
		return defaultAbstainAnswer, nil, true, nil
	}
	if a.abstainEnabled && (strings.Contains(answerText, "证据不足") || strings.Contains(strings.ToLower(answerText), "insufficient")) {
		return defaultAbstainAnswer, nil, true, nil
	}
	return answerText, latestAssistantSources(history), false, nil
}

func (a *App) generateAnswerText(ctx context.Context, systemPrompt string, userPrompt string, onChunk func(string) error) (string, error) {
	if onChunk != nil {
		if streamer, ok := a.generator.(ai.StreamingTextGenerator); ok {
			return streamer.GenerateTextStream(ctx, systemPrompt, userPrompt, onChunk)
		}
	}

	response, err := a.generator.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	response = strings.TrimSpace(response)
	if onChunk != nil && response != "" {
		if err := onChunk(response); err != nil {
			return response, err
		}
	}
	return response, nil
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

// GetConversationBookID returns the bookId stored on an existing conversation.
// This allows the server layer to resolve the correct book without requiring the
// frontend to track which book each conversation belongs to.
func (a *App) GetConversationBookID(user domain.User, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", fmt.Errorf("conversation id required")
	}
	conversation, ok, err := a.store.GetConversation(conversationID)
	if err != nil {
		return "", fmt.Errorf("load conversation: %w", err)
	}
	if !ok {
		return "", ErrConversationNotFound
	}
	if conversation.UserID != user.ID && user.Role != domain.RoleAdmin {
		return "", ErrConversationForbidden
	}
	return conversation.BookID, nil
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

func (a *App) ensureConversation(user domain.User, book domain.Book, question string, conversationID string) (domain.Conversation, bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		conversation, ok, err := a.store.GetConversation(conversationID)
		if err != nil {
			return domain.Conversation{}, false, fmt.Errorf("load conversation: %w", err)
		}
		if !ok {
			return domain.Conversation{}, false, ErrConversationNotFound
		}
		if conversation.UserID != user.ID && user.Role != domain.RoleAdmin {
			return domain.Conversation{}, false, ErrConversationForbidden
		}
		return conversation, false, nil
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
	return conversation, true, nil
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

func buildContext(hits []retrieval.StageHit) (string, []domain.Source) {
	var sb strings.Builder
	sources := make([]domain.Source, 0, len(hits))
	for i, hit := range hits {
		chunk := hit.Chunk
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
			Label:     label,
			Location:  location,
			Snippet:   snippet,
			ChunkID:   chunk.ID,
			SourceRef: strings.TrimSpace(chunk.Metadata["source_ref"]),
			Score:     hit.Score,
			Language:  strings.TrimSpace(chunk.Metadata["language"]),
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
	if section := strings.TrimSpace(meta["section_path"]); section != "" {
		return section
	}
	if section := strings.TrimSpace(meta["section"]); section != "" {
		return section
	}
	if idx := strings.TrimSpace(meta["chunk_index"]); idx != "" {
		return "chunk " + idx
	}
	if idx := strings.TrimSpace(meta["chunk"]); idx != "" {
		return "chunk " + idx
	}
	return ""
}
