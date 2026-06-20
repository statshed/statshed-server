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
