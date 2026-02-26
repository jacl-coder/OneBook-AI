package app

import "testing"

func TestEnrichChunkMetadata(t *testing.T) {
	base := map[string]string{
		"source_type": "pdf",
		"source_ref":  "page:3",
		"page":        "3",
	}
	content := "Hello, 世界"
	got := enrichChunkMetadata(base, "book-123", 5, 20, content)

	if got["document_id"] != "book-123" {
		t.Fatalf("document_id = %q, want %q", got["document_id"], "book-123")
	}
	if got["chunk_index"] != "5" {
		t.Fatalf("chunk_index = %q, want %q", got["chunk_index"], "5")
	}
	if got["chunk_count"] != "20" {
		t.Fatalf("chunk_count = %q, want %q", got["chunk_count"], "20")
	}
	if got["content_runes"] != "9" {
		t.Fatalf("content_runes = %q, want %q", got["content_runes"], "9")
	}
	if got["content_sha256"] == "" {
		t.Fatalf("content_sha256 should not be empty")
	}
	if got["source_ref"] != "page:3" {
		t.Fatalf("source_ref = %q, want %q", got["source_ref"], "page:3")
	}
}
