# Log Upload + Viewer PRD / Implementation Plan

Goal: Add optional log file upload to existing status update flow (CLI `submit` -> backend `POST /status` endpoint). Backend can disable log uploads via env flag and enforce a configurable max size (target ~1000 lines). Frontend shows availability of logs and provides a joyful modal viewer with error highlighting, jump controls, and large-log handling (last 1000 lines with “Show all”). Logs expire alongside the associated status record; when status is updated, old log is replaced.

## Scope
- CLI: allow attaching optional log file to `submit`.
- Backend: accept/store log (when enabled), enforce config limits, expose log availability/metadata on `POST /status` and status read APIs, serve logs to frontend, and handle expiration/replacement.
- Frontend: show log availability in status UI, modal viewer with UX features, and graceful handling when logs are absent or upload disabled.

## Non-Goals
- Real-time log streaming.
- Multi-log history per status (only latest log retained).

## Assumptions / Constraints
- Backend endpoint is `POST /status` as documented in `restapi.md`.
- Log viewer is a separate modal.
- For large logs, default view shows last 1000 lines; “Show all” loads full log.

## Open Questions (fill in if needed)
- [ ] Confirm whether log metadata should also appear in `GET /groups` summary (currently only job counts)
- [ ] Confirm auth/permissions for viewing logs (assumed same as status read)

---

## Backend Checklist

### Config & Feature Flag
- [x] Add env flag (e.g., `LOG_UPLOAD_ENABLED`) with default = true
- [x] Add configurable max log size/line limit (e.g., `MAX_LOG_LINES` or `MAX_LOG_BYTES`)
- [x] Document new env vars in backend config docs

### API Contract
- [x] Extend `POST /status` to accept optional log file payload
  - [x] Use `multipart/form-data` when a log is present; keep JSON body for no-log submissions
  - [x] Fields: `group`, `job`, `status`, `message` (text) + `log` (file, text/plain)
  - [x] Update `restapi.md` request schema and examples
- [x] Extend `POST /status` response `job` object to include log metadata
  - [x] Proposed fields: `has_log`, `log_line_count`, `log_truncated`, `log_updated_at`
- [x] Add `GET /groups/{name}/jobs/{job}/log` to retrieve log content
  - [x] Query params: `tail=1000` (default), `all=true`
  - [x] Response: `{ "log": "...", "line_count": 900, "truncated": false }`

### Storage & Lifecycle
- [x] Store log content linked to job/status record (db/blob/fs)
- [x] Enforce max size on upload
  - [ ] Reject too-large logs with clear error
  - [x] Optional: truncate to last N lines if policy is truncate vs reject
- [x] Ensure logs expire/deleted when associated status expires (same lifecycle)
- [x] Ensure new status updates replace old log (delete previous log)

### Validation & Errors
- [x] When log uploads disabled, accept status update but ignore log (status succeeds)
  - [x] Return explicit warning field in response for CLI/FE to display
- [x] Validate file type/encoding (plain text)
- [x] Ensure log retrieval respects auth

### Tests
- [x] Unit tests for config flags and size enforcement
- [x] Integration tests for submit with/without log, disabled flag, and retrieval
- [x] Expiration/replacement tests to ensure logs are deleted

---

## CLI Checklist (../cli)

### UX / Flags
- [x] Add optional `--log <path>` flag to `submit`
- [x] Provide friendly error if file missing/unreadable/too large
- [x] If backend indicates log uploads disabled, show non-blocking warning

### Request Changes
- [x] Include log file in `POST /status` payload per updated API (multipart or inline)
- [x] If log too large client-side, warn and allow submission without log
  - [x] Optional: auto-trim to last N lines (confirm with product)

### Tests & Docs
- [x] Update CLI help text and docs for `submit`
- [x] Add tests for submit with/without log and disabled flag behavior

---

## Frontend Checklist (this repo)

### Status UI Changes
- [x] Show "Logs available" indicator when `hasLog=true`
- [x] Hide or disable log UI when no logs or uploads disabled
- [x] Add "View logs" action in status detail view

### Modal Log Viewer UX
- [x] New modal component with joyful, readable layout
  - [x] Monospace or coding-friendly font
  - [x] Comfortable line spacing and line numbers
  - [x] Highlight lines containing "error" (case-insensitive)
  - [x] SECURITY: Escape HTML/JS in log content to prevent XSS; consider stripping ANSI control sequences
- [x] Joyful touches (non-blocking)
  - [x] Soft gradient header + friendly microcopy (e.g., "Here's what happened")
  - [x] Subtle background pattern or glow behind log pane
  - [x] Animated scroll-to-error (100–200ms ease)
- [x] Navigation controls
  - [x] Jump to next error
  - [x] Jump to previous error
  - [x] Jump to top
  - [x] Jump to bottom
- [x] Large log handling
  - [x] Default load: last 1000 lines (server `tail=1000`)
  - [x] "Show all" button to fetch full stored log (note: may already be truncated at upload if original exceeded MAX_LOG_LINES)
  - [x] Loading and error states for full log fetch
  - [x] Explicit note when truncated (e.g., "Showing last 1000 lines" or "Log was truncated at upload")

### Data & API Integration
- [x] Extend status fetch to include log metadata (surface `has_log`, `log_line_count`, etc.)
- [x] Add log fetch call with `tail=1000` and `all=true`
- [x] Handle 404/no-log responses gracefully

### Tests
- [x] Component tests for log viewer controls
- [x] Visual/UX QA for highlighting and navigation
- [x] E2E: open modal, navigate errors, switch to full log

---

## Rollout / Backward Compatibility
- [x] Backend default: log uploads enabled with safe limits
- [x] CLI: `--log` optional; no behavior change without it
- [x] Frontend: log viewer only shows when metadata indicates logs

## Implementation Sequence (Suggested)
- [x] Backend: config + API contract updates in `restapi.md`
- [x] Backend: storage + lifecycle + retrieval
- [x] CLI: `submit --log` integration
- [x] Frontend: metadata UI + modal viewer
- [x] Tests + documentation updates across repos
