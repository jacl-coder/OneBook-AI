package app

import (
	"context"
	"fmt"
	"strings"

	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
)

const (
	questionTypeOverview   = "overview"
	questionTypeSingleFact = "single_fact"
	questionTypeSummary    = "summary"
	questionTypeAnalysis   = "analysis"
	questionTypeFollowUp   = "follow_up"
)

func (a *App) buildQueryPlan(ctx context.Context, book domain.Book, question string, history []domain.Message) domain.QueryPlan {
	decision := decideQueryRoute(question, history)
	standalone := strings.TrimSpace(question)
	if decision.Route == queryRouteRAG {
		standalone = a.contextualizeRetrievalQuestion(ctx, book, question, history)
	}
	questionType := classifyQuestionType(decision.Route, question, standalone, history)
	requiredEvidence := a.requiredEvidenceCount(question, standalone)
	if questionType == questionTypeAnalysis && requiredEvidence < a.minEvidenceCount {
		requiredEvidence = a.minEvidenceCount
	}
	if questionType == questionTypeSummary && requiredEvidence < 2 {
		requiredEvidence = 2
	}
	if decision.Route == queryRouteDocumentOverview {
		requiredEvidence = 1
	}
	queries := []string{}
	if decision.Route == queryRouteRAG {
		queries = a.buildRetrievalQueries(ctx, standalone)
		queries = appendDocumentFactQueries(queries, book, standalone)
	}
	return domain.QueryPlan{
		Route:                 string(decision.Route),
		QuestionType:          questionType,
		OriginalQuestion:      strings.TrimSpace(question),
		StandaloneQuestion:    standalone,
		RetrievalQueries:      uniqueRetrievalQueries(queries),
		RequiredEvidenceCount: requiredEvidence,
		ReuseChunkIDs:         latestAssistantChunkIDs(history),
		NeedsRetrieval:        decision.Route == queryRouteRAG,
		NeedsHistory:          needsConversationAwareRetrieval(question, history) || decision.Route == queryRouteHistoryOnly,
	}
}

func classifyQuestionType(route queryRoute, question string, standalone string, history []domain.Message) string {
	if route == queryRouteDocumentOverview {
		return questionTypeOverview
	}
	if route == queryRouteHistoryOnly {
		return questionTypeFollowUp
	}
	if route == queryRouteOutOfScopeReject {
		return "out_of_scope"
	}
	normalized := normalizeRouterText(question + " " + standalone)
	if hasRecentAssistantReply(history) && needsConversationAwareRetrieval(question, history) {
		if isSingleFactQuestion(question) || isSingleFactQuestion(standalone) {
			return questionTypeSingleFact
		}
		return questionTypeFollowUp
	}
	if isSingleFactQuestion(question) || isSingleFactQuestion(standalone) {
		return questionTypeSingleFact
	}
	for _, token := range []string{"总结", "概括", "主要内容", "讲什么", "简介"} {
		if strings.Contains(normalized, normalizeRouterText(token)) {
			return questionTypeSummary
		}
	}
	for _, token := range []string{"为什么", "原因", "分析", "评价", "比较", "区别", "影响", "意义"} {
		if strings.Contains(normalized, normalizeRouterText(token)) {
			return questionTypeAnalysis
		}
	}
	return "rag"
}

func appendDocumentFactQueries(queries []string, book domain.Book, question string) []string {
	wantKeys := factKeysForQuestion(question)
	if len(wantKeys) == 0 || len(book.DocumentFacts) == 0 {
		return queries
	}
	for _, fact := range book.DocumentFacts {
		if !containsString(wantKeys, strings.TrimSpace(fact.Key)) {
			continue
		}
		queries = append(queries, strings.TrimSpace(fact.Label+" "+fact.Value))
	}
	return queries
}

