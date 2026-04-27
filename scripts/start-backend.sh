#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AUTH_PORT="${AUTH_PORT:-8082}"
BOOK_PORT="${BOOK_PORT:-8083}"
CHAT_PORT="${CHAT_PORT:-8084}"
INGEST_PORT="${INGEST_PORT:-8085}"
INDEXER_PORT="${INDEXER_PORT:-8086}"
GATEWAY_PORT="${GATEWAY_PORT:-8081}"
STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-120}"
RABBITMQ_URL="${RABBITMQ_URL:-amqp://onebook:onebook@localhost:5672/}"
INGEST_QUEUE_EXCHANGE="${INGEST_QUEUE_EXCHANGE:-onebook.jobs}"
INGEST_QUEUE_NAME="${INGEST_QUEUE_NAME:-onebook.ingest.jobs}"
INGEST_QUEUE_CONSUMER="${INGEST_QUEUE_CONSUMER:-onebook-ingest-service}"
INDEXER_QUEUE_EXCHANGE="${INDEXER_QUEUE_EXCHANGE:-onebook.jobs}"
INDEXER_QUEUE_NAME="${INDEXER_QUEUE_NAME:-onebook.indexer.jobs}"
INDEXER_QUEUE_CONSUMER="${INDEXER_QUEUE_CONSUMER:-onebook-indexer-service}"
export RABBITMQ_URL \
  INGEST_QUEUE_EXCHANGE INGEST_QUEUE_NAME INGEST_QUEUE_CONSUMER \
  INDEXER_QUEUE_EXCHANGE INDEXER_QUEUE_NAME INDEXER_QUEUE_CONSUMER

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

clear_onebook_env() {
  local var=""
  while IFS= read -r var; do
    case "$var" in
      DATABASE_URL|REDIS_ADDR|REDIS_PASSWORD|LOGS_DIR|ONEBOOK_EMBEDDING_DIM|EMBEDDING_BATCH_SIZE|EMBEDDING_CONCURRENCY|QDRANT_URL|QDRANT_API_KEY|QDRANT_COLLECTION|OPENSEARCH_URL|OPENSEARCH_INDEX|OPENSEARCH_USERNAME|OPENSEARCH_PASSWORD|MINIO_ENDPOINT|MINIO_ACCESS_KEY|MINIO_SECRET_KEY|MINIO_BUCKET|MINIO_USE_SSL|OLLAMA_HOST|OLLAMA_EMBEDDING_MODEL|GENERATION_PROVIDER|GENERATION_API_KEY|GENERATION_MODEL|GENERATION_BASE_URL|RERANKER_URL|RERANKER_MODEL|RERANKER_MODEL_REVISION|RERANKER_CACHE_DIR|RERANKER_SENTENCE_TRANSFORMERS_HOME|RERANKER_MAX_DOCS|RERANKER_MAX_CHARS|RERANKER_BATCH_SIZE|CORS_ALLOWED_ORIGINS|CORS_ALLOW_CREDENTIALS|AUTH_EVAL_STORAGE_DIR|AUTH_EVAL_WORKER_POLL_INTERVAL|RESEND_API_KEY|RESEND_FROM|ALIYUN_ACCESS_KEY_ID|ALIYUN_ACCESS_KEY_SECRET|ALIYUN_SMS_SIGN_NAME|ALIYUN_SMS_SIGNUP_LOGIN_TEMPLATE_CODE|ALIYUN_SMS_CHANGE_PHONE_TEMPLATE_CODE|ALIYUN_SMS_PASSWORD_RESET_TEMPLATE_CODE|ALIYUN_SMS_BIND_PHONE_TEMPLATE_CODE|ALIYUN_SMS_VERIFY_BINDING_TEMPLATE_CODE|OAUTH_GOOGLE_CLIENT_ID|OAUTH_GOOGLE_CLIENT_SECRET|OAUTH_GOOGLE_REDIRECT_URL|OAUTH_MICROSOFT_CLIENT_ID|OAUTH_MICROSOFT_CLIENT_SECRET|OAUTH_MICROSOFT_REDIRECT_URL|OAUTH_MICROSOFT_TENANT|OAUTH_STATE_REDIS_PREFIX|OAUTH_APP_BASE_URL)
        unset "$var"
        ;;
      JWT_*|ONEBOOK_*|RABBITMQ_*|CHAT_*|INGEST_*|INDEXER_*|AUTH_*)
        unset "$var"
        ;;
    esac
  done < <(compgen -e)
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

configure_ocr_runtime() {
  COMPOSE_ARGS=(-f "$ROOT_DIR/docker-compose.yml")

  if ! command -v nvidia-smi >/dev/null 2>&1; then
    echo "NVIDIA GPU not detected on host; OCR service will run on CPU."
    return
  fi

  if ! command -v docker >/dev/null 2>&1; then
    echo "docker not found; OCR GPU auto-detection skipped."
    return
  fi

  if docker info --format '{{json .Runtimes}}' 2>/dev/null | grep -q '"nvidia"'; then
    echo "NVIDIA GPU runtime detected; enabling OCR GPU override."
    COMPOSE_ARGS+=(-f "$ROOT_DIR/docker-compose.gpu.yml")
    return
  fi

  echo "NVIDIA GPU detected, but Docker is not configured for GPU containers."
  echo "OCR service will run on CPU."
  echo "To enable GPU support, run: sudo ./scripts/setup-nvidia-container-toolkit.sh"
}

clear_onebook_env

if [ -f "$ROOT_DIR/.env" ]; then
  echo "Loading environment variables from .env..."
  set -a
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
  set +a
fi

if [[ -z "${ONEBOOK_EMBEDDING_DIM:-}" ]]; then
  export ONEBOOK_EMBEDDING_DIM="3072"
  echo "ONEBOOK_EMBEDDING_DIM not set, defaulting to ${ONEBOOK_EMBEDDING_DIM}"
fi

# Resolve LOGS_DIR to an absolute path so all services share the same directory.
LOGS_DIR="$(to_abs_path "${LOGS_DIR:-backend/logs}")"
export LOGS_DIR
mkdir -p "$LOGS_DIR"

if [[ -z "${CORS_ALLOWED_ORIGINS:-}" ]]; then
  # Default allow local Swagger UI and Vite dev server.
  export CORS_ALLOWED_ORIGINS="http://localhost:8089,http://localhost:5173"
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

configure_ocr_runtime

# Kill any leftover listeners on backend service ports to ensure idempotent startup.
for p in "$GATEWAY_PORT" "$AUTH_PORT" "$BOOK_PORT" "$CHAT_PORT" "$INGEST_PORT" "$INDEXER_PORT"; do
  fuser -k "${p}/tcp" >/dev/null 2>&1 || true
done

docker compose "${COMPOSE_ARGS[@]}" up -d postgres redis rabbitmq minio swagger-ui ocr-service reranker-service qdrant opensearch

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

echo "Waiting for RabbitMQ to be ready..."
until docker exec onebook-rabbitmq rabbitmq-diagnostics -q ping >/dev/null 2>&1; do
  sleep 2
done
echo "RabbitMQ is ready."

echo "Waiting for OCR service to be ready..."
until curl -sf http://localhost:8087/healthz >/dev/null 2>&1; do
  sleep 2
done
echo "OCR service is ready."

if [[ -n "${RERANKER_URL:-}" ]]; then
  echo "Waiting for reranker service to be ready..."
  until curl -sf "${RERANKER_URL%/rerank}/healthz" >/dev/null 2>&1; do
    sleep 2
  done
  echo "Reranker service is ready."
fi

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
