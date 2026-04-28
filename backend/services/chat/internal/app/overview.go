package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"onebookai/pkg/domain"
)

func (a *App) answerDocumentOverview(ctx context.Context, book domain.Book, question string, onChunk func(string) error) (string, []domain.Source, bool, error) {
	chunks, err := a.store.ListChunksByBook(book.ID)
	if err != nil {
		return "", nil, false, fmt.Errorf("load document overview chunks: %w", err)
	}
	overviewChunks := selectOverviewChunks(chunks, 4)
	citations := buildOverviewSources(book, overviewChunks)
	if len(citations) == 0 && strings.TrimSpace(book.DocumentSummary) == "" && strings.TrimSpace(book.OriginalFilename) == "" && strings.TrimSpace(book.Title) == "" {
		return defaultAbstainAnswer, nil, true, nil
	}
	if onChunk != nil {
		answer := fallbackDocumentOverviewAnswer(book, overviewChunks)
		if err := onChunk(answer); err != nil {
			return "", nil, false, err
		}
		return answer, citations, false, nil
	}

	contextText := buildDocumentOverviewContext(book, overviewChunks)
	systemPrompt := "你是一个文档概览助手。只根据给定的文件信息和首页/开头内容判断文档是什么，并用简洁中文回答。"
	userPrompt := fmt.Sprintf("用户问题：%s\n\n文档信息：\n%s\n\n要求：回答这份文档是什么、核心用途是什么；如果能判断文档类型，先说类型；引用证据编号。", question, contextText)
	answer, genErr := a.generateAnswerText(ctx, systemPrompt, userPrompt, onChunk)
	answer = strings.TrimSpace(answer)
	if genErr != nil {
		if onChunk != nil {
			return "", nil, false, genErr
		}
		answer = ""
	}
	if answer == "" || strings.Contains(answer, "证据不足") || strings.Contains(strings.ToLower(answer), "insufficient") {
		answer = fallbackDocumentOverviewAnswer(book, overviewChunks)
	}
	return answer, citations, false, nil
}

func buildDocumentOverviewContext(book domain.Book, chunks []domain.Chunk) string {
	var sb strings.Builder
	sb.WriteString("标题：")
	sb.WriteString(firstNonEmpty(book.Title, book.OriginalFilename))
	sb.WriteString("\n原始文件名：")
	sb.WriteString(book.OriginalFilename)
	if strings.TrimSpace(book.DocumentType) != "" {
		sb.WriteString("\n文档类型：")
		sb.WriteString(readableDocumentType(book.DocumentType))
	}
	if len(book.Keywords) > 0 {
		sb.WriteString("\n关键词：")
		sb.WriteString(strings.Join(book.Keywords, "、"))
	}
	if strings.TrimSpace(book.DocumentSummary) != "" {
		sb.WriteString("\n文档摘要：")
		sb.WriteString(book.DocumentSummary)
	}
	if strings.TrimSpace(book.FirstPageText) != "" {
		sb.WriteString("\n首页文本：")
		sb.WriteString(limitOverviewRunes(book.FirstPageText, 900))
	}
	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("\n[%d]", i+1))
		if location := chunkLocation(chunk.Metadata); location != "" {
			sb.WriteString(" (" + location + ")")
		}
		sb.WriteString(" ")
		sb.WriteString(limitOverviewRunes(chunk.Content, 700))
	}
	return sb.String()
}

func buildOverviewSources(book domain.Book, chunks []domain.Chunk) []domain.Source {
	sources := make([]domain.Source, 0, len(chunks)+1)
	if len(chunks) == 0 && strings.TrimSpace(book.DocumentSummary) != "" {
		sources = append(sources, domain.Source{
			Label:     "[1]",
			Location:  "document metadata",
			Snippet:   limitOverviewRunes(book.DocumentSummary, 240),
			SourceRef: "book:metadata",
			Language:  strings.TrimSpace(book.Language),
		})
	}
	for i, chunk := range chunks {
		snippet := strings.TrimSpace(chunk.Content)
		if len([]rune(snippet)) > 240 {
			snippet = limitOverviewRunes(snippet, 240) + "..."
		}
		sources = append(sources, domain.Source{
			Label:     fmt.Sprintf("[%d]", i+1),
			Location:  chunkLocation(chunk.Metadata),
			Snippet:   snippet,
			ChunkID:   chunk.ID,
			SourceRef: strings.TrimSpace(chunk.Metadata["source_ref"]),
			Language:  strings.TrimSpace(chunk.Metadata["language"]),
		})
	}
	return sources
}

