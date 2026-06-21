//go:build contract

// Ported from contract/test_admin.py — GET /api/admin/stats and DELETE /api/admin/cleanup
// (data retention). The three cleanup tests that need aged rows POST jobs normally, then
// backdate updated_at ~60 days into the past via direct SQL (the server only writes 'now').
package contracttest

import (
	"testing"
	"time"
)

// adminOldTS is ~60 days in the past, in the app's stored text format (matches the helpers).
func adminOldTS() string {
	return time.Now().UTC().AddDate(0, 0, -60).Format("2006-01-02 15:04:05.000000")
}

// adminJobNames returns the set of job names currently visible via GET /api/jobs.
func adminJobNames(t *testing.T) map[string]bool {
	t.Helper()
	_, body := getJSON(t, "/api/jobs")
	jobs := glist(t, body, "jobs")
	names := make(map[string]bool, len(jobs))
	for i := range jobs {
		names[gstr(t, gelem(t, jobs, i), "name")] = true
	}
	return names
}

func TestAdminStatsEmpty(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/admin/stats")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total_jobs", 0)
	mustEqNum(t, body, "total_groups", 0)
	byStatus := gmap(t, body, "jobs_by_status")
	mustEqNum(t, byStatus, "success", 0)
	mustEqNum(t, byStatus, "error", 0)
}

func TestAdminStatsWithData(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g2", "job": "job1", "status": "progress"})

	status, body := getJSON(t, "/api/admin/stats")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total_jobs", 3)
	mustEqNum(t, body, "total_groups", 2)
	byStatus := gmap(t, body, "jobs_by_status")
	mustEqNum(t, byStatus, "success", 1)
	mustEqNum(t, byStatus, "error", 1)
	mustEqNum(t, byStatus, "progress", 1)
}

func TestAdminCleanupRequiresOlderThanDays(t *testing.T) {
	begin(t, "default")
	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"statuses": []string{"stale"}})
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "older_than_days")
}

func TestAdminCleanupInvalidOlderThanDays(t *testing.T) {
	begin(t, "default")
	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 0})
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "positive integer")
}

func TestAdminCleanupInvalidStatus(t *testing.T) {
	begin(t, "default")
	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 30, "statuses": []string{"invalid"}})
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "Invalid status")
}

func TestAdminCleanupDryRun(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test-group", "job": "old", "status": "stale"})
	backdate(t, "jobs", "name='old'", map[string]any{"updated_at": adminOldTS()})

	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 30, "statuses": []string{"stale"}, "dry_run": true})
	mustStatus(t, status, 200)
	mustEqNum(t, body, "deleted_jobs", 1)
	mustEqNum(t, body, "deleted_groups", 1)
	mustEqBool(t, body, "dry_run", true)

	// Dry run must NOT delete: the job still exists.
	if !adminJobNames(t)["old"] {
		t.Errorf("dry_run deleted the job; expected 'old' to remain")
	}
}

func TestAdminCleanupDeletesJobs(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test-group", "job": "old", "status": "stale"})
	backdate(t, "jobs", "name='old'", map[string]any{"updated_at": adminOldTS()})

	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 30, "statuses": []string{"stale"}, "dry_run": false})
	mustStatus(t, status, 200)
	mustEqNum(t, body, "deleted_jobs", 1)
	mustEqNum(t, body, "deleted_groups", 1)
	mustEqBool(t, body, "dry_run", false)

	// The job is gone.
	if adminJobNames(t)["old"] {
		t.Errorf("expected 'old' to be deleted")
	}
}

func TestAdminCleanupPreservesRecentJobs(t *testing.T) {
	begin(t, "default")
	// Create a recent job (not backdated).
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "stale"})

	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 30, "statuses": []string{"stale"}})
	mustStatus(t, status, 200)
	mustEqNum(t, body, "deleted_jobs", 0)
	mustEqNum(t, body, "deleted_groups", 0)
}

func TestAdminCleanupRespectsStatusFilter(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test-group", "job": "stale-job", "status": "stale"})
	postJSON(t, "/api/status", map[string]any{"group": "test-group", "job": "error-job", "status": "error"})
	backdate(t, "jobs", "name='stale-job'", map[string]any{"updated_at": adminOldTS()})
	backdate(t, "jobs", "name='error-job'", map[string]any{"updated_at": adminOldTS()})

	// Cleanup with default statuses (stale, timeout).
	status, body := deleteJSONBody(t, "/api/admin/cleanup",
		map[string]any{"older_than_days": 30})
	mustStatus(t, status, 200)
	mustEqNum(t, body, "deleted_jobs", 1) // Only the stale job.

	// The error job still exists.
	if !adminJobNames(t)["error-job"] {
		t.Errorf("expected 'error-job' to survive the status filter")
	}
}

func TestAdminCleanupRequiresJSON(t *testing.T) {
	begin(t, "default")
	status, body := deleteJSONBody(t, "/api/admin/cleanup", map[string]any{})
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "older_than_days")
}
