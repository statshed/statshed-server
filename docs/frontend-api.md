# Frontend API Documentation

This document describes the REST API and WebSocket interface that the StatShed frontend uses to communicate with the backend.

## Base Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| API URL | `http://localhost:7828` | Configured via `VITE_API_URL` environment variable |
| Content-Type | `application/json` | All requests and responses |
| Request Timeout | 30 seconds | |

## REST Endpoints

### Health Check

**GET `/health`**

Returns overall health summary across all jobs.

**Response (200):**
```json
{
  "status": "healthy" | "unhealthy" | "in_progress" | "empty",
  "total_jobs": 10,
  "healthy": 8,
  "unhealthy": 1,
  "in_progress": 1,
  "by_status": {
    "success": 8,
    "error": 1,
    "progress": 1,
    "timeout": 0,
    "stale": 0
  }
}
```

---

### Status Submission

**POST `/status`**

Submit or update a job status. Creates the group and/or job if they don't exist.

**Request Body:**
```json
{
  "group": "string (required, max 255 chars)",
  "job": "string (required, max 255 chars)",
  "status": "success" | "error" | "progress" | "timeout" | "stale",
  "message": "string (optional, max 4096 chars)"
}
```

**Response (201):**
```json
{
  "success": true,
  "job": {
    "id": 1,
    "group_id": 1,
    "group_name": "backups",
    "name": "db-backup",
    "status": "success",
    "message": "All tests passed",
    "updated_at": "2025-01-17T12:00:00Z",
    "created_at": "2025-01-17T10:00:00Z"
  }
}
```

**Errors:**
- `400`: Missing required fields, invalid status, or length exceeded
- `500`: Database error

---

### Groups

**GET `/groups`**

List all groups with health summary.

**Response (200):**
```json
{
  "groups": [
    {
      "id": 1,
      "name": "backups",
      "progress_timeout_minutes": null,
      "staleness_timeout_hours": null,
      "created_at": "2025-01-17T10:00:00Z",
      "job_count": 5,
      "health": "healthy" | "unhealthy" | "in_progress" | "empty",
      "status_counts": {
        "success": 5,
        "error": 0,
        "progress": 0,
        "timeout": 0,
        "stale": 0
      }
    }
  ]
}
```

---

**GET `/groups/{name}/jobs`**

Get all jobs in a specific group.

**Path Parameters:**
- `name` - Group name (URL-encoded)

**Response (200):**
```json
{
  "group": {
    "id": 1,
    "name": "backups",
    "progress_timeout_minutes": null,
    "staleness_timeout_hours": null,
    "created_at": "2025-01-17T10:00:00Z"
  },
  "jobs": [
    {
      "id": 1,
      "group_id": 1,
      "group_name": "backups",
      "name": "db-backup",
      "status": "success",
      "message": "All tests passed",
      "updated_at": "2025-01-17T12:00:00Z",
      "created_at": "2025-01-17T10:00:00Z"
    }
  ]
}
```

**Errors:**
- `404`: Group not found

---

### Global Configuration

**GET `/config`**

Retrieve global configuration settings.

**Response (200):**
```json
{
  "progress_timeout_minutes": 5,
  "staleness_timeout_hours": 24
}
```

---

**PUT `/config`**

Update global configuration settings.

**Request Body:**
```json
{
  "progress_timeout_minutes": 10,
  "staleness_timeout_hours": 48
}
```

**Response (200):** Updated config values

**Errors:**
- `400`: Invalid values (must be positive integers)

---

### Group-Specific Configuration

**GET `/groups/{name}/config`**

Get group-specific configuration overrides.

**Path Parameters:**
- `name` - Group name (URL-encoded)

**Response (200):**
```json
{
  "group": "backups",
  "progress_timeout_minutes": null,
  "staleness_timeout_hours": 48,
  "effective_progress_timeout_minutes": 5,
  "effective_staleness_timeout_hours": 48
}
```

Note: `null` values indicate the group uses global defaults. The `effective_*` fields show the actual values in use (either group-specific or global fallback).

**Errors:**
- `404`: Group not found

---

**PUT `/groups/{name}/config`**

Update group-specific configuration overrides.

**Path Parameters:**
- `name` - Group name (URL-encoded)

**Request Body:**
```json
{
  "progress_timeout_minutes": 30,
  "staleness_timeout_hours": 72
}
```

Set a field to `null` to revert to global defaults.

**Response (200):** Updated config with effective values

**Errors:**
- `400`: Invalid values
- `404`: Group not found

---

## WebSocket Events (Socket.IO)

The frontend connects to the same base URL using Socket.IO for real-time updates.

### Events (Server → Client)

| Event | Payload | Description |
|-------|---------|-------------|
| `status_update` | `{ "job": Job }` | A job's status has been updated |
| `group_created` | `{ "group": Group }` | A new group was created |
| `health_update` | `{}` | Signal to refresh health data (e.g., after timeout transitions) |

---

## Data Models

### Group

```typescript
interface Group {
  id: number;
  name: string;                          // unique, max 255 chars
  progress_timeout_minutes: number | null;
  staleness_timeout_hours: number | null;
  created_at: string;                    // ISO 8601 datetime
}
```

### Job

```typescript
interface Job {
  id: number;
  group_id: number;
  group_name: string;
  name: string;                          // unique within group, max 255 chars
  status: 'success' | 'error' | 'progress' | 'timeout' | 'stale';
  message: string | null;                // max 4096 chars
  updated_at: string;                    // ISO 8601 datetime
  created_at: string;                    // ISO 8601 datetime
}
```

### Config

```typescript
interface Config {
  progress_timeout_minutes: number;      // default: 5
  staleness_timeout_hours: number;       // default: 24
}
```

---

## Job Status Values

| Status | Description | Auto-Transition |
|--------|-------------|-----------------|
| `success` | Job completed successfully | → `stale` after staleness timeout |
| `error` | Job failed with error | None |
| `progress` | Job currently running | → `timeout` after progress timeout |
| `timeout` | Progress job exceeded timeout | None |
| `stale` | No update within staleness period | None |

---

## Input Limits

| Field | Maximum Length |
|-------|----------------|
| Group name | 255 characters |
| Job name | 255 characters |
| Message | 4096 characters |
| Request body | 1 MB |

---

## Frontend API Client

The API client is implemented in `src/api.ts`:

```typescript
import * as api from './api';

// Health
await api.getHealth();

// Groups
await api.getGroups();
await api.getGroupJobs(groupName);

// Global config
await api.getConfig();
await api.updateConfig({ progress_timeout_minutes: 10 });

// Group config
await api.getGroupConfig(groupName);
await api.updateGroupConfig(groupName, { staleness_timeout_hours: 48 });

// Submit status
await api.submitStatus({
  group: 'backups',
  job: 'db-backup',
  status: 'success',
  message: 'Backup completed'
});
```

---

## WebSocket Hook

The WebSocket connection is managed via a React hook in `src/hooks/useSocket.ts`:

```typescript
import { useSocket } from './hooks/useSocket';

useSocket({
  onStatusUpdate: (data: { job: Job }) => {
    // Handle job status update
  },
  onGroupCreated: (data: { group: Group }) => {
    // Handle new group
  },
  onHealthUpdate: () => {
    // Refresh health data
  }
});
```

---

## HTTP Status Codes

| Code | Meaning |
|------|---------|
| `200` | Success (GET, PUT) |
| `201` | Created (POST) |
| `400` | Bad Request (validation errors) |
| `404` | Not Found |
| `500` | Server Error |
