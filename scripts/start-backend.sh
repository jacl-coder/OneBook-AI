#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AUTH_PORT="${AUTH_PORT:-8081}"
BOOK_PORT="${BOOK_PORT:-8082}"
CHAT_PORT="${CHAT_PORT:-8083}"
INGEST_PORT="${INGEST_PORT:-8084}"
INDEXER_PORT="${INDEXER_PORT:-8085}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"
STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-120}"

PIDS=()

to_abs_path() {
  local path="$1"
  if [[ "$path" = /* ]]; then
    echo "$path"
    return
  fi
  echo "$ROOT_DIR/$path"
}

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
}

wait_for_service() {
  local name="$1"
  local url="$2"
  local pid="$3"
  local timeout="${4:-120}"
  local elapsed=0

  echo "Waiting for ${name} to be ready..."
  until curl -sf "$url" >/dev/null 2>&1; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "${name} exited before becoming ready."
      exit 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
    if (( elapsed >= timeout )); then
      echo "Timed out waiting for ${name}: ${url}"
      exit 1
    fi
  done
  echo "${name} is ready."
}

if [ -f "$ROOT_DIR/.env" ]; then
  echo "Loading environment variables from .env..."
  set -a
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
  set +a
fi

if [[ -z "${CORS_ALLOWED_ORIGINS:-}" ]]; then
  export CORS_ALLOWED_ORIGINS="http://localhost:8086"
  echo "CORS_ALLOWED_ORIGINS not set, defaulting to ${CORS_ALLOWED_ORIGINS}"
fi

JWT_PRIVATE_KEY_PATH="$(to_abs_path "${JWT_PRIVATE_KEY_PATH:-secrets/jwt/private.pem}")"
JWT_PUBLIC_KEY_PATH="$(to_abs_path "${JWT_PUBLIC_KEY_PATH:-secrets/jwt/public.pem}")"
JWT_KEY_ID="${JWT_KEY_ID:-jwt-active}"
export JWT_PRIVATE_KEY_PATH JWT_PUBLIC_KEY_PATH JWT_KEY_ID

mkdir -p "$(dirname "$JWT_PRIVATE_KEY_PATH")"
if [ ! -f "$JWT_PRIVATE_KEY_PATH" ] || [ ! -f "$JWT_PUBLIC_KEY_PATH" ]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate RS256 JWT keys"
    exit 1
  fi
  echo "Generating local RS256 JWT keypair under $(dirname "$JWT_PRIVATE_KEY_PATH")..."
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$JWT_PRIVATE_KEY_PATH" >/dev/null 2>&1
  openssl rsa -in "$JWT_PRIVATE_KEY_PATH" -pubout -out "$JWT_PUBLIC_KEY_PATH" >/dev/null 2>&1
  chmod 600 "$JWT_PRIVATE_KEY_PATH"
fi

ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH="$(to_abs_path "${ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH:-secrets/internal-jwt/private.pem}")"
ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH="$(to_abs_path "${ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH:-secrets/internal-jwt/public.pem}")"
ONEBOOK_INTERNAL_JWT_KEY_ID="${ONEBOOK_INTERNAL_JWT_KEY_ID:-internal-active}"
export ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH ONEBOOK_INTERNAL_JWT_KEY_ID

mkdir -p "$(dirname "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH")"
if [ ! -f "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH" ] || [ ! -f "$ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH" ]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate internal RS256 JWT keys"
    exit 1
  fi
  echo "Generating local internal RS256 JWT keypair under $(dirname "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH")..."
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH" >/dev/null 2>&1
  openssl rsa -in "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH" -pubout -out "$ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH" >/dev/null 2>&1
  chmod 600 "$ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH"
fi

mkdir -p "$ROOT_DIR/backend/.cache/go-build"
trap cleanup EXIT INT TERM

"$ROOT_DIR/scripts/ollama-embedding.sh"

docker compose -f "$ROOT_DIR/docker-compose.yml" up -d postgres redis minio minio-init swagger-ui

echo "Waiting for MinIO to be ready..."
until curl -sf http://localhost:9000/minio/health/live >/dev/null 2>&1; do
  sleep 1
done
echo "MinIO is ready."

echo "Waiting for Postgres to be ready..."
until docker exec onebook-postgres pg_isready -U onebook >/dev/null 2>&1; do
  sleep 1
done
echo "Postgres is ready."

cd "$ROOT_DIR/backend/services/auth"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/auth &
AUTH_PID=$!
PIDS+=("$AUTH_PID")
wait_for_service "Auth service" "http://localhost:${AUTH_PORT}/healthz" "$AUTH_PID" "$STARTUP_TIMEOUT_SECONDS"

cd "$ROOT_DIR/backend/services/book"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/book &
BOOK_PID=$!
PIDS+=("$BOOK_PID")
wait_for_service "Book service" "http://localhost:${BOOK_PORT}/healthz" "$BOOK_PID" "$STARTUP_TIMEOUT_SECONDS"

cd "$ROOT_DIR/backend/services/chat"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/chat &
CHAT_PID=$!
PIDS+=("$CHAT_PID")
wait_for_service "Chat service" "http://localhost:${CHAT_PORT}/healthz" "$CHAT_PID" "$STARTUP_TIMEOUT_SECONDS"

cd "$ROOT_DIR/backend/services/ingest"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/ingest &
INGEST_PID=$!
PIDS+=("$INGEST_PID")
wait_for_service "Ingest service" "http://localhost:${INGEST_PORT}/healthz" "$INGEST_PID" "$STARTUP_TIMEOUT_SECONDS"

cd "$ROOT_DIR/backend/services/indexer"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/indexer &
INDEXER_PID=$!
PIDS+=("$INDEXER_PID")
wait_for_service "Indexer service" "http://localhost:${INDEXER_PORT}/healthz" "$INDEXER_PID" "$STARTUP_TIMEOUT_SECONDS"

echo "All backend services are ready."
echo "Gateway:  http://localhost:${GATEWAY_PORT}"

cd "$ROOT_DIR/backend/services/gateway"
GOCACHE="$ROOT_DIR/backend/.cache/go-build" go run ./cmd/server
