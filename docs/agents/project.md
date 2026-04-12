# Project orientation

## What this repo is

**Spotube** is a self-hosted web app that bidirectionally syncs playlists between **Spotify** and **YouTube Music**. The stack is a **Go** backend (Echo, Goose migrations, Jet codegen, SQLite) and a **React 19** SPA (Vite, TanStack Router/Query, Tailwind v4).

There is **no Docker** in this repo: the database is a local SQLite file under `backend/data/`.

## Where things live

| Path | Purpose |
|------|---------|
| `backend/cmd/server/` | Go server entrypoint |
| `backend/cmd/migrate/` | Goose migration CLI |
| `backend/internal/` | Handlers, jobs, auth, config, HTTPS server |
| `backend/migrations/` | SQL migrations |
| `backend/internal/db/` | Jet-generated models and query builders (do not hand-edit table definitions here) |
| `frontend/src/routes/` | TanStack file-based routes |
| `frontend/src/lib/` | API client (`api.ts`), shared utilities |
| `frontend/e2e/` | Playwright E2E tests |
| `docs/rfcs/` | Feature RFCs and implementation notes |
| `docs/product_spec/` | Product requirements |
| `agents/skills/` | Optional agent skills (templates, workflows) |

For commands and daily workflow, see [Development workflow](dev-workflow.md).
