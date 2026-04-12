# Coding conventions

## Scope and style

- Keep changes focused on the task; avoid drive-by refactors and unrelated file churn.
- Match existing naming, imports, and patterns in the package you touch.
- Prefer extending existing helpers over duplicating similar logic.

## Backend (Go)

- **Database:** Use Jet-generated builders under `backend/internal/db/` for application reads/writes. Raw SQL belongs in Goose migrations under `backend/migrations/`, not in handlers (see RFCs and existing handlers for patterns).
- **HTTP:** Echo handlers take explicit dependency structs; return structured JSON errors, not stack traces.
- **Tests:** Prefer real SQLite (often in-memory) over mocking the DB; use `httptest` for handler tests.

## Frontend (React + TypeScript)

- **Data:** TanStack Query for server state; validate API responses where it matters (e.g. Zod).
- **Routing:** File-based routes under `frontend/src/routes/`; do not edit generated `routeTree.gen.ts` by hand.
- **API:** Use the shared HTTP client in `frontend/src/lib/api.ts` (cookies, CSRF, base URL from `VITE_API_URL`).
- **Tests:** Vitest + MSW for unit tests; Playwright only when the user asks for E2E.

## RFCs

Larger features should align with or update documents in `docs/rfcs/`. Use skills under `agents/skills/` (e.g. create-rfc-from-prd, implement-rfc) when appropriate.
