# The image of profit-tracker for Azure Container Apps. See ADR 0006.
# Build it from the repository root: docker build -t profit-tracker .

# Stage 1: build the frontend.
FROM node:26-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: build the Go binary with the frontend embedded, as `make build` does.
# The SQLite driver is pure Go, so the binary needs no C library.
# Keep the Go version the same as in backend/go.mod.
FROM golang:1.27.3-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /src/frontend/dist/ ./internal/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/profit-tracker ./cmd/profit-tracker
# The database folder. The final image has no shell to make it.
RUN mkdir /out/data

# Stage 3: the binary on distroless. The image has no shell and no package manager.
FROM gcr.io/distroless/static-debian13:nonroot
# The label connects the package in GHCR to the repository. The repository then controls the access.
LABEL org.opencontainers.image.source=https://github.com/JRojowski/profit-tracker
COPY --from=backend /out/profit-tracker /profit-tracker
COPY --from=backend --chown=nonroot:nonroot /out/data /data
ENV TZ=Europe/Warsaw \
    PROFIT_TRACKER_ADDR=0.0.0.0:8080 \
    PROFIT_TRACKER_DB=/data/profit-tracker.db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/profit-tracker"]
