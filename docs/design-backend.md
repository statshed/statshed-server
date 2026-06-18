# StatShed Backend Design Document

A Flask-based backend providing REST API and WebSocket services for the StatShed status dashboard.

## Overview

The backend is the central service that:
- Stores job status data in SQLite via SQLAlchemy
- Provides REST API endpoints consumed by the frontend and CLI
- Pushes real-time updates via WebSocket (Socket.IO)
- Runs background tasks for timeout/staleness detection

```
backend/
├── app.py              # Flask application factory and routes
├── models.py           # SQLAlchemy models (Group, Job, Config)
├── config.py           # Application configuration
├── background.py       # Background task for timeout checking
└── requirements.txt    # Python dependencies
```

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Web Framework | Flask | REST API and request handling |
| ORM | SQLAlchemy | Database abstraction |
| Database | SQLite | Persistent storage |
| WebSocket | Flask-SocketIO (`async_mode='threading'`) | Real-time push updates |
| Background Tasks | Flask-SocketIO `start_background_task` | Periodic timeout checks |
| Linting | Ruff | Code formatting and linting |

### Async Mode & Concurrency Model

**Decision:** Use `async_mode='threading'` with a single-worker process.

| Setting | Value | Rationale |
|---------|-------|-----------|
| `async_mode` | `'threading'` | Simple, compatible with SQLite, no eventlet/gevent complexity |
| Workers | 1 (single process) | Required for SQLite; prevents duplicate background tasks |
| Background scheduler | `socketio.start_background_task()` | Integrated with Flask-SocketIO's threading model |

**Why not eventlet/gevent:**
- Adds monkey-patching complexity
- Not needed for expected concurrency levels
- SQLite doesn't benefit from async I/O

**Configuration:**
```python
socketio = SocketIO(app, async_mode='threading')

# Background task using Flask-SocketIO's threading
def start_timeout_checker():
    def check_timeouts():
        while True:
            socketio.sleep(60)
            with app.app_context():
                run_timeout_check()
    socketio.start_background_task(check_timeouts)
```

> **Warning:** Do NOT use APScheduler with multiple workers - this causes duplicate task execution. Use Flask-SocketIO's built-in background task mechanism instead.

## Security & Access Control

### Design Decision: No Authentication

This dashboard is designed for **internal/localhost use only** and intentionally operates without authentication. This is appropriate because:

- It runs on a trusted internal network or localhost
- The data displayed is operational status information, not sensitive user data
- Simplicity and ease of integration with internal tools is prioritized

**If exposing beyond localhost**, the following should be implemented:

| Security Layer | Recommendation |
|----------------|----------------|
| Transport | TLS (HTTPS) via reverse proxy (nginx, Caddy) |
| Authentication | API keys for CLI clients, session cookies for frontend |
| Authorization | Read-only vs read-write roles for config endpoints |
| Rate Limiting | Per-IP limits at reverse proxy level |

### Current Access Model

| Endpoint Type | Access Level |
|---------------|--------------|
| `GET /health`, `GET /groups`, `GET /groups/<name>/jobs` | Open read |
| `POST /status` | Open write (status submission) |
| `PUT /config`, `PUT /groups/<name>/config` | Open write (config changes) |
| WebSocket | Open subscribe |

> **Note:** For production deployments exposed to untrusted networks, wrap with an authenticating reverse proxy or implement JWT/API-key middleware.

### CORS Policy

**REST API CORS** must be configured separately from WebSocket CORS:

| Deployment | CORS Setting |
|------------|--------------|
| Localhost only | Not needed (same-origin) |
| Internal network | Allowlist internal origins |
| External exposure | Strict allowlist + authentication |

**Flask-CORS Configuration:**
```python
from flask_cors import CORS

# For localhost development
CORS(app, origins=["http://localhost:5173"])

# For production - restrict to known origins
CORS(app, origins=[
    "https://dashboard.internal.example.com",
], supports_credentials=True)
```