func factKeysForQuestion(question string) []string {
	normalized := normalizeRouterText(question)
	keys := []string{}
	add := func(items ...string) { keys = append(keys, items...) }
	if strings.Contains(normalized, "学生") || strings.Contains(normalized, "姓名") || strings.Contains(normalized, "谁") {
		add("student_name")
	}
	if strings.Contains(normalized, "实习时间") || strings.Contains(normalized, "什么时候") || strings.Contains(normalized, "何时") || strings.Contains(normalized, "日期") || strings.Contains(normalized, "时间") {
		add("internship_start", "internship_end", "proof_date")
	}
	if strings.Contains(normalized, "部门") {
		add("department")
	}
	if strings.Contains(normalized, "岗位") {
		add("position")
	}
	if strings.Contains(normalized, "学校") {
		add("school")
	}
	return uniqueStrings(keys)
}

func (a *App) selectEvidence(ctx context.Context, book domain.Book, plan domain.QueryPlan, retrieved []retrieval.StageHit, history []domain.Message) ([]retrieval.StageHit, []domain.Evidence, error) {
	candidates := make([]retrieval.StageHit, 0, len(retrieved)+len(plan.ReuseChunkIDs)+4)
	if plan.QuestionType == questionTypeSummary {
		if profileHit, ok := documentProfileHit(book); ok {
			candidates = append(candidates, profileHit)
		}
	}
	candidates = append(candidates, retrieved...)
	if len(plan.ReuseChunkIDs) > 0 {
		chunks, err := a.store.GetChunksByIDs(plan.ReuseChunkIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, chunk := range chunks {
			candidates = append(candidates, retrieval.StageHit{
				ChunkID:  chunk.ID,
				BookID:   chunk.BookID,
				Score:    1.05,
				Stage:    "previous_citation",
				Content:  chunk.Content,
				Metadata: chunk.Metadata,
				Chunk:    chunk,
			})
		}
	}
	if plan.QuestionType == questionTypeSingleFact {
		factHits, err := a.factBackedHits(ctx, book, plan)
		if err != nil {
			return nil, nil, err
		}
		candidates = append(factHits, candidates...)
	}
	selected := selectUniqueEvidenceHits(candidates, a.topK)
	evidence := make([]domain.Evidence, 0, len(selected))
	for _, hit := range selected {
		evidence = append(evidence, hitToEvidence(hit))
	}
	_ = history
	return selected, evidence, nil
}

func documentProfileHit(book domain.Book) (retrieval.StageHit, bool) {
	summary := strings.TrimSpace(book.DocumentSummary)
	if summary == "" {
		return retrieval.StageHit{}, false
	}
	chunk := domain.Chunk{
		ID:      strings.TrimSpace(book.ID) + ":document_profile",
		BookID:  book.ID,
		Content: summary,
		Metadata: map[string]string{
			"source_ref": "book:metadata",
			"page":       "",
			"language":   strings.TrimSpace(book.Language),
		},
	}
	return retrieval.StageHit{
		ChunkID:  chunk.ID,
		BookID:   book.ID,
		Score:    1.1,
		Stage:    "document_profile",
		Content:  summary,
		Metadata: chunk.Metadata,
		Chunk:    chunk,
	}, true
}

func (a *App) factBackedHits(ctx context.Context, book domain.Book, plan domain.QueryPlan) ([]retrieval.StageHit, error) {
	keys := factKeysForQuestion(plan.StandaloneQuestion)
	if len(keys) == 0 {
		keys = factKeysForQuestion(plan.OriginalQuestion)
	}
	if len(keys) == 0 || len(book.DocumentFacts) == 0 {
		return nil, nil
	}
	chunks, err := a.store.ListChunksByBook(book.ID)
	if err != nil {
		return nil, err
	}
	out := []retrieval.StageHit{}
	for _, fact := range book.DocumentFacts {
		if !containsString(keys, strings.TrimSpace(fact.Key)) || strings.TrimSpace(fact.Value) == "" {
			continue
		}
		for _, chunk := range chunks {
			sourceRef := ""
			page := ""
			if chunk.Metadata != nil {
				sourceRef = strings.TrimSpace(chunk.Metadata["source_ref"])
				page = strings.TrimSpace(chunk.Metadata["page"])
			}
			valueMatches := strings.Contains(chunk.Content, fact.Value)
			sourceMatches := strings.TrimSpace(fact.SourceRef) != "" && strings.TrimSpace(fact.SourceRef) == sourceRef
			pageMatches := strings.TrimSpace(fact.Page) != "" && strings.TrimSpace(fact.Page) == page
			if !valueMatches && !sourceMatches && !pageMatches {
				continue
			}
			out = append(out, retrieval.StageHit{
				ChunkID:  chunk.ID,
				BookID:   chunk.BookID,
				Score:    1.2,
				Stage:    "structured_fact",
				Content:  chunk.Content,
				Metadata: chunk.Metadata,
				Chunk:    chunk,
			})
			break
		}
	}
	return out, nil
}

func selectUniqueEvidenceHits(hits []retrieval.StageHit, limit int) []retrieval.StageHit {
	if limit <= 0 {
		limit = 5
	}
	seen := map[string]struct{}{}
	out := make([]retrieval.StageHit, 0, limit)
	for _, hit := range hits {
		chunk := hit.Chunk
		id := strings.TrimSpace(chunk.ID)
		if id == "" {
			id = strings.TrimSpace(hit.ChunkID)
		}
		if id == "" {
			continue
		}
		key := id
		if family := strings.TrimSpace(chunk.Metadata["chunk_family"]); family != "" {
			key = family
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, hit)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func hitToEvidence(hit retrieval.StageHit) domain.Evidence {
	chunk := hit.Chunk
	page := ""
	if chunk.Metadata != nil {
		page = strings.TrimSpace(chunk.Metadata["page"])
	}
	evidenceType := "chunk"
	reason := "hybrid retrieval"
	switch hit.Stage {
	case "structured_fact":
		evidenceType = "structured_fact"
		reason = "matched structured document fact"
	case "document_profile":
		evidenceType = "document_profile"
		reason = "document profile summary"
	case "previous_citation":
		evidenceType = "previous_citation"
		reason = "reused previous cited evidence"
	case "rerank", "fusion":
		reason = "ranked as relevant evidence"
	}
	return domain.Evidence{
		ChunkID:      chunk.ID,
		Page:         page,
		Location:     chunkLocation(chunk.Metadata),
		Snippet:      truncateRunes(chunk.Content, 240),
		Score:        hit.Score,
		SourceReason: reason,
		EvidenceType: evidenceType,
	}
}

func buildContextWithEvidence(hits []retrieval.StageHit, evidence []domain.Evidence) (string, []domain.Source) {
	evidenceByChunk := map[string]domain.Evidence{}
	for _, item := range evidence {
		evidenceByChunk[item.ChunkID] = item
	}
	var sb strings.Builder
	sources := make([]domain.Source, 0, len(hits))
	for i, hit := range hits {
		chunk := hit.Chunk
		label := fmt.Sprintf("[%d]", i+1)
		location := chunkLocation(chunk.Metadata)
		snippet := truncateRunes(chunk.Content, 240)
		ev := evidenceByChunk[chunk.ID]
		sb.WriteString(label)
		if location != "" {
			sb.WriteString(" (" + location + ")")
		}
		if ev.SourceReason != "" {
			sb.WriteString(" {" + ev.SourceReason + "}")
		}
		sb.WriteString(" ")
		sb.WriteString(chunk.Content)
		sb.WriteString("\n\n")
		sources = append(sources, domain.Source{
			Label:        label,
			Location:     location,
			Snippet:      snippet,
			ChunkID:      chunk.ID,
			SourceRef:    strings.TrimSpace(chunk.Metadata["source_ref"]),
			Score:        hit.Score,
			Language:     strings.TrimSpace(chunk.Metadata["language"]),
			SourceReason: ev.SourceReason,
			EvidenceType: ev.EvidenceType,
		})
	}
	return sb.String(), sources
}

func validateEvidenceSelection(plan domain.QueryPlan, hits []retrieval.StageHit, citations []domain.Source, abstainEnabled bool) domain.ValidationResult {
	if !abstainEnabled {
		return domain.ValidationResult{Passed: true, Reason: "abstain disabled"}
	}
	if len(hits) < plan.RequiredEvidenceCount || len(citations) < plan.RequiredEvidenceCount {
		return domain.ValidationResult{
			Passed: false,
			Reason: fmt.Sprintf("当前问题类型 %s 至少需要 %d 条可引用证据，但只找到 %d 条", plan.QuestionType, plan.RequiredEvidenceCount, len(citations)),
		}
	}
	return domain.ValidationResult{Passed: true, Reason: "已选证据满足回答策略"}
}

func abstainAnswerForPlan(plan domain.QueryPlan, reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "未找到足够可引用证据"
	}
	return defaultAbstainAnswer + "\n\n缺少证据原因：" + reason
}

func answerRequirementForPlan(plan domain.QueryPlan) string {
	switch plan.QuestionType {
	case questionTypeSingleFact:
		return "要求：回答单点事实；如果证据中能直接定位答案，用一句话回答并引用编号；不要展开无关内容。"
	case questionTypeSummary:
		return "要求：概括整份文档；必须覆盖核心对象、用途和关键时间/事实；引用多个相关编号；不要只总结一个局部片段。"
	case questionTypeAnalysis:
		return "要求：只基于多条证据做分析；证据不足则明确拒答并说明缺少哪类依据。"
	default:
		return "要求：只基于证据回答；引用相关编号；证据不足则明确拒答。"
	}
}

func buildStructuredAnswerPrompt(book domain.Book, plan domain.QueryPlan, historyText string, contextText string, requirement string) string {
	var sb strings.Builder
	sb.WriteString("文档标题：")
	sb.WriteString(firstNonEmpty(book.Title, book.OriginalFilename))
	if strings.TrimSpace(book.DocumentType) != "" {
		sb.WriteString("\n文档类型：")
		sb.WriteString(readableDocumentType(book.DocumentType))
	}
	if strings.TrimSpace(book.DocumentSummary) != "" {
		sb.WriteString("\n文档摘要：")
		sb.WriteString(book.DocumentSummary)
	}
	sb.WriteString("\n\n原始问题：")
	sb.WriteString(plan.OriginalQuestion)
	sb.WriteString("\n独立检索问题：")
	sb.WriteString(plan.StandaloneQuestion)
	sb.WriteString("\n问题类型：")
	sb.WriteString(plan.QuestionType)
	if historyText != "" {
		sb.WriteString("\n\n最近对话：\n")
		sb.WriteString(historyText)
	}
	sb.WriteString("\n\n选中证据：\n")
	sb.WriteString(contextText)
	sb.WriteString("\n")
	sb.WriteString(requirement)
	return sb.String()
}

func enrichRetrievalDebug(debug *domain.RetrievalDebug, trace domain.AnswerTrace) {
	if debug == nil {
		return
	}
	plan := trace.QueryPlan
	debug.Route = plan.Route
	debug.QuestionType = plan.QuestionType
	debug.StandaloneQuestion = plan.StandaloneQuestion
	debug.RequiredEvidenceCount = plan.RequiredEvidenceCount
	debug.SelectedEvidence = trace.SelectedEvidence
	debug.SelectedChunkIDs = selectedChunkIDs(trace.SelectedEvidence)
	debug.ValidationReason = trace.ValidationResult.Reason
	debug.QueryPlan = &plan
	if len(plan.RetrievalQueries) > 0 {
		debug.Queries = plan.RetrievalQueries
	}
}

func sourcesToEvidence(sources []domain.Source, evidenceType string) []domain.Evidence {
	out := make([]domain.Evidence, 0, len(sources))
	for _, source := range sources {
		out = append(out, domain.Evidence{
			ChunkID:      source.ChunkID,
			Location:     source.Location,
			Snippet:      source.Snippet,
			Score:        source.Score,
			SourceReason: firstNonEmpty(source.SourceReason, evidenceType),
			EvidenceType: firstNonEmpty(source.EvidenceType, evidenceType),
		})
	}
	return out
}

func latestAssistantChunkIDs(history []domain.Message) []string {
	sources := latestAssistantSources(history)
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.ChunkID) != "" {
			out = append(out, strings.TrimSpace(source.ChunkID))
		}
	}
	return uniqueStrings(out)
}

func selectedChunkIDs(evidence []domain.Evidence) []string {
	out := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if strings.TrimSpace(item.ChunkID) != "" {
			out = append(out, strings.TrimSpace(item.ChunkID))
		}
	}
	return uniqueStrings(out)
}

func validationReason(abstained bool, success string) string {
	if abstained {
		return "answer abstained"
	}
	return success
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}
