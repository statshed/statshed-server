# Error Acknowledgement Feature - Implementation Plan

## Overview

Add the ability to acknowledge (ack) errors and timeouts in statshed, allowing the dashboard to show "green" status when all errors have been acknowledged by an operator. Acked jobs display with strikethrough styling showing their original status.

## Requirements Summary

- **Persistence**: Acks stored in database, survive restarts
- **Visual**: Acked jobs show original status (error/timeout/stale) with strikethrough
- **Counts**: Acked jobs excluded from error counts; dashboard is "healthy" when all errors acked
- **Clear behavior**:
  - `acked → success/progress` = ack cleared (job recovered)
  - `acked → error/timeout` (same job, new submission) = stays acked
  - `error → success → error` = requires new ack
- **Scope**: Ack individual jobs, all in a group, or all globally
- **No undo needed** (can re-ack if job errors again after recovering)

---

## Phase 1: Backend Data Model & API

### 1.1 Database Schema Change

Add `acked` boolean column to the `jobs` table:

```python
# models.py - Add to Job model
acked: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False, index=True)
acked_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
```

**Migration**: Create Alembic migration to add columns with default `False` for `acked` and `NULL` for `acked_at`.

**Index considerations**:
- Add index on `acked` column for efficient filtering
- Add composite index on `(status, acked)` for health queries

### 1.2 Status Transition Logic

Modify `POST /status` endpoint to handle ack clearing:

```python
# When updating a job's status:
if new_status in ('success', 'progress'):
    # Job recovered - clear the ack
    job.acked = False
    job.acked_at = None
# For error/timeout/stale: preserve existing acked state
# (acked jobs stay acked even with new error messages)
```

**Note on "new submission" semantics**: Jobs are identified by `(group_id, name)` composite key. There is no run/submission identifier. A "new submission" simply means calling `POST /status` again for the same job. Since acked jobs stay acked on subsequent error submissions, no run ID is needed.

### 1.3 New API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/jobs/<id>/ack` | POST | Ack a single job |
| `/groups/<name>/ack` | POST | Ack all errored jobs in a group |
| `/ack-all` | POST | Ack all errored jobs globally |

**Endpoint Implementation Requirements**:
- All ack endpoints MUST set `acked_at = datetime.now(UTC)` when setting `acked = True`
- Acking an already-acked job is a no-op (returns success, does not update `acked_at`)
- Only jobs with status `error`, `timeout`, or `stale` can be acked

**Request/Response**:
```json
// POST /jobs/123/ack
// Response 200:
{
  "job": {
    "id": 123,
    "name": "backup-job",
    "status": "error",
    "acked": true,
    "acked_at": "2025-01-21T10:30:00Z",
    ...
  }
}

// POST /groups/nightly-builds/ack
// Response 200:
{
  "acked_count": 5,
  "group": "nightly-builds"
}

// POST /ack-all
// Response 200:
{
  "acked_count": 12
}
```

### 1.4 Modify Health Calculation

Update `/health` endpoint to exclude acked jobs from unhealthy count:

```python
# Current: unhealthy = error + timeout + stale
# New: unhealthy = (error + timeout + stale) WHERE acked = False

# Add new field to response:
{
  "status": "healthy",  # Now "healthy" if all errors are acked
  "unhealthy": 0,       # Excludes acked jobs
  "acked": 5,           # NEW: count of acked jobs
  "total_jobs": 25,
  "healthy": 20,
  "in_progress": 0,
  "by_status": {
    "success": 20,
    "error": 3,         # Raw count (includes acked)
    "progress": 0,
    "timeout": 2,       # Raw count (includes acked)
    "stale": 0
  }
}
```

### 1.5 Modify Group Summary Calculations

**CRITICAL**: Group-level counts MUST also exclude acked jobs to maintain consistency with `/health`.

Update `/groups` endpoint response to include ack-aware counts:

