# Health Cards Click-Through Feature Plan

## Overview

Make the "Healthy", "Errors", and "In Progress" cards on the Dashboard clickable, navigating to a new Jobs page that displays the matching jobs.

**Design Decisions:**
- **Display method:** Navigate to a new page (`/jobs?status=...`)
- **Error card behavior:** Shows both `error` AND `timeout` jobs (matches card count)
- **Total Jobs card:** Remains display-only (not clickable)
- **Job details shown:** Job name, group name, status badge, last updated, message

---

## Phase 1: Backend - Add Jobs Listing Endpoint [X]

### 1.1 Create `/jobs` endpoint in `app.py` [X]

**File:** `../backend/app.py`

Add a new endpoint that returns jobs filtered by status(es):

```
GET /jobs?status=success         → Jobs with status "success"
GET /jobs?status=error,timeout   → Jobs with status "error" OR "timeout"
GET /jobs?status=progress        → Jobs with status "progress"
GET /jobs                        → All jobs (optional, for future use)
```

**Response format:**
```json
{
  "jobs": [
    {
      "id": 1,
      "name": "backend-tests",
      "group_id": 1,
      "group_name": "nightly-builds",
      "status": "error",
      "message": "Test failed: assertion error in test_auth.py",
      "updated_at": "2025-01-17T12:00:00Z",
      "created_at": "2025-01-17T10:00:00Z"
    }
  ],
  "total": 15
}
```

**Implementation notes:**
- Accept `status` as comma-separated query parameter
- Validate status values against `VALID_STATUSES`
- Use existing `Job.to_dict()` method for serialization
- Join with Group to include `group_name`
- Order by `updated_at DESC` (most recently updated first)
- Consider pagination if job count could be large (optional enhancement)

### 1.2 Add index if needed [X]

The existing `ix_jobs_status` index should be sufficient for status filtering.

---

## Phase 2: Frontend - Jobs Page [X]

### 2.1 Add API function [X]

**File:** `src/api/jobs.ts`

```typescript
export async function fetchJobsByStatus(statuses: string[]): Promise<JobsResponse> {
  const params = statuses.length ? `?status=${statuses.join(',')}` : '';
  const response = await fetch(`${API_BASE_URL}/jobs${params}`);
  // ... error handling
  return response.json();
}
```

### 2.2 Add React Query hook [X]

**File:** `src/hooks/useJobs.ts`

```typescript
export function useJobsByStatus(statuses: string[]) {
  return useQuery({
    queryKey: queryKeys.jobsByStatus(statuses),
    queryFn: () => fetchJobsByStatus(statuses),
    // ... options
  });
}
```

### 2.3 Add query key [X]

**File:** `src/lib/constants.ts`

Add: `jobsByStatus: (statuses: string[]) => ['jobs', 'byStatus', statuses]`

### 2.4 Create Jobs page component [X]

**File:** `src/pages/Jobs.tsx`

Page structure:
- Header with title indicating filter (e.g., "Error Jobs", "Healthy Jobs", "Jobs In Progress")
- Back link to Dashboard
- Job count summary
- Job list/table with:
  - Job name (link to group detail page)
  - Group name (link to group detail page)
  - Status badge (colored)
  - Message (if present, potentially truncated with expand)
  - Last updated (relative time like "2 hours ago")
- Loading skeleton while fetching
- Empty state if no matching jobs

### 2.5 Create JobList component [X]

**File:** `src/components/jobs/JobList.tsx`

Updated existing component to support `showGroup` prop for displaying jobs from multiple groups.

### 2.6 Create JobListItem component [X]

**File:** `src/components/jobs/JobCard.tsx`

Updated existing JobCard component with `showGroup` prop to optionally display group name with link.

### 2.7 Add route [X]

**File:** `src/App.tsx`

Add route: `<Route path="/jobs" element={<Jobs />} />`

### 2.8 Update types if needed [X]

**File:** `src/types/index.ts`

Added `JobsResponse` interface for the /jobs endpoint response.

---

## Phase 3: Frontend - Make Cards Clickable [X]

### 3.1 Update HealthStats component [X]

**File:** `src/components/health/HealthStats.tsx`

Changes:
1. [X] Import `Link` from `react-router-dom`
2. [X] Wrap the Healthy, Errors, and In Progress cards with `Link` components
3. [X] Add appropriate URLs:
   - Healthy → `/jobs?status=success`
   - Errors → `/jobs?status=error,timeout`
   - In Progress → `/jobs?status=progress`
4. [X] Add hover/focus styles to indicate clickability (cursor pointer, subtle background change)
5. [X] Keep "Total Jobs" card as non-clickable (no link wrapper)
6. [X] Ensure accessibility: cards should be focusable and keyboard navigable

### 3.2 Styling considerations [X]

- [X] Add visual affordance that cards are clickable (hover state)
- [X] Ensure color contrast remains accessible
- [X] Mobile: touch targets are adequate (cards are full-width clickable)

---

## Phase 4: Testing & Polish [X]

### 4.1 Backend tests [X]

**File:** `../backend/tests/test_api.py`

Added tests for new `/jobs` endpoint:
- [X] Test filtering by single status
- [X] Test filtering by multiple statuses (comma-separated)
- [X] Test invalid status returns 400
- [X] Test empty result set
- [X] Test response structure
- [X] Test ordering by updated_at DESC
- [X] Test whitespace handling in status parameter

### 4.2 Frontend considerations [X]

- [X] Test navigation from each card (via useJobs hook tests)
- [X] Test back navigation (via Jobs page back link)
- [X] Test empty states (emptyMessage prop)
- [X] Test loading states (JobList loading skeleton)

### 4.3 WebSocket integration

Decided: Keep it simple initially (static snapshot with manual refresh via React Query's 60s refetchInterval), add real-time updates later if needed.

---

## File Summary

### Backend changes:
| File | Change |
|------|--------|
| `../backend/app.py` | [X] Add `/jobs` endpoint |
| `../backend/tests/test_api.py` | [X] Add tests for new endpoint (11 tests) |

### Frontend new files:
| File | Purpose |
|------|---------|
| `src/hooks/useJobs.ts` | [X] React Query hook |
| `src/hooks/useJobs.test.tsx` | [X] Tests for useJobs hook (4 tests) |
| `src/pages/Jobs.tsx` | [X] Jobs listing page |

### Frontend modified files:
| File | Change |
|------|--------|
| `src/api/jobs.ts` | [X] Add `getJobsByStatus` API function |
| `src/api/index.ts` | [X] Export new function |
| `src/api/types.ts` | [X] Export `JobsResponse` type |
| `src/App.tsx` | [X] Add `/jobs` route |
| `src/lib/constants.ts` | [X] Add query key for jobs by status |
| `src/types/index.ts` | [X] Add `JobsResponse` interface |
| `src/components/health/HealthStats.tsx` | [X] Make cards clickable with links |
| `src/components/jobs/JobList.tsx` | [X] Add `showGroup` and `emptyMessage` props |
| `src/components/jobs/JobCard.tsx` | [X] Add `showGroup` prop with group link |
| `src/hooks/index.ts` | [X] Export `useJobsByStatus` hook |
| `src/test/mocks/handlers.ts` | [X] Add mock handler for /jobs endpoint |

---

## Optional Enhancements (Future)

1. **Pagination** - If job counts get large, add pagination to `/jobs` endpoint
2. **Search** - Add job name search on the Jobs page
3. **Sorting** - Allow sorting by different columns
4. **Real-time updates** - Subscribe to WebSocket events on Jobs page
5. **URL state** - Sync search/sort state to URL for shareability
6. **Export** - Allow exporting job list to CSV
