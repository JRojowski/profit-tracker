# Repository map

This file answers "Where is X?". It names the location of each part of the code. It holds no code rules.
Read AGENTS.md first.

## Quick lookup

| You need | Go to |
|---|---|
| The program entry point and the flags | `backend/cmd/profit-tracker/main.go`, `backend/cmd/profit-tracker/config.go` |
| The list of HTTP routes | `backend/internal/httpapi/handler.go` |
| An HTTP handler | `backend/internal/httpapi/<resource>.go` |
| The functions that the Assistant can call | `backend/internal/httpapi/assistant_functions.go` |
| The Assistant loop and its instructions | `backend/internal/assistant/` |
| The OpenRouter model client | `backend/internal/openrouter/` |
| The database schema | `backend/internal/store/migrations/` |
| A SQL query (sqlc) | `backend/internal/store/queries/*.sql` |
| The sqlc configuration | `backend/sqlc.yaml` |
| The XTB and Kraken export readers | `backend/internal/xtb/`, `backend/internal/kraken/` |
| The Import preview and save | `backend/internal/importer/` |
| The price provider (Yahoo) | `backend/internal/price/yahoo/` |
| The Exchange Rate provider (NBP) | `backend/internal/exchange/nbp/` |
| The frontend routes | `frontend/src/App.tsx`, `frontend/src/lib/routes.ts` |
| The frontend API client | `frontend/src/lib/api.ts` |
| A page of the frontend | `frontend/src/pages/` |
| A shadcn/ui component | `frontend/src/components/ui/` |
| The screenshot test fixtures | `frontend/screenshots/fixtures.ts` |
| The domain terms | `GLOSSARY.md` |
| The decisions | `docs/adr/` |

## Top level

| Path | Holds |
|---|---|
| `.claude/` | The project agents, skills, hooks and settings of Claude Code. |
| `.github/` | The CI and deployment workflows, and the Dependabot configuration. |
| `backend/` | The Go module: the HTTP API, the domain calculations and the SQLite store. |
| `docs/` | The ADRs, the agent documents, the v3 mockups, the manual checklists and the deployment document. |
| `frontend/` | The Vite + React + TypeScript + shadcn/ui application. |
| `infra/` | The Bicep templates for Azure. |
| `scripts/` | The scripts of the Makefile: `demo.sh`, `gate.sh` (the stack selection of the quality gate), `setup-azure.sh`. |

Root files: `Makefile` (all commands), `Dockerfile`, `.dockerignore`, `.gitignore`, `AGENTS.md`, `CLAUDE.md`, `GLOSSARY.md`.

## Modules

### backend

| Path | Purpose |
|---|---|
| `backend/cmd/profit-tracker/` | The main program. It reads the flags and the environment and starts the HTTP server. |
| `backend/cmd/demo-exports/` | Writes the synthetic demo exports. |
| `backend/internal/httpapi/` | The JSON HTTP API under `/api`. It also serves the built frontend. |
| `backend/internal/store/` | Keeps the data in one SQLite file. Holds the migrations and the sqlc queries. |
| `backend/internal/importer/` | Makes an Import preview from a Provider export and saves the confirmed Import. |
| `backend/internal/xtb/` | Reads the xlsx export of one XTB Account. |
| `backend/internal/kraken/` | Reads the ledger export (CSV) of Kraken. |
| `backend/internal/ledger/` | Calculates the state of one Account from its Ledger, the prices and the Exchange Rates. |
| `backend/internal/lot/` | Builds the Lots of an Account from the Fills of its buy orders. |
| `backend/internal/returns/` | Calculates the Simple Return and the Annual Return. |
| `backend/internal/allaccounts/` | Calculates the view of all Accounts together in PLN. |
| `backend/internal/allocation/` | Calculates the Allocation of one Account or of all Accounts. |
| `backend/internal/valuehistory/` | Calculates the Value History of one Account. |
| `backend/internal/benchmark/` | Holds the Benchmarks and calculates the Benchmark Return of a period. |
| `backend/internal/tier/` | Holds the Tiers of Rebalancing and their configuration rules. |
| `backend/internal/rebalancing/` | Calculates the Rebalancing table. |
| `backend/internal/ikze/` | Calculates the IKZE Contributions of each year against the IKZE Limit. |
| `backend/internal/price/` | Gets the current prices and the price history from a price provider and stores them. |
| `backend/internal/pricecheck/` | Finds the Price Warnings of an Instrument. |
| `backend/internal/exchange/` | Gets the NBP table A Exchange Rates and stores them. |
| `backend/internal/assistant/` | The conversation loop of the Assistant. |
| `backend/internal/openrouter/` | The `assistant.Model` that uses the OpenRouter chat completions endpoint. |
| `backend/internal/demo/` | Holds the synthetic XTB and Kraken exports for a demo database. |
| `backend/internal/web/` | Embeds the built frontend in the binary. `make build` fills `dist/`. |

### frontend

| Path | Purpose |
|---|---|
| `frontend/src/main.tsx`, `frontend/src/App.tsx` | The entry point and the router. |
| `frontend/src/pages/` | One file for each route. |
| `frontend/src/components/` | The application components. |
| `frontend/src/components/ui/` | The shadcn/ui components. |
| `frontend/src/hooks/` | The React hooks (`use-*.ts`). |
| `frontend/src/lib/` | The API client, the app state, the formatting and the table and chart helpers. |
| `frontend/screenshots/` | The Playwright screenshot tests, the fake backend and the fixtures. |
| `frontend/screenshots/images/` | The stored images of the screenshot tests (macOS). |

## Conventions inside a module

| Kind | Where | Example |
|---|---|---|
| HTTP route | `backend/internal/httpapi/handler.go` | `mux.HandleFunc("GET /api/health", a.health)` |
| HTTP handler | `backend/internal/httpapi/<resource>.go` | `backend/internal/httpapi/tiers.go` |
| Assistant function | `backend/internal/httpapi/assistant_<topic>.go` | `backend/internal/httpapi/assistant_holdings.go` |
| Migration | `backend/internal/store/migrations/NNNN_<name>.sql` | `backend/internal/store/migrations/0001_accounts.sql` |
| SQL query | `backend/internal/store/queries/<table>.sql`, generated `<table>.sql.go` | `backend/internal/store/queries/accounts.sql` |
| Provider client | a subpackage of the service | `backend/internal/price/yahoo/`, `backend/internal/exchange/nbp/` |
| Page | `frontend/src/pages/<name>-page.tsx` | `frontend/src/pages/rebalancing-page.tsx` |
| Component | `frontend/src/components/<kebab-name>.tsx` | `frontend/src/components/holdings-table.tsx` |
| Hook | `frontend/src/hooks/use-<name>.ts` | `frontend/src/hooks/use-load.ts` |
| Decision | `docs/adr/NNNN-<title>.md` | `docs/adr/0002-lot-composition-from-fills.md` |

## Tests

- Go: each `<file>.go` has its test in `<file>_test.go` in the same directory. An internal test uses `<file>_internal_test.go`.
- Go test data: `testdata/` next to the test, for example `backend/internal/price/yahoo/testdata/`.
- The HTTP API tests are in `backend/internal/httpapi/*_test.go`. They are the most numerous tests.
- The real-export tests (`TestRealExports`) read `exports/`. They skip when `exports/` has no xlsx file.
- Frontend: no unit tests. The screenshot tests in `frontend/screenshots/*.spec.ts` cover each page.
- CI: `.github/workflows/ci.yml` runs the backend tests, the backend lint, the frontend check and the Bicep lint.