> **Warning:** Without CORS restrictions, any website can make requests to write endpoints (`POST /status`, `PUT /config`) if the server is exposed beyond localhost. For non-localhost deployments, always configure an explicit origin allowlist.

## SQLite Configuration

SQLite requires specific configuration for safe concurrent access with Flask-SocketIO and background tasks.

### Required Settings

| Setting | Value | Purpose |
|---------|-------|---------|
| WAL Mode | `PRAGMA journal_mode=WAL` | Allow concurrent reads during writes |
| Busy Timeout | `PRAGMA busy_timeout=5000` | Wait 5s instead of failing immediately on lock |
| Synchronous | `PRAGMA synchronous=NORMAL` | Balance durability and performance |

### Thread Safety

| Concern | Solution |
|---------|----------|
| Multi-threaded access | Use scoped sessions (`scoped_session`) |
| Background task sessions | Create new session per task execution |
| Connection pool | `check_same_thread=False` for SQLite URI |

### SQLAlchemy Engine Configuration

```python
engine = create_engine(
    DATABASE_URL,
    connect_args={"check_same_thread": False},
    pool_pre_ping=True,
)

@event.listens_for(engine, "connect")
def set_sqlite_pragma(dbapi_connection, connection_record):
    cursor = dbapi_connection.cursor()
    cursor.execute("PRAGMA journal_mode=WAL")
    cursor.execute("PRAGMA busy_timeout=5000")
    cursor.execute("PRAGMA synchronous=NORMAL")
    cursor.close()
```

### Scaling Considerations

SQLite is suitable for:
- Single-server deployments
- Low-to-moderate write concurrency (< 100 writes/sec)
- Datasets under 10GB

For higher scale requirements, migrate to PostgreSQL by changing `DATABASE_URL`.

### Single-Process Requirement

> **⚠️ Critical:** SQLite with WAL mode requires a **single-process deployment**. Do NOT run multiple gunicorn/uwsgi workers against SQLite.

**Problems with multi-worker SQLite:**
- Lock contention causes "database is locked" errors under load
- Background timeout checker runs in each worker, causing duplicate state transitions
- WAL checkpointing conflicts between processes

**Acceptable configurations:**

| Configuration | SQLite | PostgreSQL |
|--------------|--------|------------|
| `flask run` (dev server) | ✅ | ✅ |
| `gunicorn -w 1` (single worker) | ✅ | ✅ |
| `gunicorn -w 4` (multiple workers) | ❌ | ✅ |
| Multiple server instances | ❌ | ✅ |

**For multi-worker/multi-instance deployments, switch to PostgreSQL:**
```bash
export DATABASE_URL="postgresql://user:pass@localhost/statshed"
```

## Database Schema

### Groups Table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PRIMARY KEY | Auto-increment ID |
| name | TEXT | UNIQUE, NOT NULL | Group identifier (max 255 chars) |
| progress_timeout_minutes | INTEGER | NULLABLE | Override for progress timeout |
| staleness_timeout_hours | INTEGER | NULLABLE | Override for staleness timeout |
| created_at | DATETIME | NOT NULL | Timestamp when group was created |

### Jobs Table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PRIMARY KEY | Auto-increment ID |
| group_id | INTEGER | FOREIGN KEY (groups.id) | Parent group reference |
| name | TEXT | NOT NULL | Job name (unique within group) |
| status | TEXT | NOT NULL | success, error, progress, timeout, stale |
| message | TEXT | NULLABLE | Optional status message (max 4096 chars) |
| updated_at | DATETIME | NOT NULL | Last status update time |
| created_at | DATETIME | NOT NULL | Timestamp when job was first created |

**Composite Unique Constraint:** (group_id, name)

### Database Indexes

For efficient timeout queries and general performance:

| Index | Columns | Purpose |
|-------|---------|---------|
| `ix_jobs_status` | `status` | Filter jobs by status in timeout checker |
| `ix_jobs_updated_at` | `updated_at` | Order by recency, timeout threshold queries |
| `ix_jobs_group_id` | `group_id` | Fast joins and per-group lookups |
| `ix_jobs_status_updated` | `status, updated_at` | Composite for timeout candidate queries |

