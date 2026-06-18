"""Application configuration.

AIDEV-NOTE: All configuration is loaded from environment variables with
sensible defaults. Database URL defaults to SQLite for local development.
"""

import os
import secrets


class Config:
    """Flask application configuration."""

    # Flask settings
    SECRET_KEY: str = os.environ.get("SECRET_KEY") or secrets.token_hex(32)
    DEBUG: bool = os.environ.get("DEBUG", "false").lower() == "true"

    # Server settings
    HOST: str = os.environ.get("HOST", "127.0.0.1")
    PORT: int = int(os.environ.get("PORT", "7828"))

    # Database settings
    # AIDEV-NOTE: SQLite requires check_same_thread=False for multi-threaded access
    DATABASE_URL: str = os.environ.get("DATABASE_URL", "sqlite:///statshed.db")

    # Default timeout values (can be overridden via API)
    DEFAULT_PROGRESS_TIMEOUT_MINUTES: int = 5
    DEFAULT_STALENESS_TIMEOUT_HOURS: int = 24
    # AIDEV-NOTE: Expiration is when jobs auto-delete. Staleness is a warning state
    # before expiration. Staleness is opt-in (disabled by default).
    DEFAULT_EXPIRATION_TIMEOUT_HOURS: int = 24

    # Timeout bounds for validation
    MIN_PROGRESS_TIMEOUT_MINUTES: int = 1
    MAX_PROGRESS_TIMEOUT_MINUTES: int = 10080  # 7 days
    MIN_STALENESS_TIMEOUT_HOURS: int = 1
    MAX_STALENESS_TIMEOUT_HOURS: int = 8760  # 1 year
    MIN_EXPIRATION_TIMEOUT_HOURS: int = 1
    MAX_EXPIRATION_TIMEOUT_HOURS: int = 8760  # 1 year

    # Input validation limits
    MAX_GROUP_NAME_LENGTH: int = 255
    MAX_JOB_NAME_LENGTH: int = 255
    MAX_MESSAGE_LENGTH: int = 4096
    MAX_CONTENT_LENGTH: int = 1024 * 1024  # 1 MB

    # AIDEV-NOTE: Upper bound on the GET /jobs `limit` query param. Pagination is
    # opt-in (no params -> all jobs), but a requested limit is clamped to this value
    # to bound the response size on very large datasets.
    MAX_JOBS_PAGE_SIZE: int = 500

    # Log upload settings
    # AIDEV-NOTE: LOG_UPLOAD_ENABLED controls whether log files can be attached to status updates.
    # When disabled, status updates still succeed but logs are ignored.
    LOG_UPLOAD_ENABLED: bool = (
        os.environ.get("LOG_UPLOAD_ENABLED", "true").lower() == "true"
    )
    # AIDEV-NOTE: MAX_LOG_LINES limits the number of lines stored per log.
    # Logs exceeding this limit are truncated to the last N lines.
    MAX_LOG_LINES: int = int(os.environ.get("MAX_LOG_LINES", "1000"))

    # WebSocket settings
    # AIDEV-NOTE: CORS_ORIGINS configurable via environment variable (comma-separated)
    # e.g., CORS_ORIGINS=http://app1.example.com:7827,http://localhost:5173
    CORS_ORIGINS: list[str] = (
        [o.strip() for o in os.environ["CORS_ORIGINS"].split(",") if o.strip()]
        if os.environ.get("CORS_ORIGINS")
        else [
            "http://localhost:5173",  # Vite dev server
            "http://127.0.0.1:5173",
            "http://localhost:7827",  # Docker frontend
            "http://127.0.0.1:7827",
        ]
    )
    PING_INTERVAL: int = 25
    PING_TIMEOUT: int = 60
    MAX_HTTP_BUFFER_SIZE: int = (
        1024 * 1024
    )  # 1 MB (must be large enough for Socket.IO handshake)


class TestConfig(Config):
    """Configuration for testing."""

    TESTING: bool = True
    DEBUG: bool = True
    DATABASE_URL: str = "sqlite:///:memory:"
