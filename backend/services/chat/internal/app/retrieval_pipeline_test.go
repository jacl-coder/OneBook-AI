package app

import (
	"context"
	"testing"

	"onebookai/pkg/ai"
	"onebookai/pkg/domain"
	"onebookai/pkg/retrieval"
)

type stubQueryRewriter struct {
	rewrites []string
	calls    int
}

func (s *stubQueryRewriter) Rewrite(_ context.Context, _ string, _ string) ([]string, error) {
	s.calls++
	return s.rewrites, nil
}

type stubGenerator struct {
	response string
}

func (s stubGenerator) GenerateText(_ context.Context, _, _ string) (string, error) {
	return s.response, nil
}

var _ ai.TextGenerator = stubGenerator{}

func TestBuildRetrievalQueriesSingleQueryWhenMultiQueryDisabled(t *testing.T) {
	a := &App{multiQueryEnabled: false}

	got := a.buildRetrievalQueries(context.Background(), "请问一下，如何重置密码？")
	want := []string{retrieval.NormalizeText("请问一下，如何重置密码？")}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("buildRetrievalQueries() = %#v, want %#v", got, want)
	}
}

func TestBuildRetrievalQueriesUsesRewriteWhenSingleQueryMode(t *testing.T) {
	rewriter := &stubQueryRewriter{rewrites: []string{"如何重置密码"}}
	a := &App{
		rewriter:            rewriter,
		queryRewriteEnabled: true,
		multiQueryEnabled:   false,
	}

	got := a.buildRetrievalQueries(context.Background(), "请问一下，如何重置密码？")
	want := []string{retrieval.NormalizeText("如何重置密码")}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("buildRetrievalQueries() = %#v, want %#v", got, want)
	}
	if rewriter.calls != 1 {
		t.Fatalf("rewriter calls = %d, want 1", rewriter.calls)
	}
}

func TestBuildRetrievalQueriesMergesVariantsAndRewritesWhenMultiQueryEnabled(t *testing.T) {
	rewriter := &stubQueryRewriter{rewrites: []string{"重置密码 方法", "如何重置密码"}}
	a := &App{
		rewriter:            rewriter,
		queryRewriteEnabled: true,
		multiQueryEnabled:   true,
	}

	got := a.buildRetrievalQueries(context.Background(), "请问一下，如何重置密码？")
	want := uniqueRetrievalQueries(append(
		retrieval.BuildQueryVariants("请问一下，如何重置密码？"),
		normalizeRetrievalQueries([]string{"重置密码 方法", "如何重置密码"})...,
	))
	if len(got) != len(want) {
		t.Fatalf("len(buildRetrievalQueries()) = %d, want %d, got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildRetrievalQueries()[%d] = %q, want %q (full=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestAnswerFromHistoryDoesNotForceAbstainWhenStrategyDisabled(t *testing.T) {
	a := &App{
		generator:      stubGenerator{response: "证据不足，但我倾向于这是作者在做铺垫。"},
		abstainEnabled: false,
	}

	answer, citations, abstained := a.answerFromHistory(context.Background(), domain.Book{Title: "book"}, "继续说", "已有历史", nil)
	if abstained {
		t.Fatalf("answerFromHistory() abstained = true, want false")
	}
	if answer == "" {
		t.Fatal("answerFromHistory() returned empty answer")
	}
	if citations != nil {
		t.Fatalf("answerFromHistory() citations = %#v, want nil", citations)
	}
}
