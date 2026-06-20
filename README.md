# StatShed Server

[![CI](https://github.com/statshed/statshed-server/actions/workflows/ci.yml/badge.svg)](https://github.com/statshed/statshed-server/actions/workflows/ci.yml)
[![License: CC0-1.0](https://img.shields.io/badge/license-CC0--1.0-lightgrey.svg)](LICENSE)

**StatShed** is a lightweight, real-time status dashboard for your jobs. Cron jobs,
CI pipelines, backups, and cluster tasks POST their status to StatShed, and a live web
dashboard shows health per group — successes, failures, things still running, and jobs
that have timed out or gone stale — updating in real time over WebSockets.

This repository, **`statshed-server`**, contains everything you need to run StatShed:

- **`backend/`** — a Flask REST API + Socket.IO server (Python 3.13), persisting to SQLite. It also serves the built frontend.
- **`frontend/`** — a React + Vite single-page dashboard, built and bundled into the backend image.

A single `statshed-server` Docker image serves the dashboard, the REST API, and the live WebSocket. One `docker compose up` brings it online.

```
                ┌──────────────────────────────────────┐
   browser ────▶│  statshed-server                     │
   :7827        │  Flask + Socket.IO  (serves SPA,      │
                │  /api REST, /socket.io WebSocket)     │
   CLI ────────▶│  SQLite @ /data                       │
   POST /api    └──────────────────────────────────────┘
                                  :7827
```

> **Heads-up on security:** StatShed has **no authentication** by design — it's meant for
> trusted/internal networks. To expose it publicly, put it behind a reverse proxy that
> adds authentication and TLS.

---

## Quick start (no build required)

The fastest way to run a released version — Docker pulls prebuilt images, nothing is
compiled locally. You need [Docker](https://docs.docker.com/get-docker/) with Compose.

```bash
# Grab the two files from the latest release
mkdir statshed && cd statshed
curl -LO https://github.com/statshed/statshed-server/releases/latest/download/docker-compose.yml
curl -LO https://github.com/statshed/statshed-server/releases/latest/download/.env.example

# Configure and launch
cp .env.example .env
# (edit .env to set SECRET_KEY — see "Configuration" below)
docker compose up -d
```

Then open **<http://localhost:7827>**. To pin a specific version, set `STATSHED_VERSION`
(e.g. `STATSHED_VERSION=v0.1.0`) in `.env`.

## Quick start (from source)

Clone the repo and build the images locally:

```bash
git clone https://github.com/statshed/statshed-server.git
cd statshed-server
cp .env.example .env        # set SECRET_KEY
docker compose up --build -d
```

Open **<http://localhost:7827>**. (`make up` is a shortcut for the compose command.)

## Submit your first status

With the stack running, report a job status to the backend API and watch it appear on the
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

Run each side natively for hot reload. You'll need [uv](https://docs.astral.sh/uv/) and
Node.js 20+.

```bash
# Terminal 1 — backend on :7828
cd backend
uv sync
uv run python app.py

# Terminal 2 — frontend on :7827 (Vite proxies /api and /socket.io to :7828)
cd frontend
npm ci
npm run dev
```

Open <http://localhost:7827>. See [CONTRIBUTING.md](CONTRIBUTING.md) for the full dev loop
and the checks CI runs.

## Configuration

Copy [`.env.example`](.env.example) to `.env` and adjust. The most common settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `STATSHED_PORT` | `7827` | Host port for the dashboard, REST API, and WebSocket (CLI clients submit status here). |
| `SECRET_KEY` | _(random)_ | Flask secret key. **Set a strong value** for any shared deployment: `openssl rand -hex 32`. An empty value generates a random key on each restart. |
| `DATABASE_URL` | `sqlite:////data/statshed.db` | Database connection. SQLite (in the `statshed-data` volume) by default; set a `postgresql://…` URL for multi-instance deployments. |
| `DEBUG` | `false` | Flask debug mode. Keep `false` outside local development. |
| `STATSHED_VERSION` | `latest` | _(release compose only)_ image tag to pull from `ghcr.io/statshed`. |

> SQLite runs the backend as a single worker (required for WAL consistency). For
> multi-worker / horizontally-scaled deployments, switch `DATABASE_URL` to PostgreSQL.

Data persists in the Docker named volume **`statshed-data`** (`docker volume ls`). Removing
it deletes all stored jobs and configuration.

## Architecture

- **Frontend** (`frontend/`): React 19 + Vite + TypeScript + Tailwind, built to static
  assets and bundled into the backend image. Flask serves the SPA at `/`, so the browser
  always talks to one origin — **no CORS configuration needed** in the bundled deployment.
- **Backend** (`backend/`): Flask + gunicorn (gevent worker) serving the React SPA, a JSON
  REST API under `/api`, and a Socket.IO endpoint at `/socket.io` for live updates.
  SQLAlchemy models with Alembic migrations that run automatically on container start. A
  background task promotes `progress` jobs to `timeout` and `success` jobs to `stale` based
  on configurable thresholds.
- **CLI clients** (separate repos under the [statshed org](https://github.com/statshed)):
  the `statshed` command-line tool POSTs status to `/api/status` on the single port.

Design docs live in [`docs/`](docs/) ([overall](docs/design.md),
[backend](docs/design-backend.md), [frontend](docs/design-frontend.md),
[REST API](docs/restapi.md), [WebSocket/frontend API](docs/frontend-api.md)).

## Repository layout

```
statshed-server/
├── backend/                  # Flask API + Socket.IO server + SPA host (Python 3.13, uv)
├── frontend/                 # React + Vite dashboard (built into the backend image)
├── docs/                     # Design docs + API reference
├── docker-compose.yml        # Single statshed service, built from source (contributors)
├── docker-compose.release.yml# Single statshed service from prebuilt ghcr.io image (end users)
├── .env.example              # Configuration template
├── Makefile                  # Convenience targets — `make help`
└── .github/workflows/        # CI (lint/test/build) and Release (image + GitHub Release)
```

## Running the tests

```bash
# Backend
cd backend && uv run pytest

# Frontend unit tests + end-to-end (Playwright)
cd frontend && npm run test:ci && npm run test:e2e
```

Or `make test` (unit) and `make e2e`. CI runs all of these on every push and PR.

## Cutting a release (maintainers)

Releases are tag-driven. Pushing a semver tag builds multi-arch images, pushes them to
GHCR, and publishes a GitHub Release with the compose file attached:

```bash
git tag v0.1.0
git push origin v0.1.0
```

> **One-time:** after the first release, set the `statshed-server` GHCR package to
> **public** (*Package settings → Change visibility*) so anyone can `docker pull` it, and
> link it to this repository for inherited visibility.

## License

StatShed is dedicated to the public domain under
[CC0 1.0 Universal](LICENSE) — use it for anything, no attribution required.
