# Expiring Status Entries Implementation Plan

## Overview

Transform the status lifecycle from the current "stale after 24h" model to a more flexible system where:

- **Staleness** is disabled by default (opt-in warning state)
- **Expiration** auto-deletes jobs after a configurable time (default 24h)
- **All statuses expire** - success, error, timeout, progress, stale all auto-delete
- **Visual fade** indicates jobs approaching expiration (100% → 50% opacity)

## Current vs New Behavior

| Aspect | Current | New |
|--------|---------|-----|
| Staleness default | 24 hours (global) | Disabled (NULL) |
| Expiration | Manual cleanup only | Auto-delete after timeout (all statuses) |
| Visual feedback | Orange "stale" badge | Gradual opacity fade (100% → 50%) |
| Group config | Override staleness hours | Configure both staleness + expiration |
| Error/timeout jobs | Persist until ack + cleanup | Expire like all other jobs |

---

## Decisions to Lock (avoid ambiguity)

- **Staleness model:** Use `staleness_enabled` plus `staleness_timeout_hours` required when enabled (avoid nullable-as-enabled ambiguity).
- **Fade source of truth:** Client-computed fade based on `expires_at` (no `fade_percentage` field in API).
- **Expiration refresh:** On group config change, refresh `expires_at` for existing jobs in the group.

---

## Phase 1: Backend Model & Database Changes

### Database Schema

- [X] Add migration to modify `groups` table:
  - [X] Change `staleness_timeout_hours` default to NULL (disabled unless enabled)
  - [X] Add `expiration_timeout_hours` column (INTEGER, default 24)
  - [X] Add `staleness_enabled` column (BOOLEAN, default FALSE)

- [X] Add migration to modify `jobs` table:
  - [X] Add `expires_at` column (DATETIME, nullable) - computed on insert/update for efficient querying
  - [X] Add index on `expires_at` (or `(group_id, expires_at)` if queries are group-scoped)

### Model Updates

- [X] Update `Group` model in `models.py`:
  - [X] Add `expiration_timeout_hours` field with default 24
  - [X] Add `staleness_enabled` field with default False
  - [X] Update `staleness_timeout_hours` to be nullable with no default behavior

- [X] Update `Job` model in `models.py`:
  - [X] Add `expires_at` computed field

### Config Updates

- [X] Update `config.py`:
  - [X] Add `DEFAULT_EXPIRATION_TIMEOUT_HOURS = 24`
  - [X] Add `MIN_EXPIRATION_TIMEOUT_HOURS = 1`
  - [X] Add `MAX_EXPIRATION_TIMEOUT_HOURS = 8760` (1 year)
  - [X] Keep staleness config but document it's opt-in

---

## Phase 2: Backend Logic Changes

### Background Job Processor

- [X] Update `background.py` staleness checker:
  - [X] Only transition to stale if group has `staleness_enabled = True`
  - [X] Skip staleness check entirely for groups with staleness disabled

- [X] Add expiration processor in `background.py`:
  - [X] Query jobs where `expires_at <= now()` (all statuses: success, error, timeout, progress, stale)
  - [X] Delete expired jobs regardless of status or ack state
  - [X] Delete in batches (limit) to avoid long locks
  - [X] Emit WebSocket event for deleted jobs (`job_expired`)
  - [X] Log expiration deletions

### Status Submission

- [X] Update `POST /status` endpoint in `app.py`:
  - [X] Compute and set `expires_at` based on group's `expiration_timeout_hours`
  - [X] Refresh `expires_at` on each status update (extends the expiration)

### Group Config Changes

- [X] On `PUT /groups/<name>/config`, refresh `expires_at` for all existing jobs in the group
  - [X] Set to `updated_at + expiration_timeout_hours`

### API Endpoints

- [X] Update `GET /groups/<name>/config`:
  - [X] Return `expiration_timeout_hours` and `staleness_enabled`
  - [X] Return effective values with clear indication of what's enabled

