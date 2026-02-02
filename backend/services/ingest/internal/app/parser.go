package app

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"
)

type chunkPayload struct {
	Content  string
	Metadata map[string]string
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
	// Try pdftotext first (better support for complex/Chinese PDFs)
	chunks, err := a.parsePDFWithPdftotext(path)
	if err == nil && len(chunks) > 0 {
		return chunks, nil
	}
	// Fallback to Go library
	return a.parsePDFWithGoLib(path)
}

// parsePDFWithPdftotext uses the system pdftotext tool (poppler-utils)
func (a *App) parsePDFWithPdftotext(path string) ([]chunkPayload, error) {
	// Check if pdftotext is available
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return nil, fmt.Errorf("pdftotext not found: %w", err)
	}

	// Run pdftotext to extract text
	cmd := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", path, "-")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext failed: %w", err)
	}

	var chunks []chunkPayload
	raw := strings.ReplaceAll(string(output), "\x00", " ")
	raw = strings.ToValidUTF8(raw, "")
	pages := strings.Split(raw, "\f")
	for pageIdx, pageText := range pages {
		pageText = normalizeTextPreserveNewlines(pageText)
		if pageText == "" {
			continue
		}
		for idx, part := range chunkTextSemantic(pageText, a.chunkSize, a.chunkOverlap) {
			chunks = append(chunks, chunkPayload{
				Content: part,
				Metadata: map[string]string{
					"page":  strconv.Itoa(pageIdx + 1),
					"chunk": strconv.Itoa(idx),
				},
			})
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no text extracted from PDF")
	}
	return chunks, nil
}

// parsePDFWithGoLib uses the Go PDF library (fallback)
func (a *App) parsePDFWithGoLib(path string) ([]chunkPayload, error) {
	file, reader, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()
	totalPages := reader.NumPage()
	var chunks []chunkPayload
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
		for idx, part := range chunkTextSemantic(text, a.chunkSize, a.chunkOverlap) {
			chunks = append(chunks, chunkPayload{
				Content: part,
				Metadata: map[string]string{
					"page":  strconv.Itoa(i),
					"chunk": strconv.Itoa(idx),
				},
			})
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no text extracted from PDF")
	}
	return chunks, nil
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
					"section": baseName,
					"chunk":   strconv.Itoa(idx),
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
				"chunk": strconv.Itoa(idx),
			},
		})
	}
	return chunks, nil
}

func normalizeTextPreserveNewlines(text string) string {
	text = strings.ReplaceAll(text, "\x00", " ")
	text = strings.ToValidUTF8(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", " ")
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
