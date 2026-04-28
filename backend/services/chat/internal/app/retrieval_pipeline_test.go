package app

import (
	"context"
	"strings"
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

func TestContextualizeRetrievalQuestionUsesRecentHistoryFallback(t *testing.T) {
	a := &App{}
	book := domain.Book{Title: "赖新鹏实习证明", OriginalFilename: "赖新鹏实习证明.pdf"}
	history := []domain.Message{
		{Role: "user", Content: "这是什么"},
		{Role: "assistant", Content: "赖新鹏实习证明是一份实习证明，内容包括学生 赖新鹏 在工程研发部门实习。"},
	}

	got := a.contextualizeRetrievalQuestion(context.Background(), book, "学生是谁", history)
	for _, want := range []string{"学生是谁", "赖新鹏", "实习证明"} {
		if !strings.Contains(got, want) {
			t.Fatalf("contextualized question = %q, want it to contain %q", got, want)
		}
	}
}

func TestContextualizeRetrievalQuestionUsesModelRewrite(t *testing.T) {
	a := &App{generator: stubGenerator{response: "这份实习证明中的学生姓名是谁？"}}
	book := domain.Book{Title: "赖新鹏实习证明"}
	history := []domain.Message{
		{Role: "assistant", Content: "这是一份实习证明。"},
	}

	got := a.contextualizeRetrievalQuestion(context.Background(), book, "学生是谁", history)
	want := "这份实习证明中的学生姓名是谁？"
	if got != want {
		t.Fatalf("contextualized question = %q, want %q", got, want)
	}
}

func TestRequiredEvidenceCountAllowsSingleFactQuestion(t *testing.T) {
	a := &App{minEvidenceCount: 2}

	got := a.requiredEvidenceCount("学生 是谁", "这份实习证明中的学生姓名是谁？")
	if got != 1 {
		t.Fatalf("requiredEvidenceCount() = %d, want 1", got)
	}
}

func TestRequiredEvidenceCountKeepsConfiguredThresholdForComplexQuestion(t *testing.T) {
	a := &App{minEvidenceCount: 2}

	got := a.requiredEvidenceCount("总结一下主要内容和原因", "总结一下主要内容和原因")
	if got != 2 {
		t.Fatalf("requiredEvidenceCount() = %d, want 2", got)
	}
}

func TestBuildQueryPlanRewritesFollowUpSingleFact(t *testing.T) {
	a := &App{minEvidenceCount: 2}
	book := domain.Book{
		Title:            "赖新鹏实习证明",
		OriginalFilename: "赖新鹏实习证明.pdf",
		DocumentFacts: []domain.DocumentFact{
			{Key: "student_name", Label: "学生姓名", Value: "赖新鹏"},
		},
	}
	history := []domain.Message{
		{Role: "user", Content: "这是什么,主要写的什么"},
		{Role: "assistant", Content: "这是一份实习证明，证明学生 赖新鹏 的实习情况。", Sources: []domain.Source{{ChunkID: "chunk-1", Label: "[1]"}}},
	}

	plan := a.buildQueryPlan(context.Background(), book, "学生是谁", history)
	if plan.QuestionType != questionTypeSingleFact {
		t.Fatalf("questionType = %q, want %q", plan.QuestionType, questionTypeSingleFact)
	}
	if plan.RequiredEvidenceCount != 1 {
		t.Fatalf("requiredEvidenceCount = %d, want 1", plan.RequiredEvidenceCount)
	}
	for _, want := range []string{"学生是谁", "赖新鹏", "实习证明"} {
		if !strings.Contains(plan.StandaloneQuestion, want) {
			t.Fatalf("standaloneQuestion = %q, want it to contain %q", plan.StandaloneQuestion, want)
		}
	}
	if !containsString(plan.RetrievalQueries, retrieval.NormalizeText("学生姓名 赖新鹏")) {
		t.Fatalf("retrievalQueries = %#v, want structured fact query", plan.RetrievalQueries)
	}
	if !containsString(plan.ReuseChunkIDs, "chunk-1") {
		t.Fatalf("reuseChunkIds = %#v, want previous citation chunk", plan.ReuseChunkIDs)
	}
}

func TestBuildQueryPlanAddsDateAndDepartmentFactQueries(t *testing.T) {
	a := &App{minEvidenceCount: 2}
	book := domain.Book{
		Title: "赖新鹏实习证明",
		DocumentFacts: []domain.DocumentFact{
			{Key: "internship_start", Label: "实习开始时间", Value: "2024-08-01"},
			{Key: "internship_end", Label: "实习结束时间", Value: "2024-12-30"},
			{Key: "department", Label: "实习部门", Value: "工程研发"},
		},
	}

	timePlan := a.buildQueryPlan(context.Background(), book, "实习时间是什么", nil)
	if !containsString(timePlan.RetrievalQueries, retrieval.NormalizeText("实习开始时间 2024-08-01")) ||
		!containsString(timePlan.RetrievalQueries, retrieval.NormalizeText("实习结束时间 2024-12-30")) {
		t.Fatalf("time retrievalQueries = %#v, want internship date fact queries", timePlan.RetrievalQueries)
	}

	departmentPlan := a.buildQueryPlan(context.Background(), book, "在哪个部门", nil)
	if !containsString(departmentPlan.RetrievalQueries, retrieval.NormalizeText("实习部门 工程研发")) {
		t.Fatalf("department retrievalQueries = %#v, want department fact query", departmentPlan.RetrievalQueries)
	}
}

func TestBuildQueryPlanRequiresMultipleEvidenceForSummary(t *testing.T) {
	a := &App{minEvidenceCount: 1}

	plan := a.buildQueryPlan(context.Background(), domain.Book{Title: "证明"}, "总结这份证明", nil)
	if plan.QuestionType != questionTypeSummary {
		t.Fatalf("questionType = %q, want %q", plan.QuestionType, questionTypeSummary)
	}
	if plan.RequiredEvidenceCount < 2 {
		t.Fatalf("requiredEvidenceCount = %d, want at least 2", plan.RequiredEvidenceCount)
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
