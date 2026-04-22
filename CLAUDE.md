# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project Overview

OneBook-AI is a "book conversation" app: users upload PDF/EPUB/TXT books, the system parses, chunks, and indexes them for vector + BM25 search, then enables Q&A against book content with Chinese language support, source citations, and grounding.

## Architecture

```
Frontend (React + Vite, :5173)
    └── Gateway (:8080) ── Unified entry, auth, rate limiting, routing
        ├── Auth (:8081) ── Signup/login/JWT/Refresh/admin/eval worker
        ├── Book (:8082) ── Book metadata, MinIO upload, state machine
        ├── Chat (:8083) ── RAG: dense+lexical retrieval → rerank → LLM
        ├── Ingest (:8084) ── File parsing & semantic chunking
        └── Indexer (:8085) ── Ollama embedding → Qdrant + OpenSearch

Infrastructure: Postgres, Redis, RabbitMQ, MinIO, Qdrant, OpenSearch, Ollama
Optional services: OCR (:8087), Reranker (:8088)
```

All external API calls go through Gateway. Internal service-to-service calls use short-lived service JWTs. Auth uses RS256 JWT with HttpOnly cookies + refresh token rotation.

## Key Commands

### Full startup
```bash
cp .env.example .env   # edit at minimum GENERATION_API_KEY
./run.sh               # starts all infra + backend services + frontend
```

### Backend
```bash
cd backend && go test ./...                          # run all tests
cd backend && go run ./cmd/check_openapi              # OpenAPI spec validation
cd backend/services/<service> && go run ./cmd/<svc>  # run single service manually
./scripts/restart-service.sh <service>                # hot-reload single service
```

### Frontend
```bash
cd frontend && npm install && npm run dev    # dev server
cd frontend && npm run lint                   # ESLint
cd frontend && npm run build                  # TypeScript + Vite build
cd frontend && npm run test:unit              # Vitest unit tests
```

### Docker
```bash
docker compose up -d                          # start infrastructure only
docker build -f backend/Dockerfile -t onebook-gateway --build-arg SERVICE=gateway --build-arg CMD=server backend
```

### RAG Evaluation
```bash
./scripts/run-rag-eval.sh                     # offline RAG eval pipeline
```

## Service Ports

| Service | Port | Service | Port |
|---|---|---|---|
| Gateway | 8080 | Ingest | 8084 |
| Auth | 8081 | Indexer | 8085 |
| Book | 8082 | Swagger UI | 8086 |
| Chat | 8083 | OCR | 8087 | Reranker | 8088 |

## Backend Structure

- **`backend/services/`** — 6 independent Go services (no web framework, stdlib `net/http`)
- **`backend/pkg/`** — Shared packages: `ai` (LLM interface), `auth` (JWT), `domain` (types), `queue` (RabbitMQ+PG), `retrieval` (dense+lexical+rerank), `storage` (MinIO), `store` (GORM)
- **`backend/internal/`** — Internal utilities: `eval`, `ratelimit`, `servicetoken`, `usertoken`, `util`
- **`backend/api/rest/`** — OpenAPI specs: `openapi.yaml` (external), `openapi-internal.yaml` (internal)

### Key patterns
- Go services use stdlib `net/http` — no Gin/Echo/Fiber
- GORM for database access, shared via `backend/pkg/store`
- RabbitMQ for async tasks (Ingest/Indexer), with Postgres task state tracking
- Shared domain types in `backend/pkg/domain`

## Frontend Structure

- **`frontend/src/pages/`** — Page components (ChatPage, LibraryPage, LoginPage, Admin*Pages)
- **`frontend/src/features/`** — Feature domains (auth, library, admin)
- **`frontend/src/shared/`** — Shared components, hooks, API client
- **`frontend/src/app/`** — Router and global providers

Key libraries: React 19, Vite 7, React Router v7, TanStack Query v5, Zustand v5, Tailwind CSS v4, Axios (withCredentials for cookie auth).

### Frontend-backend contract
- All requests → Gateway at `http://localhost:8080`
- Cookie auth via `withCredentials: true` (HttpOnly JWT)
- 401 triggers single-flight refresh, then replays original request
- Upload requires `Idempotency-Key` header
- Poll `GET /api/books/{id}` until status is `ready` or `failed`

## Development Rules

1. **Minimal changes**: understand existing code before editing; implement the smallest viable change
2. **No breaking changes** unless explicitly requested
3. **Validate**: `go test ./...` for backend, `npm run lint && npm run build` for frontend
4. **API changes**: update `backend/api/rest/openapi.yaml` (external) or `openapi-internal.yaml` (internal)
5. **Commits**: Conventional Commits with scope — `<type>(<scope>): <subject>`
   - Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `build`, `ci`, `perf`
   - Scopes: `frontend`, `auth`, `gateway`, `backend`, `api`, `docs`, `ci`, `infra`, `app`
