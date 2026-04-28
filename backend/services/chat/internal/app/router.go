package app

import (
	"strings"

	"onebookai/pkg/domain"
)

const outOfScopeAbstainAnswer = "当前问题超出已上传内容和当前会话范围，无法基于书内证据给出可靠回答。"

type queryRoute string

const (
	queryRouteRAG              queryRoute = "rag"
	queryRouteDocumentOverview queryRoute = "document_overview"
	queryRouteHistoryOnly      queryRoute = "history_only"
	queryRouteOutOfScopeReject queryRoute = "out_of_scope_reject"
)

type queryRouteDecision struct {
	Route  queryRoute
	Reason string
}

func decideQueryRoute(question string, history []domain.Message) queryRouteDecision {
	normalized := normalizeRouterText(question)
	if normalized == "" {
		return queryRouteDecision{Route: queryRouteRAG, Reason: "empty"}
	}
	if hasRecentAssistantReply(history) && isHistoryOnlyFollowUp(normalized) {
		return queryRouteDecision{Route: queryRouteHistoryOnly, Reason: "follow_up"}
	}
	if isDocumentOverviewQuestion(normalized) {
		return queryRouteDecision{Route: queryRouteDocumentOverview, Reason: "document_overview"}
	}
	if isClearlyOutOfScopeRealtime(normalized) && !hasDocumentAnchor(normalized) && !looksLikeConversationReference(normalized) {
		return queryRouteDecision{Route: queryRouteOutOfScopeReject, Reason: "out_of_scope_realtime"}
	}
	return queryRouteDecision{Route: queryRouteRAG, Reason: "default"}
}

func isDocumentOverviewQuestion(question string) bool {
	if question == "" {
		return false
	}
	exact := []string{
		"这是什么",
		"这个是什么",
		"这是啥",
		"这是干什么的",
		"这个文件是什么",
		"这份文件是什么",
		"这是什么文件",
		"这是什么资料",
		"这份资料是什么",
		"这个pdf是什么",
		"这本书讲什么",
		"这本书是什么",
		"这份文档讲什么",
		"这个文档讲什么",
		"总结一下",
		"概括一下",
		"简介一下",
	}
	for _, item := range exact {
		if question == normalizeRouterText(item) {
			return true
		}
	}
	if len([]rune(question)) > 40 {
		return false
	}
	patterns := []string{
		"这是什么",
		"这个文件",
		"这份文件",
		"这个文档",
		"这份文档",
		"这份资料",
		"总结",
		"概括",
		"简介",
		"讲什么",
		"主要内容",
	}
	for _, pattern := range patterns {
		if strings.Contains(question, normalizeRouterText(pattern)) {
			return true
		}
	}
	return false
}

func normalizeRouterText(text string) string {
	replacer := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		"，", ",",
		"。", ".",
		"？", "?",
		"！", "!",
		"：", ":",
		"（", "(",
		"）", ")",
	)
	return strings.ToLower(strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(text))), " "))
}

func hasRecentAssistantReply(history []domain.Message) bool {
	for i := len(history) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(history[i].Role), "assistant") && strings.TrimSpace(history[i].Content) != "" {
			return true
		}
	}
	return false
}

func isHistoryOnlyFollowUp(question string) bool {
	if question == "" {
		return false
	}
	explicit := []string{
		"再详细一点",
		"再详细一些",
		"展开讲讲",
		"展开一点",
		"继续",
		"继续说",
		"接着说",
		"换个说法",
		"换一种说法",
		"简单解释一下",
		"通俗解释一下",
		"翻译一下",
		"总结一下",
		"举个例子",
		"这是什么意思",
		"上一段是什么意思",
		"上一条是什么意思",
		"刚才那段是什么意思",
	}
	for _, item := range explicit {
		if question == normalizeRouterText(item) {
			return true
		}
	}
	prefixes := []string{
		"再详细",
		"展开",
		"继续",
		"接着",
		"换个",
		"换一种",
		"翻译",
		"总结",
		"举个例子",
		"解释一下",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(question, normalizeRouterText(prefix)) {
			return true
		}
	}
	if len([]rune(question)) > 48 {
		return false
	}
	return looksLikeConversationReference(question)
}

func looksLikeConversationReference(question string) bool {
	refTokens := []string{
		"上面",
		"前面",
		"刚才",
		"这段",
		"上一段",
		"上一条",
		"上一个回答",
		"前一个回答",
		"这里",
		"这个说法",
		"这个结论",
	}
	for _, token := range refTokens {
		if strings.Contains(question, normalizeRouterText(token)) {
			return true
		}
	}
	return false
}

func hasDocumentAnchor(question string) bool {
	docTokens := []string{
		"这本书",
		"书里",
		"书中",
		"文中",
		"文里",
		"前文",
		"上文",
		"章节",
		"第",
		"页",
		"作者",
	}
	for _, token := range docTokens {
		if strings.Contains(question, normalizeRouterText(token)) {
			return true
		}
	}
	return false
}

func isClearlyOutOfScopeRealtime(question string) bool {
	realtimeTerms := []string{
		"天气",
		"温度",
		"股价",
		"汇率",
		"新闻",
		"热搜",
		"比分",
		"战绩",
		"比赛结果",
		"谁赢了",
		"油价",
		"航班",
		"路况",
	}
	timeTerms := []string{
		"今天",
		"今日",
		"现在",
		"实时",
		"最新",
		"刚刚",
		"最近",
		"目前",
		"明天",
		"昨天",
	}
	hasRealtimeTerm := false
	for _, token := range realtimeTerms {
		if strings.Contains(question, normalizeRouterText(token)) {
			hasRealtimeTerm = true
			break
		}
	}
	if !hasRealtimeTerm {
		return false
	}
	for _, token := range timeTerms {
		if strings.Contains(question, normalizeRouterText(token)) {
			return true
		}
	}
	return question == normalizeRouterText("北京的天气怎么样") ||
		question == normalizeRouterText("今天股价多少") ||
		question == normalizeRouterText("最新新闻是什么")
}

func latestAssistantSources(history []domain.Message) []domain.Source {
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if !strings.EqualFold(strings.TrimSpace(msg.Role), "assistant") || len(msg.Sources) == 0 {
			continue
		}
		out := make([]domain.Source, 0, len(msg.Sources))
		for _, source := range msg.Sources {
			out = append(out, source)
		}
		return out
	}
	return nil
}
