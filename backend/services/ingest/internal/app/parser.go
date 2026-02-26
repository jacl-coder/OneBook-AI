package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"
)

type chunkPayload struct {
	Content  string
	Metadata map[string]string
}

type pageExtraction struct {
	Page        int
	Text        string
	Method      string
	OCRAvgScore float64
}

type pageQuality struct {
	Runes         int
	NonEmptyLines int
	AlphaNumRatio float64
	Score         float64
}

func (a *App) parseAndChunk(filename, path string) ([]chunkPayload, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return a.parsePDF(path)
	case ".epub":
		return a.parseEPUB(path)
	default:
		return a.parseText(path)
	}
}

func (a *App) parsePDF(path string) ([]chunkPayload, error) {
	nativePages, nativeErr := a.parsePDFNativePages(path)
	if len(nativePages) == 0 && !a.ocrEnabled {
		return nil, fmt.Errorf("no text extracted from PDF; native=%v", nativeErr)
	}

	selectedPages := nativePages
	needsOCR := false
	if a.ocrEnabled {
		if len(nativePages) == 0 {
			needsOCR = true
		} else {
			for _, page := range nativePages {
				if isLowQualityPage(evaluatePageQuality(page.Text), a.pdfMinRunes, a.pdfMinScore) {
					needsOCR = true
					break
				}
			}
		}
	}
	if needsOCR {
		ocrPages, ocrErr := a.parsePDFWithPaddleOCR(path)
		if ocrErr != nil && len(nativePages) == 0 {
			return nil, fmt.Errorf("no text extracted from PDF; native=%v; paddleocr=%v", nativeErr, ocrErr)
		}
		if ocrErr == nil {
			selectedPages = a.mergePDFPages(nativePages, ocrPages)
		}
	}
	if len(selectedPages) == 0 {
		return nil, fmt.Errorf("no text extracted from PDF")
	}
	return a.buildPDFChunks(selectedPages), nil
}

func (a *App) parsePDFNativePages(path string) ([]pageExtraction, error) {
	pdftotextPages, pdftotextErr := a.parsePDFPagesWithPdftotext(path)
	if len(pdftotextPages) > 0 {
		return pdftotextPages, nil
	}
	goLibPages, goLibErr := a.parsePDFPagesWithGoLib(path)
	if len(goLibPages) == 0 {
		return nil, fmt.Errorf("pdftotext=%v; golib=%v", pdftotextErr, goLibErr)
	}
	return goLibPages, nil
}

