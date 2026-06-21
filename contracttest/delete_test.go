//go:build contract

// Ported from contract/test_delete.py — DELETE /api/jobs/<id> behavior. Delete returns 200
// with {deleted_job:{...}, group_id, group_name} (not a count) and removes the job; a second
// delete of the same id is a 404 not_found. Deleting also flows through to health and group
// aggregates. Job ids come straight from the POST response.
package contracttest

import (
	"fmt"
	"testing"
)

// deleteFindGroup returns the group object named name from GET /api/groups (fatal if absent).
func deleteFindGroup(t *testing.T, name string) map[string]any {
	t.Helper()
	_, body := getJSON(t, "/api/groups")
	groups := glist(t, body, "groups")
	for i := range groups {
		g := gelem(t, groups, i)
		if gstr(t, g, "name") == name {
			return g
		}
	}
	t.Fatalf("group %q not found", name)
	return nil
}

func TestDeleteJobSuccess(t *testing.T) {
	begin(t, "default")
	// Create a job; the POST response carries its id.
	created, createdBody := postJSON(t, "/api/status",
		map[string]any{"group": "test", "job": "job1", "status": "success"})
	mustStatus(t, created, 201)
	jobID := gint(t, gmap(t, createdBody, "job"), "id")

	_, jobs := getJSON(t, "/api/jobs")
	mustEqNum(t, jobs, "total", 1)

	// Delete the job.
	status, body := deleteJSON(t, fmt.Sprintf("/api/jobs/%d", jobID))
	mustStatus(t, status, 200)
	delJob := gmap(t, body, "deleted_job")
	mustEqNum(t, delJob, "id", float64(jobID))
	mustEqStr(t, delJob, "name", "job1")
	mustEqStr(t, body, "group_name", "test")

	// Verify job is gone.
	_, jobs = getJSON(t, "/api/jobs")
	mustEqNum(t, jobs, "total", 0)
}

func TestDeleteJobNotFound(t *testing.T) {
	begin(t, "default")
	// Create and delete a job to get a known non-existent id.
	_, createdBody := postJSON(t, "/api/status",
		map[string]any{"group": "test", "job": "temp", "status": "success"})
	deletedID := gint(t, gmap(t, createdBody, "job"), "id")
	deleteJSON(t, fmt.Sprintf("/api/jobs/%d", deletedID))

	// Now try to delete the already-deleted job.
	status, body := deleteJSON(t, fmt.Sprintf("/api/jobs/%d", deletedID))
	mustStatus(t, status, 404)
	mustEqStr(t, body, "error", "not_found")
}

func TestDeleteJobTwiceReturns404(t *testing.T) {
	begin(t, "default")
	_, createdBody := postJSON(t, "/api/status",
		map[string]any{"group": "test", "job": "job1", "status": "success"})
	jobID := gint(t, gmap(t, createdBody, "job"), "id")

	// First delete succeeds.
	status, _ := deleteJSON(t, fmt.Sprintf("/api/jobs/%d", jobID))
	mustStatus(t, status, 200)

	// Second delete returns 404.
	status, _ = deleteJSON(t, fmt.Sprintf("/api/jobs/%d", jobID))
	mustStatus(t, status, 404)
}

func TestDeleteJobUpdatesHealth(t *testing.T) {
	begin(t, "default")
	// Create an error job.
	_, createdBody := postJSON(t, "/api/status",
		map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := gint(t, gmap(t, createdBody, "job"), "id")

	// Verify unhealthy count.
	_, health := getJSON(t, "/api/health")
	mustEqNum(t, health, "unhealthy", 1)

	deleteJSON(t, fmt.Sprintf("/api/jobs/%d", jobID))

	// Verify health is now empty.
	_, health = getJSON(t, "/api/health")
	mustEqStr(t, health, "status", "empty")
	mustEqNum(t, health, "total_jobs", 0)
}

func TestDeleteJobUpdatesGroupCounts(t *testing.T) {
	begin(t, "default")
	// Create two jobs in the same group.
	_, createdBody := postJSON(t, "/api/status",
		map[string]any{"group": "test", "job": "job1", "status": "success"})
	jobID := gint(t, gmap(t, createdBody, "job"), "id")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "success"})

	// Verify group has 2 jobs (find by name to avoid ordering assumptions).
	group := deleteFindGroup(t, "test")
	mustEqNum(t, group, "job_count", 2)

	// Delete one job.
	deleteJSON(t, fmt.Sprintf("/api/jobs/%d", jobID))

	// Verify group now has 1 job.
	group = deleteFindGroup(t, "test")
	mustEqNum(t, group, "job_count", 1)
}
