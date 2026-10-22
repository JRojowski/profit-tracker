# profit-tracker

## Commands

Run these commands from the repository root.

| Task | Command |
|---|---|
| Run all backend tests | `make test` |
| Lint the backend | `make lint` |
| Install the frontend packages | `make frontend-install` |
| Typecheck and build the frontend | `make frontend-check` |
| Build one binary with the embedded frontend | `make build` (output: `bin/profit-tracker`) |
| Start the backend for development | `make backend-dev` |
| Start the frontend for development | `make frontend-dev` (Vite sends `/api` to `PROFIT_TRACKER_ADDR`, default `127.0.0.1:8080`) |

The binary reads the flags `-addr` and `-db`, or the variables `PROFIT_TRACKER_ADDR` and `PROFIT_TRACKER_DB`. The default database is `profit-tracker/profit-tracker.db` in the user configuration directory.

## Layout

- `backend/`: the Go module.
- `frontend/`: the Vite + React + TypeScript + shadcn/ui application.

## Go backend

- Delegate Go implementation work to the `go-developer` subagent.
- When you write or review Go code yourself, load the `go-code-standard` skill first.

## Agent skills

### Issue tracker

Issues and specs live as local markdown files under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

The tracker uses the five default triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
