# Single-Image `statshed-server` Release — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the two-image release (Flask backend + nginx/React frontend) into one `statshed-server` Docker image that serves the REST API, the Socket.IO WebSocket, and the React SPA from a single Flask/gunicorn process.

**Architecture:** The Flask app, under its existing single gevent-websocket gunicorn worker, gains an `/api` Blueprint (the API moves off root so the SPA can own root), a conditional static/SPA-fallback handler serving the built `dist/`, and nginx's security headers/CSP/caching ported into Flask `after_request`. A single multi-stage Dockerfile builds the SPA in a Node stage and copies it into the Python image. Compose, the release workflow, and docs collapse to one service/image.

**Tech Stack:** Python 3.13, Flask 3 + flask-socketio + gevent, SQLAlchemy 2 + Alembic, gunicorn (`GeventWebSocketWorker`), `uv`; React 19 + Vite 6 (build only); Docker multi-stage, GitHub Actions, GHCR.

## Global Constraints

- **Python `>=3.13`**; manage deps with **`uv` only** (`uv add`, `uv sync --frozen`) — never pip/poetry/requirements.txt.
- Backend must stay green under: `uv run ruff format --check .`, `uv run ruff check .`, `uv run mypy app.py models.py config.py background.py`, `uv run pytest`. Run `uv run ruff format .` after editing Python.
- All new/edited functions get **type annotations** (Sean's global rule).
- Preserve existing **`AIDEV-NOTE`/`AIDEV-TODO`** anchors; add new ones where code is subtle. Never alter Alembic migration files.
- **Single gunicorn worker** (`-w 1`) stays — required for SQLite WAL consistency.
- **No authentication** is added (by design; internal-network app).
- **CSP exact-match:** the ported Content-Security-Policy must keep the inline-script hash `'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='` verbatim, or the pre-hydration theme script in `index.html` is blocked.
- **Ports:** container listens internally on `7828`; the published host port is `7827` (compose maps `7827:7828`). API ingest is under `/api` (e.g. `POST /api/status`).
- **Release images** stay multi-arch `linux/amd64,linux/arm64`.

---

## File Structure

**Backend (`backend/`)**
- `app.py` — add `Blueprint`, rename 16 route decorators, add `register_spa()` + import-time guard, add `after_request` security headers, init `flask-compress`.
- `pyproject.toml` / `uv.lock` — add `flask-compress` (Task 4, optional).
- `tests/conftest.py` — `client` fixture prepends `/api`; add `raw_client` fixture.
- `tests/test_routing.py` *(new)* — API lives under `/api`, not root.
- `tests/test_static_serving.py` *(new)* — SPA served at root, no `/jobs` collision, unknown `/api/*` is 404 JSON.
- `tests/test_security_headers.py` *(new)* — headers + exact CSP present.

**Image / deploy (repo root)**
- `Dockerfile` *(new)* — single multi-stage build.
- `.dockerignore` *(new)* — root build-context excludes.
- Remove: `backend/Dockerfile`, `frontend/Dockerfile`, `frontend/nginx.conf.template`.
- `docker-compose.yml`, `docker-compose.release.yml` — single `statshed` service.
- `.github/workflows/release.yml` — build one image; `.github/workflows/ci.yml` — optional smoke job.

**Docs**
- `README.md`, `backend/README.md`, `frontend/README.md`.

> **Optional tasks:** Task 4 (`flask-compress` gzip) and Task 8 (CI smoke job) were flagged optional in the spec. Implement both unless Sean says otherwise; each is self-contained and skippable.

---

### Task 1: Move the REST API behind an `/api` Blueprint

Moves all 16 API view routes off root onto a Blueprint mounted at `/api`, and updates the test client so the existing 371 root-path test calls keep working. **Required first** — `/jobs` is both an SPA route and an API route, so the API cannot stay at root once Flask serves the SPA (Task 2).

**Files:**
- Modify: `backend/app.py` (imports ~16-22; add Blueprint after `socketio` block ~56; rename `@app.route`→`@api.route` at lines 275, 384, 487, 564, 626, 700, 754, 1026, 1090, 1165, 1267, 1294, 1400, 1469, 1646, 1699; register blueprint before `if __name__` ~1871)
- Modify: `backend/tests/conftest.py:60-63`
- Create: `backend/tests/test_routing.py`

**Interfaces:**
- Produces: a `flask.Blueprint` named `"api"` registered at `url_prefix="/api"`; all existing endpoints now answer under `/api/...` (e.g. `/api/health`, `/api/jobs`, `/api/status`). Endpoint names become `api.<func>` (no `url_for` exists in the backend, so this is safe).
- Produces (test): `raw_client` fixture (no prefix) and a `client` fixture that auto-prepends `/api`.

- [ ] **Step 1: Write the failing test** — `backend/tests/test_routing.py`

```python
"""The REST API is served under /api, not at root.

AIDEV-NOTE: Uses raw_client (no /api auto-prefix) to assert the real URL space.
"""


def test_health_is_under_api(raw_client):
    assert raw_client.get("/api/health").status_code == 200


def test_health_not_at_root(raw_client):
    # Root /health no longer exists (no SPA static dir in tests -> plain 404).
    assert raw_client.get("/health").status_code == 404


def test_status_is_under_api(raw_client):
    # /status moved too; bare POST /status is gone.
    assert raw_client.post("/status", json={}).status_code == 404
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && uv run pytest tests/test_routing.py -v`
Expected: FAIL — `raw_client` fixture doesn't exist yet / `/api/health` is 404.

- [ ] **Step 3: Add the `raw_client` and prefixing `client` fixtures** — replace `backend/tests/conftest.py:60-63`

```python
from flask.testing import FlaskClient


class _ApiPrefixClient(FlaskClient):
    """Test client that transparently prepends /api to request paths.

    AIDEV-NOTE: Production serves the REST API under an /api Blueprint so the
    React SPA can own root (the /jobs *page* vs the GET /jobs *API*). The test
    suite predates that split and addresses routes at root (/jobs, /status, ...).
    This wrapper injects /api so those ~371 calls keep working unchanged. Paths
    under /socket.io or already under /api are passed through untouched.
    """

    def open(self, *args, **kwargs):  # type: ignore[override]
        path = None
        pos = None
        if args and isinstance(args[0], str):
            path, pos = args[0], 0
        elif isinstance(kwargs.get("path"), str):
            path = kwargs["path"]
        if (
            path is not None
            and path.startswith("/")
            and not path.startswith("/api")
            and not path.startswith("/socket.io")
        ):
            new = "/api" + path
            if pos == 0:
                args = (new,) + args[1:]
            else:
                kwargs["path"] = new
        return super().open(*args, **kwargs)


@pytest.fixture
def client(app):
    """Test client that addresses the API at root (auto-prefixed to /api)."""
    app.test_client_class = _ApiPrefixClient
    return app.test_client()


@pytest.fixture
def raw_client(app):
    """Test client with NO prefixing — for testing real URL routing/SPA."""
    app.test_client_class = FlaskClient
    return app.test_client()
```

- [ ] **Step 4: Add the Blueprint to `app.py`**

In the imports, change line 16 and add `Blueprint`:

```python
from flask import Blueprint, Flask, jsonify, request
```

Immediately after the `socketio = SocketIO(...)` block (~line 56), add:

```python
# AIDEV-NOTE: The REST API lives under an /api Blueprint so the unified image can
# serve the React SPA at root without route collisions (e.g. the SPA's /jobs page
# vs this API's GET /jobs). nginx used to strip /api; now Flask owns the prefix.
api = Blueprint("api", __name__)
```

Rename every `@app.route(` to `@api.route(` (16 occurrences, listed in **Files** above). Do **not** touch `@app.errorhandler`, `@app.teardown_appcontext`, or `@socketio.on`.

Then, just before `if __name__ == "__main__":` (~line 1871), register it:

```python
# AIDEV-NOTE: Register the API under /api. Must come after all @api.route defs.
app.register_blueprint(api, url_prefix="/api")
```

- [ ] **Step 5: Verify the new test passes and the full suite is green**

Run: `cd backend && uv run pytest tests/test_routing.py -v && uv run pytest`
Expected: PASS — `test_routing.py` green; all pre-existing tests pass via the auto-prefix client.

- [ ] **Step 6: Format, lint, type-check**

Run: `cd backend && uv run ruff format . && uv run ruff check . && uv run mypy app.py models.py config.py background.py`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add backend/app.py backend/tests/conftest.py backend/tests/test_routing.py
git commit -m "Move REST API behind an /api Blueprint"
```

---

### Task 2: Serve the React SPA from Flask (static + history fallback)

Adds a conditional catch-all that serves the built `dist/` with an `index.html` fallback for client-side routes, while keeping `/api/*` strictly API (unknown `/api/*` → 404 JSON, never the SPA).

**Files:**
- Modify: `backend/app.py` (imports; add `register_spa()` + import-time guard near the end)
- Create: `backend/tests/test_static_serving.py`

**Interfaces:**
- Consumes: the `/api` Blueprint from Task 1.
- Produces: `register_spa(app: Flask, static_dir: str) -> None` — registers `/` and `/<path:path>` to serve files from `static_dir`, falling back to `index.html`; aborts 404 for `api/*` paths. Auto-called at import when `STATIC_DIR` (default `<app dir>/static`) is a directory.

- [ ] **Step 1: Write the failing test** — `backend/tests/test_static_serving.py`

```python
"""Flask serves the built SPA at root, without shadowing the /api API."""


def _make_dist(tmp_path):
    (tmp_path / "index.html").write_text(
        "<!doctype html><title>StatShed</title><div id='root'></div>"
    )
    assets = tmp_path / "assets"
    assets.mkdir()
    (assets / "app.js").write_text("console.log('hi')")
    return str(tmp_path)


def test_spa_served_at_root(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/")
    assert resp.status_code == 200
    assert b"StatShed" in resp.data


def test_spa_fallback_does_not_shadow_jobs_api(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    client = app.test_client()
    # /jobs is an SPA route -> must return the SPA shell, NOT the GET /jobs JSON.
    assert b"StatShed" in client.get("/jobs").data
    # The API is still reachable under /api.
    assert client.get("/api/health").status_code == 200


def test_real_asset_is_served(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/assets/app.js")
    assert resp.status_code == 200
    assert b"console.log" in resp.data


def test_unknown_api_path_is_404_not_spa(app, tmp_path):
    from app import register_spa

    register_spa(app, _make_dist(tmp_path))
    resp = app.test_client().get("/api/does-not-exist")
    assert resp.status_code == 404
    assert b"StatShed" not in resp.data  # JSON error, not the SPA shell
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && uv run pytest tests/test_static_serving.py -v`
Expected: FAIL — `cannot import name 'register_spa'`.

- [ ] **Step 3: Implement `register_spa` + the import-time guard in `app.py`**

Add to the imports: `import os` (top, near `import re`), and extend the flask + werkzeug imports:

```python
from flask import Blueprint, Flask, abort, jsonify, request, send_from_directory
from werkzeug.exceptions import HTTPException, NotFound
```

Just before `app.register_blueprint(api, url_prefix="/api")` (added in Task 1), add:

```python
# AIDEV-NOTE: In the unified image Flask serves the built React SPA from
# STATIC_DIR (the Vite dist/, copied to ./static by the Dockerfile). Registered
# only when the dir exists, so local dev (`python app.py`, Vite serves the SPA on
# its own port) is unaffected. /api/* and /socket.io are never served as the SPA.
def register_spa(app: Flask, static_dir: str) -> None:
    """Serve the SPA from static_dir with history-API (index.html) fallback."""

    @app.route("/", defaults={"path": ""})
    @app.route("/<path:path>")
    def serve_spa(path: str):
        if path.startswith("api/"):
            abort(404)  # unknown API path -> JSON 404 via errorhandler, not SPA
        if path:
            try:
                resp = send_from_directory(static_dir, path)
                resp.headers["Cache-Control"] = "public, immutable, max-age=31536000"
                return resp
            except NotFound:
                pass  # client-side route -> fall through to index.html
        resp = send_from_directory(static_dir, "index.html")
        resp.headers["Cache-Control"] = "no-cache"
        return resp


_STATIC_DIR: str = os.environ.get("STATIC_DIR") or os.path.join(
    os.path.dirname(__file__), "static"
)
if os.path.isdir(_STATIC_DIR):
    register_spa(app, _STATIC_DIR)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd backend && uv run pytest tests/test_static_serving.py -v`
Expected: PASS (all four tests).

- [ ] **Step 5: Full suite + format/lint/type-check**

Run: `cd backend && uv run pytest && uv run ruff format . && uv run ruff check . && uv run mypy app.py models.py config.py background.py`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add backend/app.py backend/tests/test_static_serving.py
git commit -m "Serve the React SPA from Flask with history-API fallback"
```

---

### Task 3: Port nginx's security headers + CSP into Flask

Replicates the headers nginx set on every response (`always`), so dropping nginx doesn't regress the security posture.

**Files:**
- Modify: `backend/app.py` (add CSP constant + `after_request`)
- Create: `backend/tests/test_security_headers.py`

**Interfaces:**
- Produces: an `@app.after_request` hook setting `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, and `Content-Security-Policy` on all responses.

- [ ] **Step 1: Write the failing test** — `backend/tests/test_security_headers.py`

```python
"""nginx's security headers/CSP are preserved now that Flask serves responses."""

EXPECTED_CSP = (
    "default-src 'self'; "
    "script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; "
    "style-src 'self' 'unsafe-inline'; "
    "img-src 'self' data:; "
    "font-src 'self'; "
    "connect-src 'self'; "
    "object-src 'none'; "
    "base-uri 'self'; "
    "frame-ancestors 'none'; "
    "form-action 'self'"
)


def test_security_headers_present(raw_client):
    resp = raw_client.get("/api/health")
    assert resp.headers["X-Frame-Options"] == "DENY"
    assert resp.headers["X-Content-Type-Options"] == "nosniff"
    assert resp.headers["Referrer-Policy"] == "strict-origin-when-cross-origin"


def test_csp_matches_nginx(raw_client):
    resp = raw_client.get("/api/health")
    assert resp.headers["Content-Security-Policy"] == EXPECTED_CSP
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd backend && uv run pytest tests/test_security_headers.py -v`
Expected: FAIL — `KeyError: 'X-Frame-Options'`.

- [ ] **Step 3: Implement the headers in `app.py`**

After the `register_blueprint`/SPA block (near the end, before `if __name__`), add:

```python
# AIDEV-NOTE: Security headers ported verbatim from the old nginx.conf.template
# (sent on every response, matching nginx's `always`). The CSP is tuned to this
# SPA; the script-src sha256 allowlists the inline pre-hydration theme bootstrap
# in index.html. If you edit that <script>, recompute the hash:
#   npm run build && node -e 'const f=require("fs"),c=require("crypto");\
#     const m=f.readFileSync("dist/index.html","utf8").match(/<script>([\s\S]*?)<\/script>/);\
#     console.log("sha256-"+c.createHash("sha256").update(m[1]).digest("base64"))'
_CSP = (
    "default-src 'self'; "
    "script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; "
    "style-src 'self' 'unsafe-inline'; "
    "img-src 'self' data:; "
    "font-src 'self'; "
    "connect-src 'self'; "
    "object-src 'none'; "
    "base-uri 'self'; "
    "frame-ancestors 'none'; "
    "form-action 'self'"
)


@app.after_request
def set_security_headers(response):
    response.headers["X-Frame-Options"] = "DENY"
    response.headers["X-Content-Type-Options"] = "nosniff"
    response.headers["Referrer-Policy"] = "strict-origin-when-cross-origin"
    response.headers["Content-Security-Policy"] = _CSP
    return response
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd backend && uv run pytest tests/test_security_headers.py -v`
Expected: PASS.

- [ ] **Step 5: Full suite + format/lint/type-check**

Run: `cd backend && uv run pytest && uv run ruff format . && uv run ruff check . && uv run mypy app.py models.py config.py background.py`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add backend/app.py backend/tests/test_security_headers.py
git commit -m "Port nginx security headers and CSP into Flask"
```

---

### Task 4 *(optional)*: gzip responses with `flask-compress`

Replaces nginx's gzip. Cheap; skip only if Sean opts out.

**Files:**
- Modify: `backend/pyproject.toml` + `backend/uv.lock` (via `uv add`), `backend/app.py`

- [ ] **Step 1: Add the dependency (updates pyproject + uv.lock)**

Run: `cd backend && uv add flask-compress`
Expected: `flask-compress` added under `[project].dependencies`; `uv.lock` updated.

- [ ] **Step 2: Initialize it in `app.py`**

Add to imports:

```python
from flask_compress import Compress
```

Immediately after the `CORS(app, ...)` line (~line 43), add:

```python
# AIDEV-NOTE: gzip responses (replaces nginx's gzip now that Flask serves assets).
Compress(app)
```

If mypy reports missing stubs for `flask_compress`, add it to the existing `[[tool.mypy.overrides]]` `module` list in `pyproject.toml` (alongside `flask_socketio.*`, `flask_cors.*`):

```toml
module = ["flask_socketio.*", "flask_cors.*", "flask_compress.*"]
```

- [ ] **Step 3: Verify the app boots and suite is green**

Run: `cd backend && uv run pytest && uv run ruff format . && uv run ruff check . && uv run mypy app.py models.py config.py background.py`
Expected: green.

- [ ] **Step 4: Commit**

```bash
git add backend/pyproject.toml backend/uv.lock backend/app.py
git commit -m "Add flask-compress for gzip (replaces nginx gzip)"
```

---

### Task 5: Single multi-stage Dockerfile (+ root `.dockerignore`); remove the old image files

One image builds the SPA in a Node stage and serves everything from the Python stage. Build context becomes the repo root.

**Files:**
- Create: `Dockerfile` (repo root), `.dockerignore` (repo root)
- Remove: `backend/Dockerfile`, `frontend/Dockerfile`, `frontend/nginx.conf.template`

- [ ] **Step 1: Create `Dockerfile`** (repo root)

```dockerfile
# syntax=docker/dockerfile:1

# ---- Stage 1: build the React SPA ----
FROM node:22-alpine AS builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Same-origin bundle: the SPA uses relative /api and same-origin Socket.IO, so no
# VITE_BACKEND_URL is needed.
RUN npm run build
# -> /app/dist

# ---- Stage 2: Python runtime serving the API + WebSocket + SPA ----
FROM python:3.13-slim

# curl is used by the compose healthcheck.
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

WORKDIR /app
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Dependency layer first for caching.
COPY backend/pyproject.toml backend/uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

# Backend source, then install the project itself.
COPY backend/ ./
RUN uv sync --frozen --no-dev

# Built SPA -> /app/static (served by Flask via STATIC_DIR default).
COPY --from=builder /app/dist ./static

RUN mkdir -p /data

COPY backend/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 7828
ENV HOST=0.0.0.0 \
    PORT=7828 \
    DATABASE_URL=sqlite:////data/statshed.db

# AIDEV-NOTE: entrypoint runs `alembic upgrade head` before launching gunicorn.
ENTRYPOINT ["/entrypoint.sh"]

# AIDEV-NOTE: Single worker (-w 1) is required for SQLite WAL mode consistency.
CMD ["uv", "run", "gunicorn", "-w", "1", "-k", "geventwebsocket.gunicorn.workers.GeventWebSocketWorker", "--bind", "0.0.0.0:7828", "app:app"]
```

- [ ] **Step 2: Create `.dockerignore`** (repo root)

```gitignore
.git
.github
docs
.remember
.codex
**/node_modules
**/.venv
**/__pycache__
**/.pytest_cache
**/.mypy_cache
**/.ruff_cache
**/dist
**/coverage
**/test-results
**/playwright-report
**/*.tsbuildinfo
*.db
*.db-shm
*.db-wal
.env
.env.*
```

- [ ] **Step 3: Remove the superseded files**

```bash
git rm backend/Dockerfile frontend/Dockerfile frontend/nginx.conf.template
```

- [ ] **Step 4: Build the image and smoke-test it**

```bash
docker build -t statshed-server:local .
docker run --rm -d --name statshed-smoke -p 7827:7828 -e SECRET_KEY=test statshed-server:local
sleep 5
curl -fsS http://127.0.0.1:7827/api/health        # expect: JSON, HTTP 200
curl -fsS http://127.0.0.1:7827/ | grep -o '<title>[^<]*</title>'   # expect: <title>StatShed</title>
curl -fsS http://127.0.0.1:7827/jobs | grep -o '<title>[^<]*</title>' # expect: <title>StatShed</title> (SPA, not JSON)
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7827/api/does-not-exist  # expect: 404
docker rm -f statshed-smoke
```
Expected: `/api/health` 200 JSON; `/` and `/jobs` return the SPA shell; unknown `/api/*` is 404.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "Add single multi-stage Dockerfile; remove split backend/frontend images"
```

---

### Task 6: Collapse compose files to one `statshed` service

**Files:**
- Modify: `docker-compose.yml`, `docker-compose.release.yml`

- [ ] **Step 1: Rewrite `docker-compose.yml`**

```yaml
# StatShed — full stack from source, as a single image.
#
#   cp .env.example .env        # then set SECRET_KEY
#   docker compose up --build -d
#   open http://localhost:7827
#
# One container serves the React SPA, the REST API, and the Socket.IO WebSocket.
# CLI clients POST job status to the same origin under /api (e.g. /api/status).

services:
  statshed:
    build: .
    image: statshed-server:local
    environment:
      - DATABASE_URL=${DATABASE_URL:-sqlite:////data/statshed.db}
      - SECRET_KEY=${SECRET_KEY:-}
      - DEBUG=${DEBUG:-false}
    ports:
      # Browser dashboard AND CLI status POSTs both use this single port.
      - "${STATSHED_PORT:-7827}:7828"
    volumes:
      - statshed-data:/data
    restart: unless-stopped
    healthcheck:
      # 127.0.0.1 (not localhost): the server listens on IPv4 only.
      test: ["CMD", "curl", "-f", "http://127.0.0.1:7828/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  statshed-data:
```

- [ ] **Step 2: Rewrite `docker-compose.release.yml`**

```yaml
# StatShed — single prebuilt image (no local build).
#
#   cp .env.example .env                                                   # set SECRET_KEY
#   STATSHED_VERSION=v0.1.0 docker compose -f docker-compose.release.yml up -d
#   open http://localhost:7827

services:
  statshed:
    image: ghcr.io/statshed/statshed-server:${STATSHED_VERSION:-latest}
    environment:
      - DATABASE_URL=${DATABASE_URL:-sqlite:////data/statshed.db}
      - SECRET_KEY=${SECRET_KEY:-}
      - DEBUG=${DEBUG:-false}
    ports:
      - "${STATSHED_PORT:-7827}:7828"
    volumes:
      - statshed-data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://127.0.0.1:7828/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  statshed-data:
```

- [ ] **Step 3: Validate both compose files**

```bash
docker compose -f docker-compose.yml config >/dev/null && echo OK
docker compose -f docker-compose.release.yml config >/dev/null && echo OK
```
Expected: `OK` twice (no schema errors).

- [ ] **Step 4: End-to-end check with compose**

```bash
SECRET_KEY=test docker compose up --build -d
sleep 8
curl -fsS http://127.0.0.1:7827/api/health   # expect 200 JSON
docker compose down
```
Expected: healthy response; stack starts from the single service.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml docker-compose.release.yml
git commit -m "Collapse compose to a single statshed service"
```

---

### Task 7: Build one image in the release workflow

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Replace the two-entry build matrix with a single image build**

Replace the `images` job's `strategy`/`matrix` and the build steps so it builds one image. The full job:

```yaml
jobs:
  image:
    name: Build & push statshed-server image
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Docker metadata (tags & labels)
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/statshed/statshed-server
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=raw,value=latest

      - name: Build & push
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

- [ ] **Step 2: Update the `release` job's `needs` and body text**

Change `needs: images` → `needs: image`. In the GitHub Release `body`, replace the two image bullets with one:

```yaml
            Multi-arch container image (`linux/amd64`, `linux/arm64`):

            - `ghcr.io/statshed/statshed-server:${{ github.ref_name }}`
```

- [ ] **Step 3: Lint the workflow**

```bash
actionlint .github/workflows/release.yml || echo "actionlint not installed — eyeball the YAML diff instead"
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/release.yml')); print('YAML OK')"
```
Expected: `YAML OK` (and no actionlint errors if installed).

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "Release: build the single statshed-server image"
```

---

### Task 8 *(optional)*: CI smoke job for the unified image

Adds a CI job that builds the image and curls `/` + `/api/health`, catching integration regressions before a tag.

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Append a `smoke` job** to `.github/workflows/ci.yml` (sibling of `backend`/`frontend`/`e2e`):

```yaml
  smoke:
    name: Unified image smoke test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Build image
        run: docker build -t statshed-server:ci .

      - name: Run and probe
        run: |
          docker run --rm -d --name statshed-ci -p 7827:7828 -e SECRET_KEY=ci statshed-server:ci
          for i in $(seq 1 20); do
            if curl -fsS http://127.0.0.1:7827/api/health; then ok=1; break; fi
            sleep 2
          done
          test "${ok:-0}" = "1" || { docker logs statshed-ci; exit 1; }
          curl -fsS http://127.0.0.1:7827/ | grep -q '<title>StatShed</title>'
          docker rm -f statshed-ci
```

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML OK')"
```
Expected: `YAML OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "CI: smoke-test the unified statshed-server image"
```

---

### Task 9: Update the docs

**Files:**
- Modify: `README.md`, `backend/README.md`, `frontend/README.md`

- [ ] **Step 1: Update the architecture bullets in `README.md`**

Replace the two bullets describing `backend/` and `frontend/` plus the one-compose line:

```markdown
- **`backend/`** — a Flask REST API + Socket.IO server (Python 3.13), persisting to SQLite. It also serves the built frontend.
- **`frontend/`** — a React + Vite single-page dashboard, built and bundled into the backend image.

A single `statshed-server` Docker image serves the dashboard, the REST API, and the live WebSocket. One `docker compose up` brings it online.
```

- [ ] **Step 2: Replace the architecture diagram in `README.md`** with a single-box version:

```
                ┌──────────────────────────────────────┐
   browser ────▶│  statshed-server                     │
   :7827        │  Flask + Socket.IO  (serves SPA,      │
                │  /api REST, /socket.io WebSocket)     │
   CLI ────────▶│  SQLite @ /data                       │
   POST /api    └──────────────────────────────────────┘
                                  :7827
```

- [ ] **Step 3: Fix remaining references** — grep and update each hit to the single-image, single-port, `/api` model:

```bash
grep -rnE "7828|nginx|statshed-frontend|statshed-backend|two (images|containers)|BACKEND_URL" README.md backend/README.md frontend/README.md
```
For each hit: the browser/CLI port is `7827`; CLI status POSTs go to `http://host:7827/api/status` (was `:7828/status`); there is no separate nginx/frontend image; `BACKEND_URL`/`VITE_BACKEND_URL` no longer apply to the bundled deployment (keep any mention only under local-dev with Vite). In `frontend/README.md`, note the SPA is built into the backend image (no nginx in production).

- [ ] **Step 4: Sanity-check the docs build/render**

```bash
grep -rnE "7828|nginx|statshed-frontend|statshed-backend" README.md && echo "REVIEW remaining hits (some dev-only mentions may be fine)" || echo "no stale infra references"
```
Expected: only intentional dev-only references remain (if any).

- [ ] **Step 5: Commit**

```bash
git add README.md backend/README.md frontend/README.md
git commit -m "Docs: describe the single statshed-server image"
```

---

## Self-Review

**Spec coverage** (each spec section → task):
- API under `/api` Blueprint → **Task 1**.
- Conditional SPA serving + `/api/*` 404 (no collision) → **Task 2**.
- Security headers + exact CSP → **Task 3**.
- `flask-compress` gzip → **Task 4** (optional).
- Single multi-stage Dockerfile, root `.dockerignore`, remove old Dockerfiles + nginx template → **Task 5**.
- Single-service compose (build + release), healthcheck `/api/health`, `STATSHED_PORT` → **Task 6**.
- One-image release workflow → **Task 7**.
- CI smoke job → **Task 8** (optional).
- README/sub-README updates (diagram, ports, CLI `/api` URL) → **Task 9**.
- Test-fixture trick keeping 371 calls unchanged → **Task 1** (`conftest.py`).
- WebSocket-under-unified-server risk → exercised by the existing `socketio_client` suite (unchanged) plus Task 5/6/8 live `/api/health` probes; the engineio middleware intercepts `/socket.io` before the SPA catch-all.

**Placeholder scan:** none — every code/edit step shows concrete content; doc step uses a grep-and-fix with explicit replacement rules.

**Type/name consistency:** `register_spa(app, static_dir)` defined in Task 2 and imported identically in its tests; `_ApiPrefixClient`/`raw_client`/`client` names consistent across Tasks 1–3; CSP string identical in Task 3 implementation and `test_security_headers.py`; container internal port `7828` and host port `7827` consistent across Dockerfile, compose, healthcheck (`/api/health`), and docs.
