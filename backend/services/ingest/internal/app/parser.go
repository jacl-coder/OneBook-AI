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

	text := normalizeText(string(output))
	if text == "" {
		return nil, fmt.Errorf("no text extracted from PDF")
	}

	var chunks []chunkPayload
	for idx, part := range chunkText(text, a.chunkSize, a.chunkOverlap) {
		chunks = append(chunks, chunkPayload{
			Content: part,
			Metadata: map[string]string{
				"chunk": strconv.Itoa(idx),
			},
		})
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
		text = normalizeText(text)
		for idx, part := range chunkText(text, a.chunkSize, a.chunkOverlap) {
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
		text := normalizeText(extractText(doc))
		baseName := filepath.Base(file.Name)
		for idx, part := range chunkText(text, a.chunkSize, a.chunkOverlap) {
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
	text := normalizeText(string(data))
	parts := chunkText(text, a.chunkSize, a.chunkOverlap)
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

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\x00", " ")
	text = strings.ToValidUTF8(text, "")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func chunkText(text string, size, overlap int) []string {
	if size <= 0 {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	step := size - overlap
	if step <= 0 {
		step = size
	}
	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			chunks = append(chunks, part)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
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
		if node.Type == html.ElementNode && (node.Data == "p" || node.Data == "br" || node.Data == "div" || node.Data == "li") {
			buf.WriteString(" ")
		}
	}
	walk(n)
	return buf.String()
}