// parsePDFPagesWithPdftotext uses the system pdftotext tool (poppler-utils).
func (a *App) parsePDFPagesWithPdftotext(path string) ([]pageExtraction, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return nil, fmt.Errorf("pdftotext not found: %w", err)
	}
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext failed: %w", err)
	}
	raw := strings.ReplaceAll(string(output), "\x00", " ")
	raw = strings.ToValidUTF8(raw, "")
	pages := strings.Split(raw, "\f")
	out := make([]pageExtraction, 0, len(pages))
	for pageIdx, pageText := range pages {
		pageText = normalizeTextPreserveNewlines(pageText)
		if pageText == "" {
			continue
		}
		out = append(out, pageExtraction{
			Page:   pageIdx + 1,
			Text:   pageText,
			Method: "pdftotext",
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no text extracted from PDF")
	}
	return out, nil
}

// parsePDFPagesWithGoLib uses the Go PDF library (fallback).
func (a *App) parsePDFPagesWithGoLib(path string) ([]pageExtraction, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()
	totalPages := reader.NumPage()
	out := make([]pageExtraction, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Skip problematic pages instead of failing entirely
			continue
		}
		text = normalizeTextPreserveNewlines(text)
		if text == "" {
			continue
		}
		out = append(out, pageExtraction{
			Page:   i,
			Text:   text,
			Method: "golib",
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no text extracted from PDF")
	}
	return out, nil
}

func (a *App) parsePDFWithPaddleOCR(path string) ([]pageExtraction, error) {
	if _, err := exec.LookPath(a.ocrCommand); err != nil {
		return nil, fmt.Errorf("paddleocr command not found: %w", err)
	}
	savePath, err := os.MkdirTemp("", "onebook-paddleocr-*")
	if err != nil {
		return nil, fmt.Errorf("create paddleocr temp dir: %w", err)
	}
	defer os.RemoveAll(savePath)

	ctx, cancel := context.WithTimeout(context.Background(), a.ocrTimeout)
	defer cancel()
	args := []string{
		"ocr",
		"-i", path,
		"--save_path", savePath,
		"--device", a.ocrDevice,
		"--use_doc_orientation_classify", "False",
		"--use_doc_unwarping", "False",
		"--use_textline_orientation", "False",
	}
	cmd := exec.CommandContext(ctx, a.ocrCommand, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run paddleocr failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	pages, err := readPaddleOCRPages(savePath)
	if err != nil {
		return nil, err
	}
	out := make([]pageExtraction, 0, len(pages))
	for _, page := range pages {
		text := normalizeTextPreserveNewlines(page.Text)
		if text == "" {
			continue
		}
		out = append(out, pageExtraction{
			Page:        page.Page,
			Text:        text,
			Method:      "paddleocr",
			OCRAvgScore: page.AvgScore,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("paddleocr extracted no usable text")
	}
	return out, nil
}

func (a *App) parseEPUB(path string) ([]chunkPayload, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer reader.Close()
	var chunks []chunkPayload
	for _, file := range reader.File {
		name := strings.ToLower(file.Name)
		if !(strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm")) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("read epub file: %w", err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read epub content: %w", err)
		}
		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse epub html: %w", err)
		}
		text := normalizeTextPreserveNewlines(extractText(doc))
		baseName := filepath.Base(file.Name)
		for idx, part := range chunkTextSemantic(text, a.chunkSize, a.chunkOverlap) {
			chunks = append(chunks, chunkPayload{
				Content: part,
				Metadata: map[string]string{
					"source_type":    "epub",
					"source_ref":     fmt.Sprintf("section:%s", baseName),
					"section":        baseName,
					"chunk":          strconv.Itoa(idx),
					"extract_method": "epub_html_parser",
				},
			})
		}
	}
	return chunks, nil
}

func (a *App) parseText(path string) ([]chunkPayload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	text := normalizeTextPreserveNewlines(string(data))
	parts := chunkTextSemantic(text, a.chunkSize, a.chunkOverlap)
	chunks := make([]chunkPayload, 0, len(parts))
	for idx, part := range parts {
		chunks = append(chunks, chunkPayload{
			Content: part,
			Metadata: map[string]string{
				"source_type":    "text",
				"source_ref":     "text",
				"chunk":          strconv.Itoa(idx),
				"extract_method": "plain_text_parser",
			},
		})
	}
	return chunks, nil
}

func (a *App) mergePDFPages(nativePages []pageExtraction, ocrPages []pageExtraction) []pageExtraction {
	native := make(map[int]pageExtraction, len(nativePages))
	for _, page := range nativePages {
		native[page.Page] = page
	}
	ocr := make(map[int]pageExtraction, len(ocrPages))
	for _, page := range ocrPages {
		ocr[page.Page] = page
	}
	allPages := make(map[int]struct{}, len(native)+len(ocr))
	for page := range native {
		allPages[page] = struct{}{}
	}
	for page := range ocr {
		allPages[page] = struct{}{}
	}
	result := make([]pageExtraction, 0, len(allPages))
	for pageNo := range allPages {
		nativePage, hasNative := native[pageNo]
		ocrPage, hasOCR := ocr[pageNo]
		switch {
		case hasNative && !hasOCR:
			result = append(result, nativePage)
		case !hasNative && hasOCR:
			result = append(result, ocrPage)
		case hasNative && hasOCR:
			nativeQuality := evaluatePageQuality(nativePage.Text)
			ocrQuality := evaluatePageQuality(ocrPage.Text)
			ocrScore := ocrQuality.Score
			if ocrPage.OCRAvgScore > 0 {
				ocrScore = (ocrScore * 0.8) + (ocrPage.OCRAvgScore * 0.2)
			}
			if isLowQualityPage(nativeQuality, a.pdfMinRunes, a.pdfMinScore) && ocrScore >= nativeQuality.Score+a.pdfScoreDiff {
				result = append(result, ocrPage)
				continue
			}
			result = append(result, nativePage)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Page < result[j].Page
	})
	return result
}

func (a *App) buildPDFChunks(pages []pageExtraction) []chunkPayload {
	chunks := make([]chunkPayload, 0, len(pages))
	for _, page := range pages {
		quality := evaluatePageQuality(page.Text)
		for idx, part := range chunkTextSemantic(page.Text, a.chunkSize, a.chunkOverlap) {
			meta := map[string]string{
				"source_type":        "pdf",
				"source_ref":         fmt.Sprintf("page:%d", page.Page),
				"page":               strconv.Itoa(page.Page),
				"chunk":              strconv.Itoa(idx),
				"extract_method":     page.Method,
				"page_quality_score": fmt.Sprintf("%.3f", quality.Score),
				"page_runes":         strconv.Itoa(quality.Runes),
			}
			if page.OCRAvgScore > 0 {
				meta["ocr_avg_score"] = fmt.Sprintf("%.3f", page.OCRAvgScore)
			}
			chunks = append(chunks, chunkPayload{
				Content:  part,
				Metadata: meta,
			})
		}
	}
	return chunks
}

func evaluatePageQuality(text string) pageQuality {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return pageQuality{}
	}
	alphaNum := 0
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alphaNum++
		}
	}
	lines := strings.Split(string(runes), "\n")
	nonEmpty := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	alphaNumRatio := float64(alphaNum) / float64(len(runes))
	lineCount := nonEmpty
	if lineCount <= 0 {
		lineCount = 1
	}
	avgLineLen := float64(len(runes)) / float64(lineCount)
	lengthScore := clamp01(float64(len(runes)) / 300.0)
	densityScore := clamp01(avgLineLen / 24.0)
	score := (0.45 * lengthScore) + (0.30 * alphaNumRatio) + (0.25 * densityScore)
	return pageQuality{
		Runes:         len(runes),
		NonEmptyLines: nonEmpty,
		AlphaNumRatio: alphaNumRatio,
		Score:         score,
	}
}

