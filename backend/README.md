# StatShed Backend

Flask-based REST API and WebSocket server for the StatShed status dashboard.

> This is one half of the [`statshed-server`](../README.md) monorepo. To run the
> **full stack** (backend + web UI) with one command, see the [root README](../README.md).
> The instructions below cover the backend on its own.

## Quick Start

```bash
# Install dependencies
uv sync

# Run the development server
uv run python app.py

# Run tests
uv run pytest tests/ -v
```

The server will start at `http://127.0.0.1:7828` by default.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `sqlite:///statshed.db` | SQLAlchemy database connection URL |
| `SECRET_KEY` | (auto-generated) | Flask secret key for sessions |
| `DEBUG` | `false` | Enable Flask debug mode (`true`/`false`) |
| `HOST` | `127.0.0.1` | Server bind address |
| `PORT` | `7828` | Server port |
| `LOG_UPLOAD_ENABLED` | `true` | Enable/disable log file uploads with status submissions |
| `MAX_LOG_LINES` | `1000` | Maximum lines to store per log (truncated if exceeded) |

### Database Configuration

The default SQLite database is suitable for single-server deployments with moderate write load.

```bash
# Use a file-based SQLite database (default)
export DATABASE_URL="sqlite:///statshed.db"

# Use PostgreSQL for multi-worker/multi-instance deployments
export DATABASE_URL="postgresql://user:pass@localhost/statshed"
```

**Important:** SQLite requires a single-worker deployment. Do not use multiple gunicorn workers with SQLite.

## API Documentation

See the following documents for API details:

- [REST API Reference](../docs/restapi.md) - CLI and external client API
- [Frontend/WebSocket API](../docs/frontend-api.md) - Frontend-specific API and WebSocket events
- [Design Document](../docs/design-backend.md) - Architecture and implementation details

## Project Structure

```
backend/
├── app.py              # Flask application and REST API routes
├── models.py           # SQLAlchemy models (Group, Job, Config)
├── config.py           # Application configuration
├── background.py       # Background task for timeout checking
├── tests/              # Test suite
│   ├── conftest.py     # Pytest fixtures
│   ├── test_api.py     # REST API tests
│   ├── test_background.py  # Timeout checker tests
│   └── test_integration.py # Integration tests
└── pyproject.toml      # Project dependencies (uv/pip)
```

## Key Features

- **REST API**: CRUD operations for groups, jobs, and configuration
- **WebSocket**: Real-time updates via Socket.IO when job statuses change
- **Background Tasks**: Automatic timeout detection (progress -> timeout, success -> stale)
- **SQLite + WAL**: Concurrent reads with single-writer for simplicity

## Configuration Defaults

| Setting | Default | Range |
|---------|---------|-------|
| Progress timeout | 5 minutes | 1 - 10080 (7 days) |
| Staleness timeout | 24 hours | 1 - 8760 (1 year) |

These can be modified via the `/config` API endpoint.

## Running in Production

For production deployments:

```bash
# Single-worker gunicorn (required for SQLite)
uv run gunicorn -w 1 -k geventwebsocket.gunicorn.workers.GeventWebSocketWorker \
    --bind 0.0.0.0:7828 app:app

# Or with eventlet
uv run gunicorn -w 1 -k eventlet --bind 0.0.0.0:7828 app:app
```

For multi-worker deployments, switch to PostgreSQL:

```bash
export DATABASE_URL="postgresql://user:pass@localhost/statshed"
uv run gunicorn -w 4 -k gevent --bind 0.0.0.0:7828 app:app
```

## Running with Docker

The recommended way to run the backend together with the web UI is the root
compose file — from the repo root:

```bash
docker compose up --build -d        # backend + frontend
# or just the backend service:
docker compose up --build -d backend
```

To build and run only this image by hand:

```bash
# Build the image (from this backend/ directory)
docker build -t statshed-backend .

# Run the container
docker run -d -p 7828:7828 -v statshed-data:/data \
  -e SECRET_KEY=$(openssl rand -hex 32) statshed-backend
```

The SQLite database is persisted in a Docker volume at `/data/statshed.db`.
Configuration is passed via environment variables (see the table above); the root
[`.env.example`](../.env.example) documents the full set.

## Development

```bash
# Format code
uv run ruff format

# Lint code
uv run ruff check

# Type check
uv run mypy app.py models.py config.py background.py

# Run tests with coverage
uv run pytest tests/ -v --cov=. --cov-report=term-missing
```

## Security Notes

This application is designed for internal/localhost use and does **not** include authentication. For external deployments:

1. Use a reverse proxy (nginx, Caddy) with authentication
2. Configure CORS to allow only trusted origins
3. Enable TLS (HTTPS)
4. Consider adding API keys or JWT authentication

See the [Design Document](../docs/design-backend.md#security--access-control) for details.