- [X] Update `PUT /groups/<name>/config`:
  - [X] Accept `expiration_timeout_hours` (integer, required)
  - [X] Accept `staleness_enabled` (boolean)
  - [X] Accept `staleness_timeout_hours` (integer, only valid if staleness_enabled)
  - [X] Validate: if staleness_enabled, staleness_timeout must be < expiration_timeout

- [X] Update `GET /jobs` and related endpoints:
  - [X] Include `expires_at` in job response
  - [X] Do not include `fade_percentage` (computed on client)

### API Shape (examples)

- [X] `GET /groups/<name>/config` response:
```json
{
  "group_name": "payments",
  "progress_timeout_minutes": 30,
  "staleness_enabled": true,
  "staleness_timeout_hours": 6,
  "expiration_timeout_hours": 24,
  "effective_progress_timeout_minutes": 30,
  "effective_staleness_timeout_hours": 6,
  "effective_expiration_timeout_hours": 24
}
```

- [X] `PUT /groups/<name>/config` request:
```json
{
  "progress_timeout_minutes": 30,
  "staleness_enabled": true,
  "staleness_timeout_hours": 6,
  "expiration_timeout_hours": 24
}
```

- [X] `GET /jobs` response item (key fields):
```json
{
  "id": 123,
  "group_name": "payments",
  "name": "daily-reconcile",
  "status": "success",
  "message": "ok",
  "acked": false,
  "updated_at": "2026-01-25T16:10:00Z",
  "expires_at": "2026-01-26T16:10:00Z"
}
```

- [X] Validation error example (staleness >= expiration):
```json
{
  "error": "validation_error",
  "message": "staleness_timeout_hours must be less than expiration_timeout_hours",
  "fields": {
    "staleness_timeout_hours": "must be less than expiration_timeout_hours"
  }
}
```

---

## Phase 3: Frontend Type & API Updates

### TypeScript Types

- [X] Update `types/index.ts`:
  - [X] Add `expires_at: string | null` to `Job` interface
  - [X] Update `GroupConfig` to include new fields (`staleness_enabled`, `expiration_timeout_hours`, `effective_expiration_timeout_hours`)
  - [X] Add `JobExpiredEvent` interface for WebSocket events

### API Client

- [X] Update API hooks in `hooks/`:
  - [X] Update group config mutation to send new fields (`api/groups.ts` and `hooks/useGroupConfig.ts`)
  - [X] Ensure job queries handle new response shape (automatic via type definitions)
  - [X] Add `job_expired` WebSocket event handler in `contexts/SocketContext.tsx`
  - [X] Update test mocks in `test/mocks/handlers.ts` with new fields

---

## Phase 4: Frontend UI - Job Display

### Fade Effect Implementation

- [X] Create `useFadePercentage` hook:
  - [X] Takes `expires_at` and `updated_at`
  - [X] Returns current opacity (1.0 → 0.5) based on time remaining
  - [X] Updates on interval (every 30s by default)
  - [X] Start fading at 50% of time remaining (midpoint between updated_at and expires_at)
  - [X] Returns `isFading`, `timeRemainingMs`, and `timeRemainingText`

- [X] Update `JobCard.tsx`:
  - [X] Apply opacity style based on fade percentage (all statuses fade)
  - [X] Add subtle visual indicator showing time remaining with clock icon
  - [X] Amber color when fading, gray when not
  - [X] Tooltip shows exact expiration time
  - [X] Acked styling (opacity-60) multiplies with fade opacity

- [X] Update `JobStatusBadge.tsx`:
  - [X] Badge stays solid (no fade) - only the parent card fades
  - [X] Add subtle amber ring indicator when `isExpiring` is true

### Real-time Updates

- [X] Handle `job_expired` WebSocket event (done in Phase 3):
  - [X] Invalidates relevant queries to remove job from UI
  - [X] Health and group counters update automatically via React Query

---

## Phase 5: Frontend UI - Group Configuration

### GroupConfigForm Updates

