# StatShed Server

[![CI](https://github.com/statshed/statshed-server/actions/workflows/ci.yml/badge.svg)](https://github.com/statshed/statshed-server/actions/workflows/ci.yml)
[![License: CC0-1.0](https://img.shields.io/badge/license-CC0--1.0-lightgrey.svg)](LICENSE)

**StatShed** is a lightweight, real-time status dashboard for your jobs. Cron jobs,
CI pipelines, backups, and cluster tasks POST their status to StatShed, and a live web
dashboard shows health per group — successes, failures, things still running, and jobs
that have timed out or gone stale — updating in real time over Server-Sent Events.

This repository, **`statshed-server`**, contains everything you need to run StatShed:

- the **server** (`cmd/` + `internal/`) — a single Go binary that serves the REST API, the
  live event stream, and the dashboard, persisting to SQLite.
- the **frontend** (`frontend/`) — a React + Vite single-page dashboard, built and embedded
  into the binary.

A single `statshed-server` Docker image serves the dashboard, the REST API, and the live
event stream. One `docker compose up` brings it online.

```
                ┌──────────────────────────────────────┐
   browser ────▶│  statshed-server (Go)                │
   :7827        │  serves the SPA, the /api REST API,   │
                │  and the /api/events SSE stream       │
   CLI ────────▶│  SQLite @ /data                       │
   POST /api    └──────────────────────────────────────┘
                                  :7827
```

> **Heads-up on security:** StatShed has **no authentication** by design — it's meant for
> trusted/internal networks. To expose it publicly, put it behind a reverse proxy that
> adds authentication and TLS.

---

## Quick start (no build required)

The fastest way to run a released version — Docker pulls a prebuilt image, nothing is
compiled locally. You need [Docker](https://docs.docker.com/get-docker/) with Compose.

```bash
# Grab the two files from the latest release
mkdir statshed && cd statshed
curl -LO https://github.com/statshed/statshed-server/releases/latest/download/docker-compose.yml
curl -LO https://github.com/statshed/statshed-server/releases/latest/download/.env.example

# Configure and launch
cp .env.example .env
docker compose up -d
```

Then open **<http://localhost:7827>**. To pin a specific version, set `STATSHED_VERSION`
(e.g. `STATSHED_VERSION=v0.1.0`) in `.env`.

## Quick start (from source)

Clone the repo and build the image locally:

```bash
git clone https://github.com/statshed/statshed-server.git
cd statshed-server
cp .env.example .env
docker compose up --build -d
```

Open **<http://localhost:7827>**. (`make up` is a shortcut for the compose command.)

## Submit your first status

With the stack running, report a job status to the server's API and watch it appear on the
dashboard **instantly** (no refresh):

```bash
curl -X POST http://localhost:7827/api/status \
  -H 'Content-Type: application/json' \
  -d '{"group":"demo","job":"hello-world","status":"success","message":"It works!"}'
```

`status` is one of `success`, `error`, `progress`, `timeout`, or `stale`. See
[docs/restapi.md](docs/restapi.md) for the full API.

In real life you'd use the **`statshed` CLI** (a small Go binary, also under the
[statshed org](https://github.com/statshed)) from your cron jobs and scripts rather than
raw `curl`.

---

## Local development (without Docker)

You'll need [Go](https://go.dev/dl/) 1.26+ and Node.js 20+. `make dev` builds the SPA into
the binary and runs the whole app on `:7828`:

```bash
make dev
```

For frontend hot-reload, run the Go API and the Vite dev server side by side:

```bash
# Terminal 1 — API only on :7828 (Vite serves the SPA in dev)
STATIC_DISABLED=1 DATABASE_URL=sqlite:///dev.db go run ./cmd/statshed-server

# Terminal 2 — frontend on :7827 (Vite proxies /api to :7828)
cd frontend
npm ci
npm run dev
```

Open <http://localhost:7827>. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full dev loop
and the checks CI runs.

> **Fresh-DB-only:** the server creates and migrates an **empty** SQLite database on first
> start and refuses a pre-existing one. In dev, point `DATABASE_URL` at a throwaway path and
> remove it between runs (`rm -f dev.db*`).

## Configuration

Copy [`.env.example`](.env.example) to `.env` and adjust. The most common settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `STATSHED_PORT` | `7827` | Host port for the dashboard, REST API, and event stream (CLI clients submit status here). |
| `DATABASE_URL` | `sqlite:////data/statshed.db` | SQLite database path (sqlite-only). Persisted in the `statshed-data` volume. **Fresh-DB-only:** an empty DB is created and migrated on first start; a pre-existing or incompatible DB is rejected. |
| `DEBUG` | `false` | `true` raises the `log/slog` level to DEBUG (verbose request logging). Keep `false` in production. |
| `SECRET_KEY` | _(unused)_ | Accepted but **ignored** by the Go server — kept only so a `.env` carried over from the legacy Python image still works. |
| `STATSHED_VERSION` | `latest` | _(release compose only)_ image tag to pull from `ghcr.io/statshed`. |

See [`.env.example`](.env.example) for the rest (`CORS_ORIGINS`, `LOG_UPLOAD_ENABLED`,
`MAX_LOG_LINES`, `MAX_JOBS_PAGE_SIZE`, `STATIC_DIR`, `STATIC_DISABLED`).

Data persists in the Docker named volume **`statshed-data`** (`docker volume ls`). Removing
it deletes all stored jobs and configuration.

## Architecture

- **Frontend** (`frontend/`): React 19 + Vite + TypeScript + Tailwind, built to static
  assets and **embedded into the Go binary**. The server serves the SPA at `/`, so the
  browser always talks to one origin — **no CORS configuration needed** in the bundled
  deployment.
- **Server** (`cmd/` + `internal/`): a single static Go binary (chi router; pure-Go
  `modernc.org/sqlite`, no CGO). It serves the SPA, a JSON REST API under `/api`, and a
  Server-Sent Events stream at `/api/events` for live updates. The schema is migrated with
  goose on an empty database at startup. A background worker promotes `progress` jobs to
  `timeout` and `success` jobs to `stale` on configurable thresholds, and expires old jobs.
  Writes are serialized on a single SQLite connection (WAL); reads run concurrently.
- **CLI clients** (separate repos under the [statshed org](https://github.com/statshed)):
  the `statshed` command-line tool POSTs status to `/api/status` on the single port.

The [REST API reference](docs/restapi.md) (including the real-time SSE events) is the most
useful starting point; other design notes live in [`docs/`](docs/).

## Repository layout

```
statshed-server/
├── cmd/statshed-server/       # Go server entrypoint (main)
├── internal/                  # server packages: api, store, realtime, background, config, staticfs
├── frontend/                  # React + Vite dashboard (built + embedded into the binary)
├── contract/                  # black-box HTTP contract / regression suite
├── docs/                      # API reference + design docs
├── Dockerfile                 # multi-stage build → one distroless image
├── docker-compose.yml         # single statshed service, built from source (contributors)
├── docker-compose.release.yml # single statshed service from prebuilt ghcr.io image (end users)
├── .env.example               # configuration template
├── Makefile                   # convenience targets — `make help`
└── .github/workflows/         # CI (build/test) and Release (image + binaries + GitHub Release)
```

## Running the tests

```bash
# Go server (unit tests with the race detector)
go test -race ./...            # or: make test

# Frontend unit tests + end-to-end (Playwright)
cd frontend && npm run test:ci && npm run test:e2e

# HTTP contract suite against the Go server (one profile shown; CI runs all six)
make contract-test TARGET=go
```

CI runs all of these — plus `golangci-lint`, the live-SSE gate, and an image smoke test —
on every push and PR.

## Cutting a release (maintainers)

Releases are tag-driven. Pushing a semver tag builds the multi-arch image, pushes it to
GHCR, and publishes a GitHub Release with standalone `linux/amd64` + `linux/arm64` binaries
(SPA embedded), `SHA256SUMS`, and the compose file attached:

```bash
git tag v0.1.0
git push origin v0.1.0
```

> **One-time:** after the first release, set the `statshed-server` GHCR package to
> **public** (*Package settings → Change visibility*) so anyone can `docker pull` it, and
> link it to this repository for inherited visibility.

> **Upgrading from the legacy Python server:** the Go server is fresh-DB-only and does not
> adopt a Python-created `statshed.db`. Point it at a **fresh** `statshed-data` volume (or
> remove the old `statshed.db`) before first start; existing job rows are intentionally not
> carried over (they are ephemeral, ~24 h expiry).

## License

StatShed is dedicated to the public domain under
[CC0 1.0 Universal](LICENSE) — use it for anything, no attribution required.
