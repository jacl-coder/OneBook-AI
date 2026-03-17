package app

import (
	"testing"

	"onebookai/pkg/domain"
)

func TestDecideQueryRouteHistoryOnlyFollowUp(t *testing.T) {
	history := []domain.Message{
		{Role: "user", Content: "解释一下第三章的核心观点"},
		{Role: "assistant", Content: "第三章主要讲...", Sources: []domain.Source{{Label: "[1]"}}},
	}

	decision := decideQueryRoute("再详细一点", history)
	if decision.Route != queryRouteHistoryOnly {
		t.Fatalf("expected history_only route, got %s", decision.Route)
	}
}

func TestDecideQueryRouteOutOfScopeRealtime(t *testing.T) {
	decision := decideQueryRoute("北京今天天气怎么样", nil)
	if decision.Route != queryRouteOutOfScopeReject {
		t.Fatalf("expected out_of_scope_reject route, got %s", decision.Route)
	}
}

func TestDecideQueryRouteKeepsBookAnchoredQuestionInRAG(t *testing.T) {
	decision := decideQueryRoute("书里关于天气描写的作用是什么", nil)
	if decision.Route != queryRouteRAG {
		t.Fatalf("expected rag route, got %s", decision.Route)
	}
}

func TestLatestAssistantSourcesReturnsMostRecentAssistantSources(t *testing.T) {
	history := []domain.Message{
		{Role: "assistant", Content: "old", Sources: []domain.Source{{Label: "[1]"}}},
		{Role: "user", Content: "follow up"},
		{Role: "assistant", Content: "new", Sources: []domain.Source{{Label: "[2]"}}},
	}

	sources := latestAssistantSources(history)
	if len(sources) != 1 || sources[0].Label != "[2]" {
		t.Fatalf("expected latest assistant sources, got %#v", sources)
	}
}
