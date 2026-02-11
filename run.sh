#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_SCRIPT="${BACKEND_SCRIPT:-$ROOT_DIR/scripts/start-backend.sh}"
FRONTEND_SCRIPT="${FRONTEND_SCRIPT:-$ROOT_DIR/scripts/start-frontend.sh}"
START_FRONTEND="${START_FRONTEND:-auto}" # auto|on|off
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

wait_for_url() {
  local name="$1"
  local url="$2"
  local pid="$3"
  local timeout="${4:-120}"
  local elapsed=0

  echo "Waiting for ${name}..."
  until curl -sf "$url" >/dev/null 2>&1; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "${name} exited before ready."
      return 1
    fi
    sleep 1
    elapsed=$((elapsed + 1))
    if (( elapsed >= timeout )); then
      echo "Timed out waiting for ${name}: ${url}"
      return 1
    fi
  done
  echo "${name} is ready."
}

mode="$(printf '%s' "$START_FRONTEND" | tr '[:upper:]' '[:lower:]')"
should_start_frontend=false
if [[ "$mode" == "on" || "$mode" == "1" || "$mode" == "true" ]]; then
  should_start_frontend=true
elif [[ "$mode" == "auto" ]]; then
  if [[ -f "$FRONTEND_SCRIPT" ]] \
    && [[ -f "$ROOT_DIR/frontend/package.json" ]] \
    && command -v npm >/dev/null 2>&1; then
    should_start_frontend=true
  fi
fi

if [[ ! -f "$BACKEND_SCRIPT" ]]; then
  echo "Backend startup script not found: ${BACKEND_SCRIPT}"
  exit 1
fi

trap cleanup EXIT INT TERM

if [[ "$should_start_frontend" == "true" ]]; then
  if [[ ! -f "$FRONTEND_SCRIPT" ]]; then
    echo "Frontend startup script not found: ${FRONTEND_SCRIPT}"
    exit 1
  fi

  echo "Starting frontend via ${FRONTEND_SCRIPT}..."
  bash "$FRONTEND_SCRIPT" &
  FRONTEND_WRAPPER_PID=$!
  PIDS+=("$FRONTEND_WRAPPER_PID")

  if [[ "$mode" != "auto" ]]; then
    wait_for_url \
      "Frontend (Vite)" \
      "http://localhost:${FRONTEND_PORT}" \
      "$FRONTEND_WRAPPER_PID" \
      "$STARTUP_TIMEOUT_SECONDS"
  fi
else
  echo "Frontend startup disabled (START_FRONTEND=${START_FRONTEND})."
fi

echo "Starting backend via ${BACKEND_SCRIPT}..."
bash "$BACKEND_SCRIPT"
