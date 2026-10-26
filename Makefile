WEB_DIST := backend/internal/web/dist

.PHONY: test lint frontend-install frontend-check frontend-dev backend-dev build demo

# Run all backend tests.
test:
	cd backend && go test -race ./...

# Format check and lint the backend.
lint:
	cd backend && golangci-lint fmt --diff && golangci-lint run ./...

frontend-install:
	cd frontend && npm ci

frontend/node_modules: frontend/package-lock.json
	cd frontend && npm ci
	touch frontend/node_modules

# Typecheck, lint, check the format of and build the frontend.
frontend-check: frontend/node_modules
	cd frontend && npm run typecheck && npm run lint && npm run format:check && npm run build

# Start the Vite dev server. It sends /api requests to the Go program.
frontend-dev: frontend/node_modules
	cd frontend && npm run dev

# Start the Go program for development, with the database at its default path.
backend-dev:
	cd backend && go run ./cmd/profit-tracker

# Build one binary with the frontend embedded: bin/profit-tracker.
build: frontend-check
	find $(WEB_DIST) -mindepth 1 ! -name .gitkeep -delete
	cp -R frontend/dist/. $(WEB_DIST)/
	cd backend && go build -o ../bin/profit-tracker ./cmd/profit-tracker

# Start the built binary with a fresh database of synthetic demo data.
demo: build
	scripts/demo.sh
