# syntax=docker/dockerfile:1

# ---- Stage 1: build the React SPA ----
FROM node:22-alpine AS spa
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Same-origin bundle: the SPA uses relative /api for the REST API and the SSE stream
# (GET /api/events), so no VITE_BACKEND_URL is needed.
RUN npm run build
# -> /app/frontend/dist

# ---- Stage 2: build the static Go binary (embeds the SPA) ----
FROM golang:1.26 AS build
# VERSION is injected into main.version (S7); defaults to "dev" for un-stamped local builds.
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace the committed placeholder dist with the real SPA build, which //go:embed then
# bakes into the binary.
RUN rm -rf internal/staticfs/dist && mkdir -p internal/staticfs/dist
COPY --from=spa /app/frontend/dist/ internal/staticfs/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /statshed-server ./cmd/statshed-server
# An empty data dir to seed the volume mount point (distroless has no shell to mkdir at
# build, so create it here and COPY it --chown'd into the final image).
RUN mkdir -p /data

# ---- Final: distroless static, nonroot ----
FROM gcr.io/distroless/static:nonroot
# CA certs come from the base image; no shell, no package manager.
COPY --from=build /statshed-server /statshed-server
# Pre-create /data owned by nonroot (UID 65532) so the statshed-data volume initializes
# writable for SQLite — the db file, its -wal/-shm sidecars, AND the directory itself.
COPY --from=build --chown=65532:65532 /data /data
EXPOSE 7828
ENV HOST=0.0.0.0 \
    PORT=7828 \
    DATABASE_URL=sqlite:////data/statshed.db
USER 65532:65532
# AIDEV-NOTE: The binary migrates the fresh DB on startup (no Alembic, no entrypoint script)
# and serves the REST API, the SSE stream, and the embedded SPA. --healthcheck probes
# /api/health on the loopback (compose overrides this with its own healthcheck, Task 7.2).
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/statshed-server", "--healthcheck"]
ENTRYPOINT ["/statshed-server"]
