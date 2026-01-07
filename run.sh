#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load environment variables from .env file
if [ -f "$ROOT_DIR/.env" ]; then
  echo "Loading environment variables from .env..."
  set -a
  source "$ROOT_DIR/.env"
  set +a
fi

mkdir -p "$ROOT_DIR/backend/.cache/go-build"

# Start local dependencies.
docker compose -f "$ROOT_DIR/docker-compose.yml" up -d postgres redis minio minio-init swagger-ui

# Wait for MinIO to be ready.
echo "Waiting for MinIO to be ready..."
until curl -sf http://localhost:9000/minio/health/live >/dev/null 2>&1; do
  sleep 1
done
echo "MinIO is ready."

# Wait for Postgres to be ready.
echo "Waiting for Postgres to be ready..."
until docker exec onebook-postgres pg_isready -U onebook >/dev/null 2>&1; do
  sleep 1
done
echo "Postgres is ready."

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
