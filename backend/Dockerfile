# syntax=docker/dockerfile:1

FROM python:3.13-slim

# Install system dependencies (curl for healthcheck)
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

WORKDIR /app

# Install uv package manager
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Copy dependency files first for better layer caching
COPY pyproject.toml uv.lock ./

# Install dependencies
RUN uv sync --frozen --no-dev --no-install-project

# Copy application code
COPY . .

# Install the project itself
RUN uv sync --frozen --no-dev

# Create directory for SQLite database persistence
RUN mkdir -p /data

# Copy and set up entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose the application port
EXPOSE 7828

# Set default environment variables for Docker
ENV HOST=0.0.0.0 \
    PORT=7828 \
    DATABASE_URL=sqlite:////data/statshed.db

# AIDEV-NOTE: Entrypoint runs alembic migrations before starting the app
ENTRYPOINT ["/entrypoint.sh"]

# Run with gunicorn and gevent for production
# AIDEV-NOTE: Single worker (-w 1) is required for SQLite WAL mode consistency
CMD ["uv", "run", "gunicorn", "-w", "1", "-k", "geventwebsocket.gunicorn.workers.GeventWebSocketWorker", "--bind", "0.0.0.0:7828", "app:app"]
