#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
TESTDATA_DIR="$BACKEND_DIR/internal/eval/testdata"

OUT_DIR="${1:-$BACKEND_DIR/.cache/rag-eval/manual-$(date -u +%Y%m%dT%H%M%SZ)}"

mkdir -p "$OUT_DIR"

cd "$BACKEND_DIR"

go run ./cmd/rag_eval all \
  --chunks "$TESTDATA_DIR/chunks.jsonl" \
  --queries "$TESTDATA_DIR/queries.jsonl" \
  --qrels "$TESTDATA_DIR/qrels.tsv" \
  --predictions "$TESTDATA_DIR/predictions.jsonl" \
  --run "$TESTDATA_DIR/run.jsonl" \
  --embeddings "$TESTDATA_DIR/embeddings.jsonl" \
  --out-dir "$OUT_DIR"

echo "RAG eval report generated at: $OUT_DIR"
