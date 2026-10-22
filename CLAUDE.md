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

## Second review with Codex

Codex gives a second review from a different model. Do this review for each change to code, before you commit.

1. Start the Codex review in the background. Then do your own review, for example with the `code-review` skill.

   ```bash
   codex exec review --uncommitted --ephemeral -o <scratchpad>/codex-review.md > /dev/null 2>&1
   ```

   The review takes some minutes. The `-o` file holds only the verdict. The full output is a long trace, so send it to `/dev/null`.
2. Treat each Codex finding as a claim. Read the code it names and confirm or reject the finding.
3. Fix each confirmed finding, test-first. Go fixes go to the `go-developer` subagent.
4. In the final report to the user, list each Codex finding with its result: fixed, or rejected with the reason.

The review is complete when each Codex finding has a result. If Codex fails (for example, the login expired), tell the user and continue without it.

## Agent skills

### Issue tracker

Issues and specs live as local markdown files under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

The tracker uses the five default triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