```python
# For each group, calculate:
{
  "id": 1,
  "name": "nightly-builds",
  "job_count": 10,
  "health": "healthy",          # Treats acked jobs as healthy
  "unhealthy_count": 0,         # NEW: excludes acked jobs
  "acked_count": 3,             # NEW: count of acked jobs in this group
  "status_counts": {
    "success": 5,
    "error": 2,                 # Raw count (includes acked)
    "progress": 1,
    "timeout": 1,               # Raw count (includes acked)
    "stale": 1
  }
}
```

**Group health calculation logic** (update `calculate_health()` function):
```python
def calculate_health(jobs: list[Job]) -> str:
    if not jobs:
        return "empty"

    # Only count unacked unhealthy jobs
    has_unhealthy = any(
        job.status in UNHEALTHY_STATUSES and not job.acked
        for job in jobs
    )
    if has_unhealthy:
        return "unhealthy"

    has_in_progress = any(job.status == "progress" for job in jobs)
    if has_in_progress:
        return "in_progress"

    return "healthy"
```

### 1.6 Modify Job Queries

Update `/jobs` and `/groups/<name>/jobs` to include `acked` and `acked_at` fields in responses.

Update `Job.to_dict()`:
```python
def to_dict(self) -> dict:
    return {
        "id": self.id,
        "group_id": self.group_id,
        "group_name": self.group.name,
        "name": self.name,
        "status": self.status,
        "message": self.message,
        "acked": self.acked,                                          # NEW
        "acked_at": self.acked_at.strftime("%Y-%m-%dT%H:%M:%SZ") if self.acked_at else None,  # NEW
        "updated_at": self.updated_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "created_at": self.created_at.strftime("%Y-%m-%dT%H:%M:%SZ"),
    }
```

Add optional query parameter: `?include_acked=true|false` (default: true)

### 1.7 WebSocket Events

Emit events when jobs are acked:

```python
# New event for ack operations
socketio.emit('jobs_acked', {
    'job_ids': [123, 456],
    'group_id': 1,              # or None for global ack
    'group_name': 'nightly-builds',  # or None for global ack
    'acked_count': 2,
    'timestamp': datetime.utcnow().isoformat() + 'Z'
})
```

**Frontend handling**: The frontend will invalidate React Query caches when receiving `jobs_acked` events, following the same pattern as existing `status_update` and `health_update` events. Specifically:
- Invalidate `['health']` query
- Invalidate `['groups']` query
- Invalidate `['jobs']` query (and any filtered variants)
- Invalidate `['group', groupName, 'jobs']` if `group_name` is present

This cache invalidation approach is consistent with the existing WebSocket handling in `SocketContext.tsx`.

### 1.8 Background Timeout Checker

When the timeout checker changes a job to `timeout` or `stale`:
- If job was previously acked AND transitioning from another unhealthy state (error→timeout, timeout→stale), preserve ack
- If job was `success`/`progress` → `timeout`/`stale`, it's a new error (acked=False)

```python
# In background.py timeout check logic:
if job.status in ('success', 'progress'):
    # Job was healthy, now timing out - this is a new error
    job.acked = False
    job.acked_at = None
    job.status = 'timeout'  # or 'stale'
elif job.status in UNHEALTHY_STATUSES:
    # Already unhealthy, preserve ack state
    job.status = 'timeout'  # or 'stale'
    # Don't touch acked/acked_at
```

---

## Phase 2: Frontend UI Components

### 2.1 JobCard Ack Button

Add ack button to JobCard for error/timeout/stale jobs:

```tsx
// JobCard.tsx additions
{isErrorState && !job.acked && (
  <button
    onClick={handleAck}
    className="text-xs text-gray-500 hover:text-green-600"
    title="Acknowledge this error"
  >
    ✓ Ack
  </button>
)}
```

### 2.2 Acked Job Styling

Show acked jobs with strikethrough and muted colors:

```tsx
// JobCard.tsx - conditional styling
const cardClasses = cn(
  "job-card",
  job.acked && "opacity-60"
);

const statusClasses = cn(
  "status-badge",
  job.acked && "line-through"
);

// Show original status with strikethrough
<span className={statusClasses}>
  {job.status} {job.acked && "(acked)"}
</span>
```