- [X] Redesign group config dialog:
  - [X] **Expiration section:**
    - [X] Number input for expiration hours (required, default 24)
    - [X] Helper text: "Jobs auto-delete after this time without updates"
  - [X] **Staleness section:**
    - [X] Checkbox: "Enable staleness warnings"
    - [X] Conditional number input for staleness hours (only shown if enabled)
    - [X] Helper text: "Jobs show warning state before expiring"
    - [X] Validation: staleness hours must be less than expiration hours

- [X] Update form validation with Zod:
  - [X] Expiration hours: required, min 1, max 8760
  - [X] Staleness enabled: boolean
  - [X] Staleness hours: required if enabled, must be < expiration hours

- [X] Add Checkbox UI component

### Global Config Updates (Optional)

- [X] Consider whether global defaults should change:
  - [X] Global expiration default (24h) - kept as is
  - [X] Global staleness default (disabled) - kept as is
  - [X] Keep global config for progress timeout; expiration/staleness configured per-group

---

## Phase 6: Migration & Cleanup

### Data Migration

- [X] Write migration script for existing data:
  - [X] Migration sets `staleness_enabled = False` for all groups (opt-in by default)
  - [X] Migration adds `expiration_timeout_hours` column (nullable, uses global default)
  - [X] Migration computes `expires_at` for all existing jobs using `updated_at + 24 hours`
  - [X] (Note: staleness_enabled=True for existing groups was decided against; users opt-in explicitly)

### Cleanup

- [X] Remove or update stale-related admin cleanup:
  - [X] Expiration now handles auto-deletion via background processor
  - [X] Manual cleanup still available via admin endpoint for edge cases

### Documentation

- [X] Add AIDEV-NOTE comments explaining the staleness vs expiration model
  - [X] Comments added throughout backend and frontend code
- [X] Phase 6 complete - migration handled in Phase 1 alembic migration

---

## Phase 7: Testing

### Backend Tests

- [X] Test staleness disabled by default (no transition occurs)
- [X] Test staleness enabled triggers transition
- [X] Test expiration deletes jobs correctly (success status)
- [X] Test expiration deletes error/timeout/stale jobs too
- [X] Test expiration deletes acked jobs (ack doesn't prevent expiry)
- [X] Test `expires_at` is refreshed on status update
- [X] Test validation: staleness_hours < expiration_hours
- [X] Test validation skipped when staleness disabled

### Frontend Tests

- [X] Test fade percentage calculation (useFadePercentage.test.ts)
- [X] Test config form shows/hides staleness input (e2e/group-detail.spec.ts)
- [X] Test form validation (e2e/group-detail.spec.ts - staleness < expiration validation)
- [X] Test WebSocket job expiration handling (via SocketContext implementation)

### Integration Tests

- [X] End-to-end: create job → wait → verify expiration (covered in backend unit tests with time manipulation)
- [X] End-to-end: create job with staleness → verify warning → verify expiration (covered in backend unit tests)
- [X] Test config changes apply to existing jobs (covered in backend unit tests)

---

## Implementation Notes

### Fade Calculation

```
fade_start = expires_at - (expiration_timeout * 0.5)  # Start fading at 50% remaining
fade_end = expires_at

if now < fade_start:
    opacity = 1.0
elif now >= fade_end:
    opacity = 0.5  # Minimum visibility before deletion
else:
    progress = (now - fade_start) / (fade_end - fade_start)
    opacity = 1.0 - (progress * 0.5)  # 1.0 → 0.5
```

Note: Fade is fixed at 50% of expiration time, not configurable per-group.

### WebSocket Events

New event needed:
```json
{
  "event": "job_expired",
  "data": {
    "job_id": 123,
    "group_name": "backups"
  }
}
```

### API Response Shape

```json
{
  "id": 123,
  "name": "nightly-backup",
  "status": "success",
  "expires_at": "2024-01-26T12:00:00Z",
  "fade_percentage": 35,
  ...
}
```

---

## Design Decisions (Resolved)

1. **Fade timing**: Fixed at 50% of expiration time remaining, not configurable per-group
2. **Opacity range**: 100% → 50% (remains readable throughout)
3. **All statuses expire**: error, timeout, progress, stale, success - all expire when expiration is set
4. **Global config**: Keep for defaults, groups can override
