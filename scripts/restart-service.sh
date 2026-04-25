#!/usr/bin/env bash
# Usage: ./scripts/restart-service.sh <service>
# Services: auth | book | chat | ingest | indexer | gateway
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

SERVICE="${1:-}"
if [[ -z "$SERVICE" ]]; then
  echo "Usage: $0 <service>"
  echo "  Services: auth | book | chat | ingest | indexer | gateway"
  exit 1
fi

declare -A SERVICE_PORTS=(
  [auth]=8082
  [book]=8083
  [chat]=8084
  [ingest]=8085
  [indexer]=8086
  [gateway]=8081
)

declare -A SERVICE_CMDS=(
  [auth]="./cmd/auth"
  [book]="./cmd/book"
  [chat]="./cmd/chat"
  [ingest]="./cmd/ingest"
  [indexer]="./cmd/indexer"
  [gateway]="./cmd/server"
)

if [[ -z "${SERVICE_PORTS[$SERVICE]:-}" ]]; then
  echo "Unknown service: $SERVICE"
  echo "  Valid: auth | book | chat | ingest | indexer | gateway"
  exit 1
fi

PORT="${SERVICE_PORTS[$SERVICE]}"
CMD="${SERVICE_CMDS[$SERVICE]}"
SVC_DIR="$ROOT_DIR/backend/services/$SERVICE"

to_abs_path() {
  local path="$1"
  [[ "$path" = /* ]] && echo "$path" && return
  echo "$ROOT_DIR/$path"
}

# Load .env
if [[ -f "$ROOT_DIR/.env" ]]; then
  echo "Loading .env..."
  set -a
  # shellcheck disable=SC1090
  source "$ROOT_DIR/.env"
  set +a
fi

# Resolve key paths (same logic as start-backend.sh)
JWT_PRIVATE_KEY_PATH="$(to_abs_path "${JWT_PRIVATE_KEY_PATH:-secrets/jwt/private.pem}")"
JWT_PUBLIC_KEY_PATH="$(to_abs_path "${JWT_PUBLIC_KEY_PATH:-secrets/jwt/public.pem}")"
export JWT_PRIVATE_KEY_PATH JWT_PUBLIC_KEY_PATH

ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH="$(to_abs_path "${ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH:-secrets/internal-jwt/private.pem}")"
ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH="$(to_abs_path "${ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH:-secrets/internal-jwt/public.pem}")"
export ONEBOOK_INTERNAL_JWT_PRIVATE_KEY_PATH ONEBOOK_INTERNAL_JWT_PUBLIC_KEY_PATH

LOGS_DIR="$(to_abs_path "${LOGS_DIR:-backend/logs}")"
export LOGS_DIR
mkdir -p "$LOGS_DIR"

if [[ -z "${ONEBOOK_EMBEDDING_DIM:-}" ]]; then
  export ONEBOOK_EMBEDDING_DIM="3072"
fi

# Kill existing process on the port
echo "Stopping $SERVICE (port $PORT)..."
fuser -k "${PORT}/tcp" >/dev/null 2>&1 && echo "Stopped." || echo "Nothing running on port $PORT."

sleep 1

# Start service in foreground
echo "Starting $SERVICE..."
cd "$SVC_DIR"
export GOCACHE="$ROOT_DIR/backend/.cache/go-build"
exec go run "$CMD"
