# OneBook-AI Project Instructions

Scope:
- Applies to all work under `/Users/jacl/Documents/Code/Golang/OneBook-AI`.

Core principles:
- Prefer security, maintainability, testability, and behavior consistency.
- Prefer small, targeted changes over broad refactors.
- Keep external API behavior predictable; if behavior changes, update docs in the same task.

Execution standard:
- Understand existing behavior before editing.
- Implement the minimal viable change to satisfy the request.
- Do not introduce breaking changes unless explicitly requested.
- Avoid destructive git operations (for example `reset --hard`) unless explicitly requested.

Validation standard:
- Backend changes: run `go test ./...` in `backend/`.
- Frontend changes: run `npm run lint` and `npm run build` in `frontend/`.
- If any check is skipped or fails, report it explicitly with risk.

API and error semantics:
- Keep response format consistent across layers.
- For API error/status semantic changes, update OpenAPI docs by scope:
  - External API changes: `backend/api/rest/openapi.yaml`
  - Internal service API changes: `backend/api/rest/openapi-internal.yaml`
  - If both scopes are affected, update both files
- Also update relevant docs under `docs/` and frontend parsing logic if affected.

Commit convention (Conventional Commits):
- Use standard types only: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `build`, `ci`, `perf`.
- Type guidance:
  - `feat`: user-facing capability or API behavior addition/change.
  - `fix`: bug fix with behavior correction.
  - `refactor`: internal restructuring without behavior change.
