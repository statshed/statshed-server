# StatShed REST API Reference

StatShed is a status dashboard for tracking job statuses across groups. This document
describes the HTTP API served by the StatShed server. For a friendly introduction and the
quick start, see the [README](../README.md).

## Base URL

All API endpoints are served under the **`/api`** prefix on the server's single port.

- The standalone binary listens on **`http://127.0.0.1:7828`** by default (configurable via
  the `HOST` and `PORT` environment variables).
- The Docker setup maps host port **`7827`** to the container's `7828`, so requests go to
  **`http://localhost:7827/api/...`**.

The examples below use `http://localhost:7827`.

## Content type

- **Requests:** `application/json` (or `multipart/form-data` for status reports that include
  a log file).
- **Responses:** `application/json` — except `GET /api/events`, which is a
  `text/event-stream` (see [Real-Time Events](#real-time-events-sse)).

## Authentication

None. Every request is trusted — run StatShed on a trusted network or behind an
authenticating reverse proxy. See [Security](../README.md#security).

## Names & validation

`group` and `job` names are **normalized to lowercase**, trimmed, and must match
`^[a-z0-9._-]+$` (letters, digits, dot, dash, underscore), up to 255 characters. A name with
other characters (e.g. spaces) is rejected with `400`.

## Error format

Every error returns a JSON envelope:

```json
{ "error": "validation_error", "message": "status is required", "field": "status" }
```

- `error` — a stable slug: `validation_error`, `bad_request`, `not_found`, `invalid_state`,
  `method_not_allowed`, `payload_too_large`, `unsupported_media_type`,
  `internal_server_error`, or `http_error`.
- `message` — a human-readable description.
- `field` — present when the error concerns a specific request field. Cross-field validation
  errors use a `fields` object instead.

Status codes used: `200`, `201`, `400`, `404`, `405`, `413`, `415`, `500`.

---

## Endpoints

### GET /api/health

Overall health summary across all jobs.

```json
{
  "status": "unhealthy",
  "total_jobs": 10,
  "healthy": 8,
  "unhealthy": 1,
  "acked": 0,
  "in_progress": 1,
  "by_status": { "success": 8, "error": 1, "progress": 1, "timeout": 0, "stale": 0 }
}
```

- `status` — overall state, by precedence: `empty` (no jobs) > `unhealthy` > `in_progress` >
  `healthy`.
- `unhealthy` excludes acknowledged jobs; `by_status` holds raw counts (including acked).

---

### POST /api/status

Submit or update a job status. Creates the group and job if they don't exist.

**Body (JSON):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `group` | string | yes | Group name (normalized; ≤255 chars) |
| `job` | string | yes | Job name (normalized; ≤255 chars) |
| `status` | string | yes | One of `success`, `error`, `progress`, `timeout`, `stale`. Clients normally send `success`/`error`/`progress`; `timeout` and `stale` are usually assigned by the server. |
| `message` | string | no | Optional message (≤4096 chars) |

```json
{ "group": "ci", "job": "backend-tests", "status": "success", "message": "All 42 passed" }
```

**With a log file** use `multipart/form-data` (fields as form parts, the log as a file part
named `log`):

```bash
curl -X POST http://localhost:7827/api/status \
  -F group=ci -F job=backend-tests -F status=error -F message="Build failed" \
  -F log=@build.log
```

- Logs require `multipart/form-data` (not JSON).
- A log longer than `MAX_LOG_LINES` (default 1000) is truncated to the last N lines.
- Log uploads can be disabled with `LOG_UPLOAD_ENABLED=false`; a log sent while disabled is
  ignored and the response includes a `warning`.
- Each new report with a log replaces the previous log.

**Response (`201 Created`):**

```json
{ "success": true, "job": { /* job object, see below */ } }
```

**Errors:** `400` (missing/invalid field, length exceeded), `413` (body over 1 MB), `500`.

---

### GET /api/jobs

List jobs across all groups, newest first (`updated_at` descending).

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `status` | string | (all) | Comma-separated statuses to filter by, e.g. `error,timeout`. Invalid values → `400`. |
| `limit` | integer | (none) | Max jobs to return; must be positive, clamped to the server max (`MAX_JOBS_PAGE_SIZE`, default 500). Omit to return all. |
| `offset` | integer | 0 | Jobs to skip, for paging; must be non-negative. |

Pagination is opt-in: with no `limit`/`offset` the full result set is returned. `total` is
always the full matching count, independent of the page window.

```json
{ "jobs": [ { /* job object */ } ], "total": 1 }
```

**Errors:** `400` (invalid `status`, `limit`, or `offset`).

---

### POST /api/jobs/{id}/ack

Acknowledge a single unhealthy job, clearing it from the unhealthy count. Only `error`,
`timeout`, and `stale` jobs can be acknowledged.

**Response (`200`):** `{ "job": { /* job object, now acked */ } }`

**Errors:** `400` (`invalid_state` — job is not in an acknowledgeable status), `404`.

---

### DELETE /api/jobs/{id}

Delete a job.

```json
{ "deleted_job": { /* job object */ }, "group_id": 1, "group_name": "ci" }
```

**Errors:** `404`.

---

### POST /api/groups/{name}/ack

Acknowledge all unhealthy jobs in a group.

**Response (`200`):** `{ "acked_count": 3, "group": "ci" }`

**Errors:** `404` (group does not exist).

---

### POST /api/ack-all

Acknowledge every unhealthy job across all groups.

**Response (`200`):** `{ "acked_count": 7 }`

---

### GET /api/groups

List all groups with health summary information.

```json
{
  "groups": [
    {
      "id": 1,
      "name": "ci",
      "progress_timeout_minutes": null,
      "staleness_timeout_hours": null,
      "staleness_enabled": false,
      "expiration_timeout_hours": null,
      "created_at": "2026-01-17T10:00:00Z",
      "job_count": 5,
      "health": "healthy",
      "unhealthy_count": 0,
      "acked_count": 0,
      "status_counts": { "success": 5, "error": 0, "progress": 0, "timeout": 0, "stale": 0 }
    }
  ]
}
```

Per-group timeout fields are `null` when the group uses the global defaults.

---

### GET /api/groups/{name}/jobs

Jobs within a group, newest first. Supports the same opt-in `limit`/`offset` pagination as
`GET /api/jobs`; `total` is the full job count for the group.

```json
{
  "group": { "id": 1, "name": "ci", "progress_timeout_minutes": null, "staleness_timeout_hours": null, "staleness_enabled": false, "expiration_timeout_hours": null, "created_at": "2026-01-17T10:00:00Z" },
  "jobs": [ { /* job object */ } ],
  "total": 1
}
```

**Errors:** `400` (invalid `limit`/`offset`), `404` (group does not exist).

---

### GET /api/groups/{name}/jobs/{job}/log

Retrieve a job's stored log.

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `tail` | integer | 1000 | Number of lines to return from the end of the log. A missing, non-integer, or non-positive value falls back to 1000. |
| `all` | boolean | false | Return the full stored log (ignores `tail`). |

```json
{
  "log": "Running tests...\nTest 1 passed\n",
  "line_count": 2,
  "truncated": false,
  "total_line_count": 2
}
```

- `line_count` — number of lines in the returned `log`.
- `total_line_count` — total lines stored for the job.
- `truncated` — `true` when this response is a tail (the stored log has more lines than were
  returned), i.e. `total_line_count > line_count`.

**Errors:** `404` (job does not exist or has no log).

---

### Configuration

#### GET /api/config

Global timeout/staleness/expiration settings.

```json
{ "progress_timeout_minutes": 5, "staleness_timeout_hours": 24, "expiration_timeout_hours": 24 }
```

#### PUT /api/config

Update one or more global settings. Only the fields you supply change, but you must supply
at least one — an empty object `{}` is rejected with `400`.

| Field | Range | Description |
|-------|-------|-------------|
| `progress_timeout_minutes` | 1–10080 | Minutes before a `progress` job becomes `timeout`. |
| `staleness_timeout_hours` | 1–8760 | Hours before a `success` job becomes `stale`. |
| `expiration_timeout_hours` | 1–8760 | Hours before a job is removed entirely. |

Returns the full config object (same shape as `GET`). **Errors:** `400` (out-of-range value).

#### GET /api/groups/{name}/config

Per-group overrides plus the effective (in-use) values.

```json
{
  "group": "ci",
  "group_name": "ci",
  "progress_timeout_minutes": null,
  "staleness_enabled": false,
  "staleness_timeout_hours": null,
  "expiration_timeout_hours": null,
  "effective_progress_timeout_minutes": 5,
  "effective_staleness_timeout_hours": 24,
  "effective_expiration_timeout_hours": 24
}
```

- `null` override fields mean the group uses the global default.
- `effective_*` show the values actually applied. `group` is a legacy alias for `group_name`.

**Errors:** `404` (group does not exist).

#### PUT /api/groups/{name}/config

Update group overrides. Supply at least one field (an empty object `{}` is rejected with
`400`); send `null` to clear an override (revert to global). Ranges match `PUT /api/config`.

| Field | Type | Description |
|-------|------|-------------|
| `progress_timeout_minutes` | integer or null | Progress-timeout override. |
| `staleness_timeout_hours` | integer or null | Staleness override. |
| `staleness_enabled` | boolean | Whether `success` jobs in this group ever go `stale`. |
| `expiration_timeout_hours` | integer or null | Expiration override. |

Cross-field rule: when `staleness_enabled` is true, the effective staleness window must be
strictly less than the effective expiration window, otherwise `400`.

Returns the full group config object (same shape as `GET`). **Errors:** `400`, `404`.

---

### Admin

#### GET /api/admin/stats

Storage and aggregate counts, for operator/admin tooling.

```json
{
  "total_jobs": 42,
  "total_groups": 6,
  "jobs_by_status": { "success": 40, "error": 1, "progress": 1, "timeout": 0, "stale": 0 },
  "database_size_bytes": 122880
}
```

#### DELETE /api/admin/cleanup

Bulk-delete old jobs (and groups left empty).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `older_than_days` | integer | yes | Delete jobs not updated within this many days (≥1). |
| `statuses` | string[] | no | Statuses to delete (default `["stale", "timeout"]`). |
| `dry_run` | boolean | no | If true, report what would be deleted without deleting. |

```json
{ "deleted_jobs": 12, "deleted_groups": 1, "dry_run": false }
```

**Errors:** `400` (missing/invalid `older_than_days`, `statuses`, or `dry_run`).

---

## Job object

Returned by status, job, and group-job endpoints:

```json
{
  "id": 1,
  "group_id": 1,
  "group_name": "ci",
  "name": "backend-tests",
  "status": "success",
  "message": "All 42 passed",
  "acked": false,
  "acked_at": null,
  "expires_at": "2026-01-18T12:00:00Z",
  "has_log": true,
  "log_line_count": 120,
  "log_truncated": false,
  "log_updated_at": "2026-01-17T12:00:00Z",
  "updated_at": "2026-01-17T12:00:00Z",
  "created_at": "2026-01-17T10:00:00Z"
}
```

Timestamps are whole-second UTC (`YYYY-MM-DDTHH:MM:SSZ`); nullable fields are present as
`null` rather than omitted.

## Job status values

| Status | Usually set by | Description |
|--------|----------------|-------------|
| `success` | client | Job completed successfully. |
| `error` | client | Job failed. |
| `progress` | client | Job is currently running. |
| `timeout` | server | A `progress` job exceeded its progress timeout. |
| `stale` | server | A `success` job exceeded its staleness window (groups with staleness enabled). |

`POST /api/status` accepts all five values; clients normally send `success`/`error`/`progress`
and let the background worker assign `timeout`/`stale`.

## Background maintenance

A worker runs about every 60 seconds and:

- marks `progress` jobs as `timeout` once they exceed `progress_timeout_minutes`;
- marks `success` jobs as `stale` once they exceed `staleness_timeout_hours`, but only for
  groups with staleness enabled (`staleness_enabled` is off by default);
- removes jobs that have passed `expiration_timeout_hours`.

Per-group overrides take precedence over the global settings.

---

## Real-Time Events (SSE)

The server streams real-time updates over [Server-Sent Events](https://developer.mozilla.org/docs/Web/API/Server-sent_events).
Connect an `EventSource` to `GET /api/events` (same host/port). The stream is
`text/event-stream`, is never compressed, and emits a `: ping` comment heartbeat about every
25 seconds. `EventSource` reconnects automatically after a drop, so re-fetch your data on
reconnect to recover any events missed during the outage.

```js
const events = new EventSource('/api/events')
events.addEventListener('status_update', (e) => {
  const data = JSON.parse(e.data)
  // refresh the affected views
})
```

Every event's `data` is a JSON object whose `schema_version` is `1`; id arrays are sorted
ascending and timestamps are whole-second UTC (`YYYY-MM-DDTHH:MM:SSZ`).

| Event | When | Payload |
|-------|------|---------|
| `status_update` | A job was created or updated | `{ "schema_version": 1, "job": { ... }, "group_id": int, "group_name": str, "previous_status": str\|null }` |
| `group_created` | A new group was created (its first job report) | `{ "schema_version": 1, "group": { ... } }` |
| `jobs_acked` | One or more jobs were acknowledged | `{ "schema_version": 1, "job_ids": [int], "group_id": int\|null, "group_name": str\|null, "acked_count": int, "timestamp": str }` |
| `job_deleted` | A job was deleted | `{ "schema_version": 1, "job_id": int, "job_name": str, "group_id": int, "group_name": str, "timestamp": str }` |
| `health_update` | The background worker transitioned jobs | `{ "schema_version": 1, "affected_job_ids": [int], "affected_group_ids": [int], "transition_type": "timeout"\|"stale", "timestamp": str }` |
| `job_expired` | A job passed its expiration and was removed | `{ "schema_version": 1, "job_id": int, "job_name": str, "group_id": int, "group_name": str, "timestamp": str }` |

On a global ack-all, `jobs_acked` has `group_id`/`group_name` of `null`.

---

## Input limits

| Field | Maximum |
|-------|---------|
| Group name | 255 characters |
| Job name | 255 characters |
| Message | 4096 characters |
| Request body | 1 MB |

---

## Environment variables

The server is configured via environment variables; see the
[Configuring the server](../README.md#configuring-the-server) section of the README for the
server settings (`HOST`, `PORT`, `DATABASE_URL`, `DEBUG`, `CORS_ORIGINS`, `LOG_UPLOAD_ENABLED`,
`MAX_LOG_LINES`, `MAX_JOBS_PAGE_SIZE`, `STATIC_DIR`/`STATIC_DISABLED`). A couple of
internal/test-only variables also exist (`HEALTHCHECK_URL`, `STATSHED_TEST_HOOKS`).

## CLI client

The `statshed` CLI ([Go](https://github.com/statshed/statshed-gocli) /
[Python](https://github.com/statshed/statshed-pycli)) wraps this API for use in scripts and
pipelines. See [CLI clients](../README.md#cli-clients).
