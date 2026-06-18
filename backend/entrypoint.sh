#!/bin/bash
# AIDEV-NOTE: Entrypoint script that runs database migrations before starting the app
# This ensures the database schema is always up-to-date when deploying new versions

set -e

echo "Running database migrations..."
uv run alembic upgrade head

echo "Starting application..."
exec "$@"