func isLowQualityPage(q pageQuality, minRunes int, minScore float64) bool {
	if q.Runes == 0 {
		return true
	}
	if minRunes > 0 && q.Runes < minRunes {
		return true
	}
	return q.Score < minScore
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type ocrPage struct {
	Page     int
	Text     string
	AvgScore float64
}

func readPaddleOCRPages(savePath string) ([]ocrPage, error) {
	files, err := filepath.Glob(filepath.Join(savePath, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("scan paddleocr json files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("paddleocr output missing json in %s", savePath)
	}
	sort.Strings(files)
	var pages []ocrPage
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read paddleocr json: %w", err)
		}
		pageSet, err := parsePaddleOCRJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("parse paddleocr json %s: %w", filepath.Base(file), err)
		}
		pages = append(pages, pageSet...)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("paddleocr json has no page text")
	}
	sort.SliceStable(pages, func(i, j int) bool {
		return pages[i].Page < pages[j].Page
	})
	return pages, nil
}

func parsePaddleOCRJSON(raw []byte) ([]ocrPage, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	items := findPageItems(payload)
	if len(items) == 0 {
		entries := collectRecEntries(payload)
		if len(entries) == 0 {
			return nil, fmt.Errorf("no rec_texts found")
		}
		return []ocrPage{buildOCRPage(1, entries)}, nil
	}

	pages := make([]ocrPage, 0, len(items))
	for i, item := range items {
		entries := collectRecEntries(item)
		if len(entries) == 0 {
			continue
		}
		pageNum := i + 1
		if m, ok := item.(map[string]any); ok {
			if v, ok := m["page_index"]; ok {
				pageNum = toInt(v) + 1
			} else if v, ok := m["page"]; ok {
				pageNum = toInt(v)
				if pageNum <= 0 {
					pageNum = i + 1
				}
			}
		}
		pages = append(pages, buildOCRPage(pageNum, entries))
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no page texts found")
	}
	return pages, nil
}

