package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunAllProducesReports(t *testing.T) {
	base := filepath.Join("..", "..", "internal", "eval", "testdata")
	outDir := filepath.Join(t.TempDir(), "rag-eval")
	args := []string{
		"all",
		"--chunks", filepath.Join(base, "chunks.jsonl"),
		"--queries", filepath.Join(base, "queries.jsonl"),
		"--qrels", filepath.Join(base, "qrels.tsv"),
		"--predictions", filepath.Join(base, "predictions.jsonl"),
		"--run", filepath.Join(base, "run.jsonl"),
		"--embeddings", filepath.Join(base, "embeddings.jsonl"),
		"--out-dir", outDir,
	}
	if err := run(args); err != nil {
		t.Fatalf("run all failed: %v", err)
	}
	for _, name := range []string{"run.json", "metrics.json", "per_query.jsonl"} {
		path := filepath.Join(outDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing output file %s: %v", name, err)
		}
	}
}
