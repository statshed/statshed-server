# Design: single `statshed-server` Docker image

- **Date:** 2026-06-19
- **Status:** Approved (pending written-spec review)
- **Topic:** Collapse the two-image release (backend + frontend/nginx) into one
  `statshed-server` image that serves the API, the WebSocket, and the React SPA
  from a single process.

## Context: current state

StatShed ships as **two** Docker images, built and pushed to GHCR by
`.github/workflows/release.yml` on any `v*.*.*` tag:

- `statshed-backend` — Flask + flask-socketio on gevent, run by gunicorn
  (`-w 1 -k geventwebsocket...GeventWebSocketWorker`, bind `:7828`). SQLAlchemy
  + Alembic + SQLite. ~1,885 lines in `backend/app.py`.
- `statshed-frontend` — nginx serving the built React/Vite SPA (`dist/`, ~600 KB)
  and reverse-proxying `/api/*` and `/socket.io/` to the backend.

`docker-compose.yml` / `docker-compose.release.yml` wire the two together; the
browser only ever talks to the nginx origin (same-origin), and nginx adds the
security headers + CSP, gzip, and asset caching.

**No version has ever been released** — `git tag -l` is empty, so the pipeline
has never run and nothing is deployed in the wild. This means contract changes
(ports, paths) break nothing downstream.

### Key facts that shaped the design

- The frontend calls the API at relative `/api/...`
  (`frontend/src/api/client.ts:44` → `fetch(\`/api${endpoint}\`)`) and connects
  Socket.IO same-origin at `/socket.io` (`BACKEND_URL` defaults to `''`).
- The backend's API routes currently live at **root** (`/jobs`, `/status`,
  `/health`, `/groups`, `/config`, `/admin/*` — ~16 routes). nginx strips the
  `/api` prefix (`proxy_pass ${BACKEND_URL}/`).
- The SPA has client-side routes `/`, `/jobs`, `/groups/:groupName`, `/settings`
  (`frontend/src/App.tsx:40-45`). **`/jobs` collides** with the API's
  `GET /jobs` — so the API cannot remain at root once Flask also serves the SPA.
- The backend test suite makes **371** requests to root paths, but the `client`
  fixture is defined in exactly one place (`backend/tests/conftest.py:60-63`).
- No `url_for(...)` calls exist in the backend, so renaming endpoints into a
  Blueprint is safe.

## Goal

One multi-arch image, `ghcr.io/statshed/statshed-server`, that runs the whole
app from a single process on a single port. Remove nginx. Update the release
pipeline, compose files, and docs to match.

## Chosen approach

**Flask serves everything** (the recommended "Approach 1"). The existing gunicorn
+ gevent-websocket worker serves the SPA, the REST API, and the WebSocket. nginx
is deleted; its responsibilities move into Flask.

Routing in the unified app:

| Path | Handled by |
|------|------------|
| `/`, `/jobs`, `/settings`, `/groups/:name`, `/assets/*`, unknown paths | SPA: static files from `dist/`, fallback to `index.html` |
| `/api/*` | REST API (moved behind an `/api` Blueprint) |
| `/socket.io` | flask-socketio (unchanged; intercepted by the engineio WSGI middleware before Flask routing) |

### Confirmed decisions

1. **Single port `7827`.** The container keeps its internal gunicorn bind
   (`:7828`); compose publishes `7827:7828`. Browser users keep
   `http://localhost:7827`. (Internal port is invisible; could be unified to 7827
   later, not required.)
2. **Client ingest contract moves to `/api` on the single port.** CLI clients
   change from `POST http://host:7828/status` to
   `POST http://host:7827/api/status`. This is a deliberate, breaking change to
   the documented ingest URL; acceptable because nothing is released yet.

## Detailed design

### Backend (`backend/`)

