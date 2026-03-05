package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadQrelsTSV(t *testing.T) {
	path := filepath.Join("testdata", "qrels.tsv")
	qrels, err := ReadQrels(path)
	if err != nil {
		t.Fatalf("ReadQrels failed: %v", err)
	}
	if len(qrels) != 4 {
		t.Fatalf("expected 4 qrels, got %d", len(qrels))
	}
	if qrels[0].QID != "q1" || qrels[0].DocID != "c1" || qrels[0].Relevance != 2 {
		t.Fatalf("unexpected first qrel: %+v", qrels[0])
	}
}

func TestReadChunksJSONLAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chunks.jsonl")
	content := "{\"id\":\"a1\",\"book_id\":\"b1\",\"content\":\"hello\",\"meta\":{\"source_type\":\"text\",\"source_ref\":\"text\",\"extract_method\":\"plain\"}}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	chunks, err := ReadChunksJSONL(path)
	if err != nil {
		t.Fatalf("ReadChunksJSONL failed: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].ChunkID != "a1" || chunks[0].DocID != "b1" || chunks[0].Text != "hello" {
		t.Fatalf("unexpected chunk: %+v", chunks[0])
	}
}

func TestReadJSONLError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{bad}\n"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if _, err := ReadQueriesJSONL(path); err == nil {
		t.Fatalf("expected parse error")
	}
}
