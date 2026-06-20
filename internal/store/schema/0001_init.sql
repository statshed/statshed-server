-- +goose Up
-- AIDEV-NOTE: Single baseline migration, logically equivalent to the Python Alembic
-- schema (behavioral-map §5; dumped from a migrated DB). Fresh-DB-only (C1): plain
-- CREATE (no IF NOT EXISTS), so running against a DB that already holds these tables
-- errors and the server fails fast rather than adopting/mutating an existing DB.
-- `id INTEGER PRIMARY KEY` is the rowid alias (ids reset to 1 after a full DELETE) —
-- deliberately NO AUTOINCREMENT. Datetimes are stored as text 'YYYY-MM-DD HH:MM:SS.ffffff'.

CREATE TABLE groups (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    progress_timeout_minutes INTEGER,
    staleness_timeout_hours INTEGER,
    staleness_enabled INTEGER NOT NULL DEFAULT 0,
    expiration_timeout_hours INTEGER,
    created_at TEXT NOT NULL
);

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY,
    group_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    acked INTEGER NOT NULL DEFAULT 0,
    acked_at TEXT,
    expires_at TEXT,
    log_content TEXT,
    log_line_count INTEGER,
    log_truncated INTEGER NOT NULL DEFAULT 0,
    log_updated_at TEXT,
    updated_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CONSTRAINT uq_job_group_name UNIQUE (group_id, name),
    FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE
);

CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX ix_jobs_status ON jobs (status);
CREATE INDEX ix_jobs_updated_at ON jobs (updated_at);
CREATE INDEX ix_jobs_group_id ON jobs (group_id);
CREATE INDEX ix_jobs_status_updated ON jobs (status, updated_at);
CREATE INDEX ix_jobs_acked ON jobs (acked);
CREATE INDEX ix_jobs_status_acked ON jobs (status, acked);
CREATE INDEX ix_jobs_expires_at ON jobs (expires_at);

-- +goose Down
DROP TABLE jobs;
DROP TABLE groups;
DROP TABLE config;