Visual treatment:
- Card slightly faded (opacity-60)
- Status badge with strikethrough: ~~error~~ or ~~timeout~~
- Optional "(acked)" label
- Different border color (gray instead of red)

### 2.3 Group-Level Ack Button

Add "Ack All" button to GroupCard header when group has unacked errors:

```tsx
// GroupCard.tsx
// Use the NEW unhealthy_count field (excludes acked)
{group.unhealthy_count > 0 && (
  <button
    onClick={handleAckAll}
    className="text-xs text-gray-500 hover:text-green-600"
  >
    ✓ Ack All ({group.unhealthy_count})
  </button>
)}

// Optionally show acked count as a badge/tooltip
{group.acked_count > 0 && (
  <span className="text-xs text-gray-400">
    {group.acked_count} acked
  </span>
)}
```

### 2.4 Global Ack Button

Add "Ack All Errors" button to HealthStats or Dashboard header:

```tsx
// HealthStats.tsx or Dashboard.tsx
{health.unhealthy > 0 && (
  <button onClick={handleAckAll}>
    ✓ Ack All Errors ({health.unhealthy})
  </button>
)}
```

### 2.5 HealthStats Updates

Update the error count card to reflect acked status:

```tsx
// Show both unacked errors and acked count
<StatCard
  label="Errors"
  value={health.unhealthy}
  subtitle={health.acked > 0 ? `${health.acked} acked` : undefined}
/>
```

### 2.6 React Query Mutations

Create mutation hooks for ack operations:

```typescript
// hooks/mutations.ts
export function useAckJob() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (jobId: number) => api.ackJob(jobId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['health'] });
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    }
  });
}

export function useAckGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (groupName: string) => api.ackGroup(groupName),
    onSuccess: (_, groupName) => {
      queryClient.invalidateQueries({ queryKey: ['health'] });
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      queryClient.invalidateQueries({ queryKey: ['group', groupName, 'jobs'] });
    }
  });
}

export function useAckAll() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.ackAll(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['health'] });
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    }
  });
}
```

### 2.7 WebSocket Event Handler

Add handler for `jobs_acked` event in `SocketContext.tsx`:

```typescript
socket.on('jobs_acked', (data: {
  job_ids: number[];
  group_id: number | null;
  group_name: string | null;
  acked_count: number;
  timestamp: string;
}) => {
  queryClient.invalidateQueries({ queryKey: ['health'] });
  queryClient.invalidateQueries({ queryKey: ['groups'] });
  queryClient.invalidateQueries({ queryKey: ['jobs'] });
  if (data.group_name) {
    queryClient.invalidateQueries({ queryKey: ['group', data.group_name, 'jobs'] });
  }
});
```

### 2.8 Favicon Updates

Modify favicon logic: green when all errors acked (not just when no errors exist).

Update the logic in `useDynamicFavicon` or equivalent:
```typescript
// Current: red if health.unhealthy > 0
// New: red if health.unhealthy > 0 (unhealthy already excludes acked)
// No change needed if unhealthy count is correctly calculated
```

---

## Phase 3: Polish & Edge Cases

### 3.1 Optimistic Updates

Implement optimistic UI updates for snappy feel:
- Immediately show job as acked on button click
- Rollback if API call fails

### 3.2 Confirmation for Bulk Ack

Optional: Add confirmation dialog for "Ack All" at group/global level:
- "Ack 12 errors in nightly-builds?"
- Skip for single job acks

### 3.3 Keyboard Shortcuts

Optional enhancement:
- `a` key to ack selected/focused job
- `Shift+A` to ack all in current view

### 3.4 Toast Notifications

Show brief toast on successful ack:
- "Acknowledged 1 error"
- "Acknowledged 5 errors in nightly-builds"

---

## Implementation Order

### Iteration 1: Core Backend (MVP)
- [X] Add `acked` and `acked_at` columns via migration
- [X] Update `Job.to_dict()` to include new fields
- [X] Implement `POST /jobs/<id>/ack` endpoint (sets `acked=True`, `acked_at=now()`)
- [X] Update health calculation to exclude acked jobs
- [X] Update group summary to include `unhealthy_count` and `acked_count`
- [X] Modify status submission to clear ack on recovery (success/progress)
- [X] Update background timeout checker to preserve ack state
- [X] Write backend tests for ack feature