func selectOverviewChunks(chunks []domain.Chunk, limit int) []domain.Chunk {
	if limit <= 0 || len(chunks) == 0 {
		return nil
	}
	candidates := make([]domain.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		if strings.TrimSpace(chunk.Metadata["page"]) == "1" || chunkIndex(chunk) <= 8 {
			candidates = append(candidates, chunk)
		}
	}
	if len(candidates) == 0 {
		candidates = chunks
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := chunkIndex(candidates[i]), chunkIndex(candidates[j])
		if left == right {
			return overviewTierRank(candidates[i]) < overviewTierRank(candidates[j])
		}
		return left < right
	})
	out := make([]domain.Chunk, 0, limit)
	seenFamilies := map[string]struct{}{}
	for _, chunk := range candidates {
		family := strings.TrimSpace(chunk.Metadata["chunk_family"])
		if family == "" {
			family = chunk.ID
		}
		if _, ok := seenFamilies[family]; ok && len(out) > 0 {
			continue
		}
		seenFamilies[family] = struct{}{}
		out = append(out, chunk)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func fallbackDocumentOverviewAnswer(book domain.Book, chunks []domain.Chunk) string {
	docType := readableDocumentType(book.DocumentType)
	if docType == "" {
		docType = inferOverviewTypeFromText(book.OriginalFilename + "\n" + book.DocumentSummary + "\n" + firstOverviewChunkText(chunks))
	}
	subject := firstNonEmpty(book.Title, book.OriginalFilename, "这份文档")
	if docType == "" || docType == "文档" {
		return fmt.Sprintf("%s 是一份文档，主要内容可从文件名和首页内容判断。", subject)
	}
	summary := strings.TrimSpace(book.DocumentSummary)
	if summary == "" {
		summary = firstOverviewChunkText(chunks)
	}
	summary = limitOverviewRunes(strings.Join(strings.Fields(summary), " "), 160)
	citation := ""
	if len(chunks) > 0 || strings.TrimSpace(book.DocumentSummary) != "" {
		citation = "（依据[1]）"
	}
	if summary == "" {
		return fmt.Sprintf("%s 是一份%s。%s", subject, docType, citation)
	}
	return fmt.Sprintf("%s 是一份%s，主要内容是：%s%s", subject, docType, summary, citation)
}

func readableDocumentType(kind string) string {
	switch strings.TrimSpace(kind) {
	case "internship_certificate":
		return "实习证明"
	case "certificate":
		return "证明材料"
	case "resume":
		return "简历"
	case "contract":
		return "合同/协议"
	case "invoice":
		return "发票"
	case "report":
		return "报告"
	case "book":
		return "书籍/长文档"
	case "document":
		return "文档"
	default:
		return ""
	}
}

func inferOverviewTypeFromText(text string) string {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "实习证明"):
		return "实习证明"
	case strings.Contains(text, "证明") || strings.Contains(text, "兹证明"):
		return "证明材料"
	case strings.Contains(text, "简历") || strings.Contains(text, "resume"):
		return "简历"
	case strings.Contains(text, "合同") || strings.Contains(text, "协议"):
		return "合同/协议"
	case strings.Contains(text, "报告") || strings.Contains(text, "report"):
		return "报告"
	default:
		return "文档"
	}
}

func firstOverviewChunkText(chunks []domain.Chunk) string {
	for _, chunk := range chunks {
		if text := strings.TrimSpace(chunk.Content); text != "" {
			return text
		}
	}
	return ""
}

func chunkIndex(chunk domain.Chunk) int {
	raw := strings.TrimSpace(chunk.Metadata["chunk_index"])
	if raw == "" {
		return 1 << 30
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 1 << 30
	}
	return value
}

func overviewTierRank(chunk domain.Chunk) int {
	switch strings.TrimSpace(chunk.Metadata["retrieval_tier"]) {
	case "semantic":
		return 0
	case "lexical":
		return 1
	default:
		return 2
	}
}

func limitOverviewRunes(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
