#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="${FRONTEND_DIR:-$ROOT_DIR/frontend}"
FRONTEND_HOST="${FRONTEND_HOST:-0.0.0.0}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-120}"

PIDS=()

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
  set -a
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
  set +a
fi

if [[ ! -d "$FRONTEND_DIR" || ! -f "$FRONTEND_DIR/package.json" ]]; then
  echo "Frontend project not found: ${FRONTEND_DIR}"
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required to start frontend."
  exit 1
fi

trap cleanup EXIT INT TERM

cd "$FRONTEND_DIR"
if [[ ! -d node_modules ]]; then
  echo "Installing frontend dependencies..."
  npm install
fi

npm run dev -- --host "$FRONTEND_HOST" --port "$FRONTEND_PORT" &
FRONTEND_PID=$!
PIDS+=("$FRONTEND_PID")

wait_for_service \
  "Frontend (Vite)" \
  "http://localhost:${FRONTEND_PORT}" \
  "$FRONTEND_PID" \
  "$STARTUP_TIMEOUT_SECONDS"

echo "Frontend URL: http://localhost:${FRONTEND_PORT}"
wait "$FRONTEND_PID"
