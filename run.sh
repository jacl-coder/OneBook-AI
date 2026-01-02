#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$ROOT_DIR/backend/.cache/go-build"

# Start local dependencies.
docker compose -f "$ROOT_DIR/docker-compose.yml" up -d postgres redis minio minio-init

# Run the auth service.
cd "$ROOT_DIR/backend/services/auth"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/auth &
AUTH_PID=$!

# Run the book service.
cd "$ROOT_DIR/backend/services/book"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/book &
BOOK_PID=$!

# Run the chat service.
cd "$ROOT_DIR/backend/services/chat"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/chat &
CHAT_PID=$!

# Run the ingest service.
cd "$ROOT_DIR/backend/services/ingest"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/ingest &
INGEST_PID=$!

# Run the indexer service.
cd "$ROOT_DIR/backend/services/indexer"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/indexer &
INDEXER_PID=$!

trap 'kill "$AUTH_PID" "$BOOK_PID" "$CHAT_PID" "$INGEST_PID" "$INDEXER_PID" 2>/dev/null || true' EXIT

# Run the gateway service.
cd "$ROOT_DIR/backend/services/gateway"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/server
