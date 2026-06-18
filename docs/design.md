# StatShed - Status Dashboard Design Document

A real-time status dashboard for monitoring cluster job operations.

## Project Overview

```
statshed/
├── backend/          # Flask API + SQLite + WebSocket
├── frontend/         # React app (Vite)
├── cli/              # Python CLI tool
├── tests/            # Playwright + pytest tests
└── README.md
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Backend | Python, Flask, Flask-SocketIO, SQLAlchemy |
| Database | SQLite |
| Frontend | React (Vite) |
| Real-time | WebSockets |
| Testing | Pytest, Playwright |
| Linting | Ruff |

## Todo Checklist

### Project Setup

- [x] Set up project structure (backend/, frontend/, cli/, tests/)
- [x] Initialize Python backend with uv and dependencies
- [x] Set up Ruff for linting and formatting

### Backend (Flask + SQLite)

- [x] Design SQLite schema (groups, jobs, configuration)
- [x] Implement REST API endpoints:
  - [x] `POST /status` - Submit job status (group, job, status, message)
  - [x] `GET /groups` - List all groups with summary health
  - [x] `GET /groups/<name>/jobs` - Get jobs in a group
  - [x] `GET /health` - Overall health summary
  - [x] `GET /config` - Get global timeout/staleness settings
  - [x] `PUT /config` - Update global timeout/staleness settings
  - [x] `GET /groups/<name>/config` - Get group-specific overrides
  - [x] `PUT /groups/<name>/config` - Update group-specific overrides
- [x] Implement WebSocket for real-time push updates
- [x] Implement background task for timeout/staleness checking
- [x] Write pytest unit tests (30 tests passing)

### Frontend (React)

- [x] Initialize React app with Vite
- [x] Dashboard overview (health summary, group list)
- [x] Group detail view (job list with statuses)
- [x] WebSocket client for live updates
- [x] Styling and UI polish (dark mode support, responsive design)

### CLI Tool

- [x] Python CLI for submitting status (commands: submit, health, groups, jobs, config)

### Testing & Documentation

- [x] Playwright end-to-end tests
- [x] README with API docs and cURL/wget examples
- [x] Development scripts

## Key Design Decisions

| Feature | Implementation |
|---------|---------------|
| **Storage** | SQLite via SQLAlchemy |
| **Real-time** | Flask-SocketIO (WebSockets) |
| **Progress timeout** | Configurable (default: 5 min), marks as "Timed Out" error |
| **Staleness** | Configurable (default: 24h), any job without update becomes stale error |
| **Group overrides** | Each group can override global timeout/staleness values |

## API Design

### Submit Job Status

```bash
# Submit a success status
curl -X POST http://localhost:7828/status \
  -H "Content-Type: application/json" \
  -d '{"group": "backups", "job": "db-backup", "status": "success"}'

# Submit an error with message
curl -X POST http://localhost:7828/status \
  -H "Content-Type: application/json" \
  -d '{"group": "backups", "job": "db-backup", "status": "error", "message": "Disk full"}'

# Submit progress status
curl -X POST http://localhost:7828/status \
  -H "Content-Type: application/json" \
  -d '{"group": "backups", "job": "db-backup", "status": "progress", "message": "50% complete"}'
```

### Query Endpoints

```bash
# Get overall health
curl http://localhost:7828/health

# List all groups
curl http://localhost:7828/groups

# Get jobs in a specific group
curl http://localhost:7828/groups/backups/jobs

# Get global config
curl http://localhost:7828/config

# Update global config
curl -X PUT http://localhost:7828/config \
  -H "Content-Type: application/json" \
  -d '{"progress_timeout_minutes": 10, "staleness_timeout_hours": 48}'
```

### CLI Usage

```bash
# Submit status via CLI
statshed submit --group backups --job db-backup --status success
statshed submit --group backups --job db-backup --status error --message "Disk full"
statshed submit -g backups -j db-backup -s progress -m "Running..."
```

## Database Schema

### Groups Table

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Unique group name |
| progress_timeout_minutes | INTEGER | Override for progress timeout (nullable) |
| staleness_timeout_hours | INTEGER | Override for staleness timeout (nullable) |
| created_at | DATETIME | When group was created |

### Jobs Table

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| group_id | INTEGER | Foreign key to groups |
| name | TEXT | Job name (unique within group) |
| status | TEXT | success, error, progress, timeout, stale |
| message | TEXT | Optional status message |
| updated_at | DATETIME | Last status update time |
| created_at | DATETIME | When job was first seen |

### Config Table

| Column | Type | Description |
|--------|------|-------------|
| key | TEXT | Primary key (config key name) |
| value | TEXT | JSON-encoded config value |

## Status Types

| Status | Description | Display Color |
|--------|-------------|---------------|
| `success` | Job completed successfully | Green |
| `error` | Job failed | Red |
| `progress` | Job currently running | Blue/Yellow |
| `timeout` | Progress job timed out waiting for update | Red |
| `stale` | Job hasn't reported in staleness period | Orange/Red |

## WebSocket Events

| Event | Direction | Description |
|-------|-----------|-------------|
| `status_update` | Server -> Client | New job status received |
| `group_created` | Server -> Client | New group was created |
| `health_update` | Server -> Client | Overall health changed |

## Frontend Pages

### Dashboard Overview

- Overall health indicator (all green, some errors, etc.)
- List of groups with:
  - Group name
  - Job count
  - Status summary (X success, Y errors, Z in progress)
  - Click to drill down

### Group Detail View

- Group name and health status
- Configuration overrides (if any)
- List of jobs with:
  - Job name
  - Current status
  - Last message
  - Last updated timestamp
