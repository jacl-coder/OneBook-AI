#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Load .env for optional OLLAMA_* settings.
if [ -f "$ROOT_DIR/.env" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
  set +a
fi

OLLAMA_HOST="${OLLAMA_HOST:-http://127.0.0.1:11434}"
MODEL="${1:-${OLLAMA_EMBEDDING_MODEL:-qwen3-embedding:latest}}"
LOG_DIR="$ROOT_DIR/.cache"
LOG_FILE="$LOG_DIR/ollama-serve.log"

if ! command -v ollama >/dev/null 2>&1; then
  echo "ollama not found in PATH. Please install/start Ollama first."
  exit 1
fi

mkdir -p "$LOG_DIR"

if ! curl -sf "$OLLAMA_HOST/api/tags" >/dev/null 2>&1; then
  echo "Starting ollama server..."
  # Start in background; log to file for troubleshooting.
  nohup ollama serve >"$LOG_FILE" 2>&1 &

  echo "Waiting for ollama server to be ready..."
  for _ in {1..30}; do
    if curl -sf "$OLLAMA_HOST/api/tags" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

if ! curl -sf "$OLLAMA_HOST/api/tags" >/dev/null 2>&1; then
  echo "Ollama server not reachable at $OLLAMA_HOST"
  echo "Check logs: $LOG_FILE"
  exit 1
fi

echo "Pulling model: $MODEL"
ollama pull "$MODEL"

echo "Ollama is ready."
echo "Server: $OLLAMA_HOST"
echo "Model:  $MODEL"