### Config Table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| key | TEXT | PRIMARY KEY | Configuration key name |
| value | TEXT | NOT NULL | JSON-encoded configuration value |

**Default Config Values:**
- `progress_timeout_minutes`: 5
- `staleness_timeout_hours`: 24

## REST API Endpoints

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Overall health summary across all jobs |

### Status Submission

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/status` | Submit or update a job status (creates group/job if needed) |

### Groups

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/groups` | List all groups with health summary |
| GET | `/groups/<name>/jobs` | Get all jobs in a specific group |

### Configuration

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/config` | Get global configuration |
| PUT | `/config` | Update global configuration |
| GET | `/groups/<name>/config` | Get group-specific config overrides |
| PUT | `/groups/<name>/config` | Update group-specific config overrides |

## WebSocket Events

### Event Definitions

| Event | Direction | Trigger | Payload |
|-------|-----------|---------|---------|
| `status_update` | Server → Client | Job status changes | See below |
| `group_created` | Server → Client | New group created | See below |
| `health_update` | Server → Client | Background task updates jobs | See below |

### Event Payload Schemas

**`status_update`**
```json
{
  "schema_version": 1,
  "job": {
    "id": 123,
    "name": "daily-backup",
    "status": "success",
    "message": "Completed in 45s",
    "updated_at": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-10T08:00:00Z"
  },
  "group_id": 5,
  "group_name": "backups",
  "previous_status": "progress"
}
```

**`group_created`**
```json
{
  "schema_version": 1,
  "group": {
    "id": 5,
    "name": "backups",
    "progress_timeout_minutes": null,
    "staleness_timeout_hours": null,
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

**`health_update`**

Emitted by the background timeout checker when jobs change state. `transition_type`
is one of `"timeout"` (progress jobs that exceeded their progress timeout) or
`"stale"` (success jobs that exceeded their staleness timeout). A single checker
pass that produces both kinds emits **one event per type**, each carrying only the
ids for that transition; `affected_job_ids`/`affected_group_ids` are scoped to the
event's `transition_type`.

```json
{
  "schema_version": 1,
  "affected_job_ids": [123, 456],
  "affected_group_ids": [5, 7],
  "transition_type": "timeout",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### WebSocket Security & Resource Controls

For localhost/internal deployments, minimal controls are needed. For exposed deployments:

| Control | Setting | Purpose |
|---------|---------|---------|
| Origin Check | Validate `Origin` header | Prevent cross-site WebSocket hijacking |
| Connection Limit | Max 100 concurrent connections | Prevent resource exhaustion |
| Heartbeat/Ping | Every 25 seconds | Detect stale connections |
| Ping Timeout | 60 seconds | Drop unresponsive clients |
| Message Size | Max 1KB inbound | Prevent memory abuse |

**Flask-SocketIO Configuration:**
```python
socketio = SocketIO(
    app,
    cors_allowed_origins=["http://localhost:5173"],  # Restrict in production
    ping_interval=25,
    ping_timeout=60,
    max_http_buffer_size=1024,
)
```

### Missed Event Recovery

The current design uses **best-effort live updates**. Clients that disconnect and reconnect should:

1. Fetch current state via `GET /groups` on reconnect
2. Resume receiving live updates

For future enhancement, consider adding `last_event_id` tracking for replay capability

## Background Task: Timeout Checker

Runs every 60 seconds to:

1. **Progress Timeout**: Jobs with `progress` status exceeding `progress_timeout_minutes` are marked as `timeout`
2. **Staleness Timeout**: Jobs with `success` status exceeding `staleness_timeout_hours` are marked as `stale`

Group-specific overrides take precedence over global settings.

### Query Optimization

To avoid O(n) full table scans, queries should use indexed WHERE clauses with a single effective timeout using `COALESCE`:

**Progress Timeout Query:**
```sql
-- Use COALESCE to get effective timeout: group override OR global default
SELECT j.* FROM jobs j
JOIN groups g ON j.group_id = g.id
WHERE j.status = 'progress'
  AND j.updated_at < datetime('now',
      '-' || COALESCE(g.progress_timeout_minutes, :global_progress_timeout) || ' minutes')
```

**Staleness Timeout Query:**
```sql
-- Use COALESCE to get effective timeout: group override OR global default
SELECT j.* FROM jobs j
JOIN groups g ON j.group_id = g.id
WHERE j.status = 'success'
  AND j.updated_at < datetime('now',
      '-' || COALESCE(g.staleness_timeout_hours, :global_staleness_timeout) || ' hours')
```

**SQLAlchemy Implementation:**
```python
from sqlalchemy import func, literal

global_timeout = get_config_value('progress_timeout_minutes', default=5)

effective_timeout = func.coalesce(
    Group.progress_timeout_minutes,
    literal(global_timeout)
)

cutoff = func.datetime('now', '-' || effective_timeout || ' minutes')

expired_jobs = (
    db.session.query(Job)
    .join(Group)
    .filter(Job.status == 'progress')
    .filter(Job.updated_at < cutoff)
    .all()
)
```

These queries leverage the `ix_jobs_status_updated` composite index to efficiently filter candidates rather than scanning all jobs.

## Input Validation

| Field | Validation |
|-------|------------|
| Group name | Required, max 255 characters, alphanumeric with `-`, `_`, `.`, **case-insensitive** |
| Job name | Required, max 255 characters, alphanumeric with `-`, `_`, `.`, **case-insensitive** |
| Status | Required, one of: success, error, progress, timeout, stale |
| Message | Optional, max 4096 characters |

### Name Canonicalization (Case-Insensitive)

Group and job names are **case-insensitive**. All names are normalized to lowercase on write to ensure consistent uniqueness.

**Behavior:**
- `POST /status` with `group: "MyBackups"` → stored as `"mybackups"`
- `GET /groups/MYBACKUPS/jobs` → matches `"mybackups"`
- Attempting to create `"Foo"` when `"foo"` exists → updates existing record

**Implementation:**
```python
# Normalize names on input
group_name = request.json.get('group', '').strip().lower()
job_name = request.json.get('job', '').strip().lower()

# Validate after normalization
if not re.match(r'^[a-z0-9._-]+$', group_name):
    return {"error": "Invalid group name"}, 400
```

**API responses** return the canonical (lowercase) name, not the original input.

### Configuration Value Bounds

Timeout values must be positive integers within reasonable ranges to prevent overflow and excessive load:

| Config Key | Min | Max | Default |
|------------|-----|-----|---------|
| `progress_timeout_minutes` | 1 | 10080 (7 days) | 5 |
| `staleness_timeout_hours` | 1 | 8760 (1 year) | 24 |

Values outside these ranges return HTTP 400 (Bad Request) with an
`{"error": "validation_error"}` body. AIDEV-NOTE: All client-input validation
errors use 400 uniformly (the `validation_error` slug is never paired with 422).

---

## Implementation Phases

### Phase 1: Project Setup

- [X] Create `backend/` directory structure
- [X] Initialize Python project with `uv`
- [X] Add dependencies (Flask, SQLAlchemy, Flask-SocketIO, etc.)
- [X] Create `config.py` with environment variable handling
- [X] Set up Ruff for linting and formatting
- [X] Create Flask application factory in `app.py`

### Phase 2: Database Layer

- [X] Define SQLAlchemy `Group` model
- [X] Define SQLAlchemy `Job` model with foreign key to Group
- [X] Define SQLAlchemy `Config` model for key-value storage
- [X] Implement `to_dict()` methods on all models for JSON serialization
- [X] Create database initialization function
- [X] Implement helper functions for config get/set with defaults

### Phase 3: Core REST API - Health & Status

- [X] Implement `GET /health` endpoint
  - [X] Query all jobs and calculate status counts
  - [X] Return overall health status (healthy/unhealthy/in_progress/empty)
- [X] Implement `POST /status` endpoint
  - [X] Validate request body (group, job, status required)
  - [X] Validate field lengths and status values
  - [X] Create group if it doesn't exist
  - [X] Create or update job record
  - [X] Return created/updated job data

### Phase 4: Core REST API - Groups

- [X] Implement `GET /groups` endpoint
  - [X] Return all groups with job counts
  - [X] Include per-group health status
  - [X] Include per-group status counts
- [X] Implement `GET /groups/<name>/jobs` endpoint
  - [X] Validate group exists (404 if not)
  - [X] Return group details and all jobs

### Phase 5: Core REST API - Configuration

- [X] Implement `GET /config` endpoint
  - [X] Return global timeout settings with defaults
- [X] Implement `PUT /config` endpoint
  - [X] Validate values are positive integers
  - [X] Update config values in database
- [X] Implement `GET /groups/<name>/config` endpoint
  - [X] Return group-specific overrides
  - [X] Calculate effective values (group override or global fallback)
- [X] Implement `PUT /groups/<name>/config` endpoint
  - [X] Allow setting overrides to null to revert to global
  - [X] Validate values when provided

### Phase 6: WebSocket Integration

- [X] Initialize Flask-SocketIO with the Flask app
- [X] Emit `status_update` event when job status changes via POST /status
- [X] Emit `group_created` event when new group is created
- [X] Configure CORS for WebSocket connections
- [X] Test WebSocket connection and event delivery

### Phase 7: Background Timeout Checker

- [X] Create background task function for timeout checking
- [X] Query jobs with `progress` status exceeding timeout threshold
- [X] Mark expired progress jobs as `timeout`
- [X] Query jobs with `success` status exceeding staleness threshold
- [X] Mark stale jobs as `stale`
- [X] Respect group-specific timeout overrides
- [X] Emit `health_update` WebSocket event when jobs are updated
- [X] Schedule task to run every 60 seconds
- [X] Ensure thread safety with database sessions

### Phase 8: Error Handling & Edge Cases

- [X] Add consistent error response format
- [X] Handle database connection errors gracefully
- [X] Add request size limit (1 MB)
- [X] Handle URL-encoded group names in path parameters
- [X] Log errors appropriately

### Phase 9: Testing

- [X] Set up pytest with test fixtures
- [X] Create test database configuration
- [X] Write tests for `POST /status` endpoint
- [X] Write tests for `GET /health` endpoint
- [X] Write tests for `GET /groups` endpoint
- [X] Write tests for `GET /groups/<name>/jobs` endpoint
- [X] Write tests for `GET /config` and `PUT /config` endpoints
- [X] Write tests for group config endpoints
- [X] Write tests for input validation and error cases
- [X] Write tests for timeout checker background task
- [X] Verify WebSocket events are emitted correctly

### Phase 10: Integration & Documentation

- [X] Verify CLI can connect and submit statuses
- [X] Verify frontend can connect and receive updates
- [X] Test concurrent status submissions
- [X] Add inline code comments for complex logic
- [X] Document environment variables in README

### Phase 11: Data Retention & Cleanup

- [X] Implement `GET /admin/stats` endpoint
  - [X] Return job counts by status
  - [X] Return group count
  - [X] Return database size (for SQLite)
- [X] Implement `DELETE /admin/cleanup` endpoint
  - [X] Accept `older_than_days` parameter
  - [X] Accept `statuses` filter (array of status values)
  - [X] Accept `dry_run` parameter
  - [X] Delete jobs matching criteria
  - [X] Delete empty groups (groups with no jobs)
  - [X] Return count of deleted jobs and groups
- [X] Write tests for admin endpoints
- [X] Update API documentation

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `sqlite:///statshed.db` | SQLAlchemy database connection URL |
| `SECRET_KEY` | (generated) | Flask secret key for sessions |
| `DEBUG` | `false` | Enable Flask debug mode |
| `HOST` | `127.0.0.1` | Server bind address |
| `PORT` | `7828` | Server port |

## HTTP Status Codes

| Code | Usage |
|------|-------|
| 200 | Successful GET or PUT |
| 201 | Successful POST (resource created) |
| 400 | Bad request: malformed JSON, missing required fields, or any client-input validation error (`validation_error` slug — invalid values, out-of-range config, bad limit/offset) |
| 404 | Resource not found (group doesn't exist) |
| 409 | Conflict (unique constraint violation, e.g., duplicate group name) |
| 429 | Too many requests (rate limit exceeded, if rate limiting enabled) |
| 500 | Internal server error |

### Error Response Format

All error responses should use a consistent JSON structure:

```json
{
  "error": "validation_error",
  "message": "progress_timeout_minutes must be between 1 and 10080",
  "field": "progress_timeout_minutes"
}
```

## Status Value Definitions

| Status | Description | Auto-Transition |
|--------|-------------|-----------------|
| `success` | Job completed successfully | → `stale` after staleness timeout |
| `error` | Job failed with error | None |
| `progress` | Job currently running | → `timeout` after progress timeout |
| `timeout` | Progress job exceeded timeout | None |
| `stale` | No update within staleness period | None |

## Health Status Logic

### Status Precedence (Highest to Lowest)

| Priority | Status | Meaning |
|----------|--------|---------|
| 1 | `unhealthy` | Any job has error, timeout, or stale status |
| 2 | `in_progress` | Any job has progress status (no unhealthy jobs) |
| 3 | `healthy` | All jobs have success status |
| 4 | `empty` | No jobs exist |

### Global Health (`GET /health`)

The overall health status is determined by evaluating all jobs across all groups:

1. `empty` - No jobs exist in the system
2. `unhealthy` - Any job has status: error, timeout, or stale
3. `in_progress` - Any job has status: progress (and no unhealthy jobs)
4. `healthy` - All jobs have status: success

### Per-Group Health (`GET /groups` response)

Each group's health is calculated independently using the same precedence rules, applied only to jobs within that group.

**Example Response:**
```json
{
  "groups": [
    {
      "name": "backups",
      "health": "healthy",
      "job_count": 5,
      "status_counts": {"success": 5}
    },
    {
      "name": "monitoring",
      "health": "unhealthy",
      "job_count": 3,
      "status_counts": {"success": 1, "error": 1, "progress": 1}
    }
  ],
  "global_health": "unhealthy"
}
```

In this example, even though "backups" is healthy, the global health is "unhealthy" because "monitoring" contains an error.

## Pagination & Filtering

**Convention.** Pagination is **opt-in** and uses `limit`/`offset` (not page numbers).
With no params an endpoint returns its full result set, so existing clients are
unaffected. The response keeps its normal flat shape — `{ <collection>, "total": <count> }`,
with no nested `pagination` object — and `total` is always the full matching count,
independent of the returned window. A requested `limit` must be a positive integer and
is clamped to `Config.MAX_JOBS_PAGE_SIZE` (500); `offset` must be non-negative. Invalid
values return `400`. Parsing is centralized in `parse_pagination()` so all paginated
endpoints behave identically.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | (none) | Max items to return; positive, clamped to 500. Omit for all. |
| `offset` | integer | 0 | Items to skip (non-negative). |

### `GET /jobs` (implemented)

Supports `limit`/`offset` plus a `status` filter (comma-separated, e.g.
`error,timeout`). Jobs are ordered by `updated_at` descending. Returns
`{ "jobs": [...], "total": <count> }`.

### `GET /groups/<name>/jobs` (implemented)

Supports `limit`/`offset`. Jobs are ordered by `updated_at` descending. Returns
`{ "group": {...}, "jobs": [...], "total": <count> }`, where `total` is the group's
full job count.

### `GET /groups` (not paginated)

Returns the full list of groups with health summaries. Group counts are typically
small, so pagination is not implemented. If it is ever needed it should follow the
same opt-in `limit`/`offset` convention above (returning
`{ "groups": [...], "total": N }`).

**Possible future filters** (not yet implemented): a `health` filter on `GET /groups`,
and `status`/`sort` params on `GET /groups/<name>/jobs`.

## Timezone Handling

All timestamps are stored and returned in **UTC** to avoid ambiguity.

### Storage

- Database stores all `DATETIME` columns as UTC (no timezone offset)
- Server generates timestamps using `datetime.utcnow()` or `datetime.now(timezone.utc)`

### API Responses

- All timestamps returned in ISO 8601 format with `Z` suffix: `2024-01-15T10:30:00Z`
- Clients are responsible for converting to local timezone for display

### Timeout Calculations

- Background task compares `updated_at` against server's current UTC time
- No client-provided timestamps are used for timeout calculations, preventing clock skew issues

**Example:**
```python
# Server-side timeout check (always uses server time)
cutoff = datetime.now(timezone.utc) - timedelta(minutes=timeout_minutes)
expired_jobs = Job.query.filter(
    Job.status == 'progress',
    Job.updated_at < cutoff
).all()
```

## Data Retention & Cleanup

The jobs table grows unbounded without cleanup. This section defines retention policies to maintain performance and storage stability.

### Retention Policy

| Data Type | Default Retention | Rationale |
|-----------|-------------------|-----------|
| Active jobs (success, progress) | Indefinite | Current state needed for dashboard |
| Stale/timeout jobs | 30 days | Historical context, then cleanup |
| Error jobs | 90 days | Longer retention for debugging |
| Empty groups (no jobs) | 7 days | Auto-cleanup orphaned groups |

### Cleanup Endpoint

| Method | Endpoint | Description |
|--------|----------|-------------|
| DELETE | `/admin/cleanup` | Trigger manual cleanup (returns count of deleted records) |
| GET | `/admin/stats` | Database statistics (job counts, table sizes) |

**`DELETE /admin/cleanup` Request:**
```json
{
  "older_than_days": 30,
  "statuses": ["stale", "timeout"],
  "dry_run": true
}
```

**Response:**
```json
{
  "deleted_jobs": 150,
  "deleted_groups": 3,
  "dry_run": true
}
```

### Automatic Cleanup (Optional)

If enabled via config, cleanup runs as part of the background task:

```python
# In config table
cleanup_enabled: true
cleanup_retention_days: 30
cleanup_statuses: ["stale", "timeout"]
```

The cleanup runs once per day (not every 60s like timeout checks) to avoid excessive database churn.

### Storage Monitoring

Monitor database size and row counts to detect unbounded growth:

```sql
-- SQLite database size
SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size();

-- Row counts by status
SELECT status, COUNT(*) FROM jobs GROUP BY status;
```

---

## Design Decisions & Rationale

This section documents key architectural decisions and their rationale.

### Q: Why no authentication?

**Decision:** No authentication by default.

**Rationale:**
- StatShed is designed as an internal status dashboard for localhost/intranet use
- The data (job statuses) is operational, not sensitive user data
- Simplicity enables easy integration with cron jobs, scripts, and internal tooling
- For external exposure, authentication should be handled at the reverse proxy layer (nginx, Caddy) rather than in the application

### Q: Why SQLite instead of PostgreSQL?

**Decision:** SQLite as the default database.

**Rationale:**
- Zero configuration required - works out of the box
- Single-file database simplifies backup and deployment
- Sufficient for expected workloads (< 100 concurrent writes/sec, < 10GB data)
- Easy migration path: change `DATABASE_URL` to PostgreSQL connection string

**When to switch to PostgreSQL:**
- Multiple application servers need to share the database
- Write concurrency exceeds SQLite's capacity
- Advanced features needed (LISTEN/NOTIFY, full-text search, JSON operators)

### Q: Why best-effort WebSocket updates instead of guaranteed delivery?

**Decision:** Best-effort live updates with client-side refresh on reconnect.

**Rationale:**
- Simplifies server implementation (no event log, no sequence IDs)
- Dashboard clients care about current state, not historical events
- Reconnect + full refresh is simple and reliable for the UI use case
- Avoids complexity of event replay, deduplication, and storage

**Future enhancement path:** If needed, add `last_event_id` to events and store recent events in a ring buffer for replay on reconnect