### Iteration 2: Frontend Single-Job Ack
- [X] Update TypeScript types for Job to include `acked` and `acked_at`
- [X] Update TypeScript types for HealthSummary to include `acked`
- [X] Update TypeScript types for GroupWithHealth to include `unhealthy_count` and `acked_count`
- [X] Add API client functions for ack operations
- [X] Add ack button to JobCard
- [X] Create `useAckJob` mutation hook
- [X] Add acked styling (strikethrough, muted)
- [X] Update HealthStats to show acked count

### Iteration 3: Bulk Ack Operations
- [X] Implement `POST /groups/<name>/ack` endpoint
- [X] Implement `POST /ack-all` endpoint
- [X] Add group-level ack button to GroupCard
- [X] Add global ack button to HealthStats
- [X] Add WebSocket `jobs_acked` event emission
- [X] Add WebSocket event handler in frontend
- [X] Update favicon logic (if needed) - No changes needed, uses unhealthy count which excludes acked

### Iteration 4: Polish
- [X] Optimistic updates
- [X] Toast notifications
- [X] Any UX refinements based on testing

---

## Data Model Summary

```
Job table:
  - id: int (PK)
  - group_id: int (FK)
  - name: string
  - status: enum (success, error, progress, timeout, stale)
  - message: string
  - acked: boolean (NEW, default=False, indexed)
  - acked_at: datetime (NEW, nullable)
  - updated_at: datetime
  - created_at: datetime

New indexes:
  - ix_jobs_acked (acked)
  - ix_jobs_status_acked (status, acked) - for health queries
```

---

## API Summary

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/jobs/<id>/ack` | POST | Ack single job |
| `/groups/<name>/ack` | POST | Ack all errors in group |
| `/ack-all` | POST | Ack all errors globally |
| `/health` | GET | Now includes `acked` count, `unhealthy` excludes acked |
| `/groups` | GET | Now includes `unhealthy_count` and `acked_count` per group |
| `/jobs` | GET | Jobs now include `acked` and `acked_at` fields |

---

## TypeScript Types

```typescript
// types/job.ts - updated
interface Job {
  id: number;
  group_id: number;
  group_name: string;
  name: string;
  status: 'success' | 'error' | 'progress' | 'timeout' | 'stale';
  message: string | null;
  acked: boolean;           // NEW
  acked_at: string | null;  // NEW - ISO 8601 datetime
  updated_at: string;
  created_at: string;
}

// types/health.ts - updated
interface HealthResponse {
  status: 'healthy' | 'unhealthy' | 'in_progress' | 'empty';
  total_jobs: number;
  healthy: number;
  unhealthy: number;  // Excludes acked
  acked: number;      // NEW
  in_progress: number;
  by_status: {
    success: number;
    error: number;    // Raw count
    progress: number;
    timeout: number;  // Raw count
    stale: number;
  };
}

// types/group.ts - updated
interface GroupSummary {
  id: number;
  name: string;
  job_count: number;
  health: 'healthy' | 'unhealthy' | 'in_progress' | 'empty';
  unhealthy_count: number;  // NEW - excludes acked
  acked_count: number;      // NEW
  status_counts: {
    success: number;
    error: number;
    progress: number;
    timeout: number;
    stale: number;
  };
  progress_timeout_minutes: number | null;
  staleness_timeout_hours: number | null;
  created_at: string;
}
```

---

## Success Criteria

1. Dashboard shows "healthy" when all errors are acked
2. Acked jobs visible with strikethrough styling
3. Acks persist across backend restarts
4. Acks auto-clear when job recovers (success/progress)
5. New errors for already-acked jobs stay acked
6. Bulk ack at group and global level works
7. Real-time updates via WebSocket
8. Group cards show "healthy" when all their errors are acked
9. `acked_at` timestamp is set correctly when acking