1. **API → `/api` Blueprint.** Introduce `api = Blueprint("api", __name__)`.
   Change the ~16 `@app.route(...)` view decorators to `@api.route(...)`, then
   `app.register_blueprint(api, url_prefix="/api")`. Leave on `app` (global):
   the two `@app.errorhandler`s, `@app.teardown_appcontext`, all `@socketio.on`
   handlers, and module-level setup (CORS, SocketIO, `start_timeout_checker`).
   `/health` moves under the prefix too → `/api/health`.

2. **SPA serving.** Add a catch-all that serves `dist/` with an `index.html`
   fallback:
   - Static dir resolved from `STATIC_DIR` env, defaulting to
     `os.path.join(os.path.dirname(__file__), "static")` (where the Dockerfile
     copies `dist/`).
   - `@app.route("/", defaults={"path": ""})` + `@app.route("/<path:path>")`:
     return the requested file if it exists under the static dir, else
     `index.html`. The `/api/*` Blueprint rules and `/socket.io` take precedence
     (more specific / middleware-intercepted), so the catch-all only sees SPA
     traffic.
   - **Conditional:** if the static dir does not exist (local dev via
     `python app.py`, with Vite serving the SPA separately), the catch-all is not
     registered, so dev behavior is unchanged.

3. **Security parity with nginx** (via `@app.after_request`):
   - Headers: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
     `Referrer-Policy: strict-origin-when-cross-origin`, and the **exact** CSP
     from `nginx.conf.template`, including the inline-script hash
     `'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='`. Carry over the
     AIDEV-NOTE explaining how to recompute the hash if the inline theme
     bootstrap in `index.html` changes.
   - Cache-control: hashed assets under `/assets/*` get
     `Cache-Control: public, immutable, max-age=31536000`; `index.html` gets
     `no-cache` so deploys are picked up.
   - These headers must be applied to API and static responses alike (matching
     nginx's `always`).

4. **gzip.** Add `flask-compress` to `pyproject.toml` and `Compress(app)` to
   replace nginx's gzip (assets ~600 KB → ~150 KB). Low risk; could be deferred
   but cheap to include.

5. **Tests.** In `conftest.py`, change the `client` fixture to return a thin
   `FlaskClient` subclass whose `open()` prepends `/api` to request paths unless
   they target `/socket.io` or already start with `/api`. This keeps all 371
   existing call sites unchanged. Add focused tests:
   - `GET /` returns the SPA HTML (when a static dir is present).
   - `GET /jobs` returns the SPA HTML, **not** API JSON (collision regression).
   - `GET /api/health` works.
   - Socket.IO test client still connects.
   - Security headers + CSP present on a representative response.

6. **CORS config** (`config.py`) stays as-is. It's a no-op for the unified
   same-origin deployment but remains useful for dev (`VITE_BACKEND_URL` direct
   connection). No change required.

### Dockerfile (single, multi-stage, build context = repo root)

Replace `backend/Dockerfile` and `frontend/Dockerfile` with one `Dockerfile`
(repo root):

1. **Stage `builder` (`node:22-alpine`):** `COPY frontend/package*.json`,
   `npm ci`, `COPY frontend/`, `npm run build` → `/app/dist`. No
   `VITE_BACKEND_URL` build arg (same-origin; not needed).
2. **Stage `runtime` (`python:3.13-slim`):** install `uv`, `COPY
   backend/pyproject.toml backend/uv.lock`, `uv sync --frozen --no-dev
   --no-install-project`, `COPY backend/`, `uv sync --frozen --no-dev`,
   `COPY --from=builder /app/dist ./static`. Keep installing `curl`
   (`apt-get install --no-install-recommends curl`) for the compose healthcheck,
   as the current backend image does. Keep `entrypoint.sh`
   (`alembic upgrade head` then exec). Keep `CMD` gunicorn single
   gevent-websocket worker binding `:7828`.

Add a repo-root `.dockerignore` (the build context is now the whole repo):
exclude `.git`, `**/node_modules`, `**/.venv`, `**/dist`, caches, `*.db`, etc.

Remove `frontend/nginx.conf.template` (logic now in Flask).

### Release pipeline (`.github/workflows/release.yml`)

- Collapse the build matrix from two entries to **one**:
  `ghcr.io/statshed/statshed-server`, context `.`, multi-arch
  `linux/amd64,linux/arm64` (unchanged), same GHCR login / metadata / cache.
- Update the Release body to reference the single image and the single-service
  compose.

### Compose

- `docker-compose.yml`: one `statshed` service — `build: .`,
  `image: statshed-server:local`, `ports: ["${STATSHED_PORT:-7827}:7828"]`
  (distinct name so it does not collide with the app's own `PORT`/`Config.PORT`,
  which only the dev runner reads), the `statshed-data` volume, env
  (`DATABASE_URL`, `SECRET_KEY`, `DEBUG`), healthcheck
  `curl -f http://127.0.0.1:7828/api/health`. Drop
  `BACKEND_URL`/`VITE_BACKEND_URL`/`CORS_ORIGINS` wiring and the
  `depends_on`/second service.
- `docker-compose.release.yml`: same single service, `image:
  ghcr.io/statshed/statshed-server:${STATSHED_VERSION:-latest}`.

### CI (`.github/workflows/ci.yml`)

- Keep the existing `backend` and `frontend` test jobs (they still test each
  side independently).
- Optional: add a `smoke` job that `docker build`s the unified image, runs it,
  and curls `/` (expects SPA HTML) and `/api/health` (expects 200). Nice-to-have;
  can be deferred.

### Docs

- `README.md`: replace the two-box architecture diagram with a single-box one;
  update the ports section and the CLI ingest URL (`/api/...` on `7827`).
- `backend/README.md` / `frontend/README.md`: note the unified image; the
  frontend is no longer served by nginx in production.

## Files added / changed / removed

**Added:** `Dockerfile` (root), `.dockerignore` (root),
`docs/superpowers/specs/2026-06-19-single-image-release-design.md`.

**Changed:** `backend/app.py` (Blueprint + SPA serving + headers),
`backend/pyproject.toml` (`flask-compress`), `backend/tests/conftest.py`
(client fixture) + a new test module, `.github/workflows/release.yml`,
`.github/workflows/ci.yml` (optional smoke job), `docker-compose.yml`,
`docker-compose.release.yml`, `README.md`, `backend/README.md`,
`frontend/README.md`.

**Removed:** `backend/Dockerfile`, `frontend/Dockerfile`,
`frontend/nginx.conf.template`.

## Testing & acceptance criteria

- `make test-backend` and `make test-frontend` pass (existing suites), with the
  new backend tests for SPA serving / collision / headers.
- `docker build -t statshed-server .` succeeds; `docker run -p 7827:7828` then:
  - `GET http://localhost:7827/` → SPA HTML with the security headers + CSP.
  - `GET http://localhost:7827/jobs` → SPA HTML (not JSON).
  - `GET http://localhost:7827/api/health` → 200 JSON.
  - `POST http://localhost:7827/api/status` → ingests a job (CLI contract).
  - The dashboard loads and receives live updates over the WebSocket.
- Alembic migrations run on container start (`entrypoint.sh`).

## Risks / verification points

- **WebSocket under the unified server.** Confirm the engineio middleware still
  intercepts `/socket.io` ahead of the SPA catch-all (manual + socketio test
  client). This is the highest-risk item.
- **CSP hash drift.** The ported CSP must keep the exact inline-script sha256, or
  the pre-hydration theme script is blocked. Covered by a header test +
  AIDEV-NOTE.
- **Static serving via gevent** instead of nginx — fine at this asset size;
  `flask-compress` + immutable caching keep it efficient.
- **Single gunicorn worker** (`-w 1`, required for SQLite WAL) is unchanged.

## Out of scope (YAGNI)

Image signing / SBOM / provenance, Docker Hub mirroring, PyInstaller standalone
binaries, PyPI wheels. The two-job CI test split stays. No auth changes.
