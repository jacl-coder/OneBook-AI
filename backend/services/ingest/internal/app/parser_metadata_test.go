package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTextAddsExtractMethodMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("alpha beta gamma\ndelta epsilon zeta"), 0o644); err != nil {
		t.Fatalf("write temp text: %v", err)
	}

	a := &App{
		chunkSize:    16,
		chunkOverlap: 4,
	}
	chunks, err := a.parseText(path)
	if err != nil {
		t.Fatalf("parseText() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("parseText() returned no chunks")
	}
	for i, chunk := range chunks {
		if chunk.Metadata["source_type"] != "text" {
			t.Fatalf("chunk[%d] source_type=%q, want text", i, chunk.Metadata["source_type"])
		}
		if chunk.Metadata["extract_method"] != "plain_text_parser" {
			t.Fatalf("chunk[%d] extract_method=%q, want plain_text_parser", i, chunk.Metadata["extract_method"])
		}
	}
}

func TestBuildPDFChunksAddsTraceMetadata(t *testing.T) {
	a := &App{
		chunkSize:    80,
		chunkOverlap: 0,
	}
	pages := []pageExtraction{
		{
			Page:   2,
			Text:   "This page text should become one chunk with trace metadata.",
			Method: "pdftotext",
		},
	}
	chunks := a.buildPDFChunks(pages)
	if len(chunks) == 0 {
		t.Fatalf("buildPDFChunks() returned no chunks")
	}
	meta := chunks[0].Metadata
	if meta["extract_method"] != "pdftotext" {
		t.Fatalf("extract_method=%q, want pdftotext", meta["extract_method"])
	}
	if meta["source_ref"] != "page:2" {
		t.Fatalf("source_ref=%q, want page:2", meta["source_ref"])
	}
	if meta["page_quality_score"] == "" {
		t.Fatalf("page_quality_score should not be empty")
	}
	if meta["page_runes"] == "" {
		t.Fatalf("page_runes should not be empty")
	}
}