type ocrEntry struct {
	Text  string
	Score float64
}

func buildOCRPage(pageNum int, entries []ocrEntry) ocrPage {
	texts := make([]string, 0, len(entries))
	scoreTotal := 0.0
	scoreCount := 0
	for _, entry := range entries {
		if entry.Text == "" {
			continue
		}
		texts = append(texts, entry.Text)
		if entry.Score > 0 {
			scoreTotal += entry.Score
			scoreCount++
		}
	}
	avgScore := 0.0
	if scoreCount > 0 {
		avgScore = scoreTotal / float64(scoreCount)
	}
	return ocrPage{
		Page:     pageNum,
		Text:     strings.Join(texts, "\n"),
		AvgScore: avgScore,
	}
}

func findPageItems(payload any) []any {
	switch v := payload.(type) {
	case []any:
		return v
	case map[string]any:
		if out, ok := v["ocrResults"]; ok {
			if items, ok := out.([]any); ok {
				return items
			}
		}
	}
	return nil
}

func collectRecEntries(payload any) []ocrEntry {
	var out []ocrEntry
	var walk func(any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if raw, ok := v["rec_texts"]; ok {
				if arr, ok := raw.([]any); ok {
					scores := make([]float64, 0, len(arr))
					if rawScores, ok := v["rec_scores"]; ok {
						if scoreArr, ok := rawScores.([]any); ok {
							for _, item := range scoreArr {
								scores = append(scores, toFloat(item))
							}
						}
					}
					for i, item := range arr {
						if text, ok := item.(string); ok {
							text = strings.TrimSpace(text)
							if text == "" {
								continue
							}
							score := 0.0
							if len(scores) > 0 && i < len(scores) {
								score = scores[i]
							}
							out = append(out, ocrEntry{Text: text, Score: score})
						}
					}
				}
			}
			for _, value := range v {
				walk(value)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(payload)
	return out
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return i
		}
	}
	return 0
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

func normalizeTextPreserveNewlines(text string) string {
	text = strings.ReplaceAll(text, "\x00", " ")
	text = strings.ToValidUTF8(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", " ")
	text = sanitizeTextRunes(text)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	text = strings.TrimSpace(strings.Join(lines, "\n"))
	if text == "" {
		return ""
	}
	return text
}

func sanitizeTextRunes(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff', '\u2060', '\u00ad':
			// Remove zero-width chars and soft hyphen noise from OCR/PDF extraction.
			continue
		case '\u00a0':
			b.WriteRune(' ')
			continue
		}
		if unicode.IsControl(r) && r != '\n' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type sentenceUnit struct {
	text           string
	paragraphStart bool
}

func chunkTextSemantic(text string, size, overlap int) []string {
	if size <= 0 {
		return nil
	}
	units := buildSentenceUnits(text, size)
	if len(units) == 0 {
		return nil
	}
	var chunks []string
	var current []sentenceUnit
	currentLen := 0
	for _, unit := range units {
		unitLen := runeLen(unit.text)
		sepLen := 0
		if len(current) > 0 {
			sepLen = separatorLen(unit.paragraphStart)
		}
		if len(current) > 0 && currentLen+sepLen+unitLen > size {
			chunkText := unitsToText(current)
			if chunkText != "" {
				chunks = append(chunks, chunkText)
			}
			current = overlapUnits(current, overlap)
			currentLen = unitsLen(current)
			sepLen = 0
			if len(current) > 0 {
				sepLen = separatorLen(unit.paragraphStart)
				if currentLen+sepLen+unitLen > size {
					current = nil
					currentLen = 0
					sepLen = 0
				}
			}
		}
		current = append(current, unit)
		currentLen += sepLen + unitLen
	}
	if len(current) > 0 {
		chunkText := unitsToText(current)
		if chunkText != "" {
			chunks = append(chunks, chunkText)
		}
	}
	return chunks
}

func buildSentenceUnits(text string, size int) []sentenceUnit {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return nil
	}
	units := make([]sentenceUnit, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		sentences := splitSentences(paragraph)
		if len(sentences) == 0 {
			continue
		}
		for i, sentence := range sentences {
			sentence = strings.TrimSpace(sentence)
			if sentence == "" {
				continue
			}
			parts := splitByRunes(sentence, size)
			for j, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				units = append(units, sentenceUnit{
					text:           part,
					paragraphStart: i == 0 && j == 0,
				})
			}
		}
	}
	return units
}

func splitParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	var paragraphs []string
	buf := make([]string, 0, len(lines))
	flush := func() {
		if len(buf) == 0 {
			return
		}
		paragraph := strings.TrimSpace(strings.Join(buf, " "))
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
		buf = buf[:0]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		line = strings.Join(strings.Fields(line), " ")
		buf = append(buf, line)
	}
	flush()
	return paragraphs
}

func splitSentences(paragraph string) []string {
	var sentences []string
	var buf []rune
	for _, r := range []rune(paragraph) {
		buf = append(buf, r)
		if isSentenceEnd(r) {
			sentence := strings.TrimSpace(string(buf))
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		sentence := strings.TrimSpace(string(buf))
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	return sentences
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '\u3002', '\uff01', '\uff1f', '\uff1b', '\uff0e':
		return true
	default:
		return false
	}
}

func splitByRunes(text string, size int) []string {
	if size <= 0 {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= size {
		return []string{string(runes)}
	}
	var parts []string
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func unitsToText(units []sentenceUnit) string {
	var sb strings.Builder
	for i, unit := range units {
		if i > 0 {
			if unit.paragraphStart {
				sb.WriteString("\n\n")
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString(unit.text)
	}
	return strings.TrimSpace(sb.String())
}

func unitsLen(units []sentenceUnit) int {
	if len(units) == 0 {
		return 0
	}
	length := runeLen(units[0].text)
	for i := 1; i < len(units); i++ {
		length += separatorLen(units[i].paragraphStart)
		length += runeLen(units[i].text)
	}
	return length
}

func overlapUnits(units []sentenceUnit, overlap int) []sentenceUnit {
	if overlap <= 0 || len(units) == 0 {
		return nil
	}
	length := 0
	sepLen := 0
	start := len(units)
	for i := len(units) - 1; i >= 0; i-- {
		unitLen := runeLen(units[i].text)
		if start == len(units) {
			length = unitLen
			start = i
		} else {
			length += sepLen + unitLen
			start = i
		}
		sepLen = separatorLen(units[i].paragraphStart)
		if length >= overlap {
			break
		}
	}
	if start == len(units) {
		return nil
	}
	return append([]sentenceUnit(nil), units[start:]...)
}

func separatorLen(paragraphStart bool) int {
	if paragraphStart {
		return 2
	}
	return 1
}

func runeLen(text string) int {
	return len([]rune(text))
}

func extractText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		switch node.Type {
		case html.TextNode:
			buf.WriteString(node.Data)
			buf.WriteString(" ")
		case html.ElementNode:
			if node.Data == "script" || node.Data == "style" {
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if node.Type == html.ElementNode {
			switch node.Data {
			case "br":
				buf.WriteString("\n")
			case "p", "div", "li", "h1", "h2", "h3", "h4", "h5", "h6":
				buf.WriteString("\n\n")
			}
		}
	}
	walk(n)
	return buf.String()
}
