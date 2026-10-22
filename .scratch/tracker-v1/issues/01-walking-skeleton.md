# 01: Walking skeleton

**What to build:** The user runs one Go binary and opens the tool on localhost. The page shows the empty React application. The backend has a SQLite database with migrations and a health endpoint. The test harness can start the backend with a temporary database, call the HTTP API, and build small synthetic xlsx files in the XTB export layout. See `.scratch/tracker-v1/spec.md`, sections Architecture and Testing Decisions.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [x] The Go module and the Vite + React + TypeScript + shadcn/ui frontend exist in one repository, in separate folders.
- [x] The production build embeds the built frontend in the Go binary. The binary serves the frontend and the `/api` routes on one port.
- [x] In development, the Vite dev server sends `/api` requests to the Go program.
- [x] The database path comes from configuration, with a default in the user's data directory. Migrations run at start.
- [x] `GET /api/health` returns status 200.
- [x] An API test starts the backend with a new temporary SQLite file and calls `GET /api/health`.
- [x] A test helper builds an xlsx file with the three XTB sheets (Closed Positions, Cash Operations, Open Positions) and their header rows, from rows that the test gives.
- [x] One command runs all backend tests. One command runs the frontend typecheck and build. The commands are in `CLAUDE.md`.
