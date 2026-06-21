//go:build contract

// Ported from contract/test_ack.py — job acknowledgement behavior.
//
// Only error/timeout/stale jobs are ack-eligible (success/progress -> 400 invalid_state);
// ack is idempotent (acked_at unchanged on re-ack); a recovery to success/progress clears
// acked. Health/group `unhealthy` counts EXCLUDE acked jobs, while by_status/status_counts
// count everything raw. Single ack returns `{"job": ...}`; group-ack and ack-all return
// `{"acked_count": N, ...}`.
package contracttest

import (
	"fmt"
	"testing"
)

// ackFirstJobID returns the id of the first job from GET /api/jobs.
func ackFirstJobID(t *testing.T) int {
	t.Helper()
	_, body := getJSON(t, "/api/jobs")
	return gint(t, gelem(t, glist(t, body, "jobs"), 0), "id")
}

// --- TestAckEndpoint ---

func TestAckJobSuccess(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 200)
	job := gmap(t, body, "job")
	mustEqBool(t, job, "acked", true)
	if isNull(job, "acked_at") {
		t.Error("acked_at should not be null")
	}
	mustEqStr(t, job, "status", "error")
}

func TestAckJobTimeoutStatus(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "timeout"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 200)
	mustEqBool(t, gmap(t, body, "job"), "acked", true)
}

func TestAckJobStaleStatus(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "stale"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 200)
	mustEqBool(t, gmap(t, body, "job"), "acked", true)
}

func TestAckJobNotFound(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/jobs/99999/ack", nil)
	mustStatus(t, status, 404)
	mustEqStr(t, body, "error", "not_found")
}

func TestAckJobInvalidStateSuccess(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 400)
	mustEqStr(t, body, "error", "invalid_state")
	mustContains(t, gstr(t, body, "message"), "Cannot ack job with status 'success'")
}

func TestAckJobInvalidStateProgress(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "progress"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "Cannot ack job with status 'progress'")
}

func TestAckJobIdempotent(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := ackFirstJobID(t)

	status, body := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 200)
	firstAckedAt := gstr(t, gmap(t, body, "job"), "acked_at")

	status, body = postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)
	mustStatus(t, status, 200)
	secondAckedAt := gstr(t, gmap(t, body, "job"), "acked_at")

	if firstAckedAt != secondAckedAt {
		t.Errorf("acked_at changed on re-ack: %q -> %q", firstAckedAt, secondAckedAt)
	}
}

// --- TestAckHealthCalculation ---

func TestHealthExcludesAckedFromUnhealthy(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")
	mustEqNum(t, body, "unhealthy", 1)
	mustEqNum(t, body, "acked", 0)

	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	_, body = getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "healthy")
	mustEqNum(t, body, "unhealthy", 0)
	mustEqNum(t, body, "acked", 1)
}

func TestHealthStatusHealthyWhenAllErrorsAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "timeout"})

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")

	_, jb := getJSON(t, "/api/jobs")
	for _, j := range glist(t, jb, "jobs") {
		id := gint(t, j.(map[string]any), "id")
		postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", id), nil)
	}

	_, body = getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "healthy")
	mustEqNum(t, body, "unhealthy", 0)
	mustEqNum(t, body, "acked", 2)
}

func TestHealthMixedAckedAndUnacked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})

	_, jb := getJSON(t, "/api/jobs")
	firstID := gint(t, gelem(t, glist(t, jb, "jobs"), 0), "id")
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", firstID), nil)

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")
	mustEqNum(t, body, "unhealthy", 1)
	mustEqNum(t, body, "acked", 1)
}

func TestHealthByStatusIncludesAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "error"})

	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	_, body := getJSON(t, "/api/health")
	mustEqNum(t, gmap(t, body, "by_status"), "error", 1)
}

// --- TestAckGroupSummary ---

func TestGroupUnhealthyCountExcludesAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "success"})

	_, body := getJSON(t, "/api/groups")
	group := gelem(t, glist(t, body, "groups"), 0)
	mustEqNum(t, group, "unhealthy_count", 1)
	mustEqNum(t, group, "acked_count", 0)
	mustEqStr(t, group, "health", "unhealthy")

	_, jb := getJSON(t, "/api/jobs?status=error")
	errID := gint(t, gelem(t, glist(t, jb, "jobs"), 0), "id")
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", errID), nil)

	_, body = getJSON(t, "/api/groups")
	group = gelem(t, glist(t, body, "groups"), 0)
	mustEqNum(t, group, "unhealthy_count", 0)
	mustEqNum(t, group, "acked_count", 1)
	mustEqStr(t, group, "health", "healthy")
}

func TestGroupStatusCountsIncludeAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	_, body := getJSON(t, "/api/groups")
	group := gelem(t, glist(t, body, "groups"), 0)
	mustEqNum(t, gmap(t, group, "status_counts"), "error", 1)
}

// --- TestAckClearOnRecovery ---

func TestAckClearedOnSuccess(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	_, jb := getJSON(t, "/api/jobs")
	mustEqBool(t, gelem(t, glist(t, jb, "jobs"), 0), "acked", true)

	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})

	_, jb = getJSON(t, "/api/jobs")
	job := gelem(t, glist(t, jb, "jobs"), 0)
	mustEqBool(t, job, "acked", false)
	mustNull(t, job, "acked_at")
}

func TestAckClearedOnProgress(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "progress"})

	_, jb := getJSON(t, "/api/jobs")
	mustEqBool(t, gelem(t, glist(t, jb, "jobs"), 0), "acked", false)
}

func TestAckPreservedOnNewError(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error", "message": "fail1"})
	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error", "message": "fail2"})

	_, jb := getJSON(t, "/api/jobs")
	job := gelem(t, glist(t, jb, "jobs"), 0)
	mustEqBool(t, job, "acked", true)
	mustEqStr(t, job, "message", "fail2")
}

func TestErrorAfterRecoveryRequiresNewAck(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	jobID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", jobID), nil)

	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, jb := getJSON(t, "/api/jobs")
	mustEqBool(t, gelem(t, glist(t, jb, "jobs"), 0), "acked", false)

	_, body := getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "unhealthy")
}

// --- TestJobsResponseIncludesAckedFields ---

func TestJobsListIncludesAckedFields(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, body := getJSON(t, "/api/jobs")
	job := gelem(t, glist(t, body, "jobs"), 0)
	mustHave(t, job, "acked")
	mustHave(t, job, "acked_at")
	mustEqBool(t, job, "acked", false)
	mustNull(t, job, "acked_at")
}

func TestGroupJobsIncludesAckedFields(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, body := getJSON(t, "/api/groups/test/jobs")
	job := gelem(t, glist(t, body, "jobs"), 0)
	mustHave(t, job, "acked")
	mustHave(t, job, "acked_at")
}

func TestStatusResponseIncludesAckedFields(t *testing.T) {
	begin(t, "default")
	_, body := postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	job := gmap(t, body, "job")
	mustHave(t, job, "acked")
	mustHave(t, job, "acked_at")
	mustEqBool(t, job, "acked", false)
}

// --- TestAckGroupEndpoint ---

func TestAckGroupSuccess(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "timeout"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job3", "status": "success"})

	status, body := postJSON(t, "/api/groups/test/ack", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 2)
	mustEqStr(t, body, "group", "test")

	_, jb := getJSON(t, "/api/jobs?status=error,timeout")
	jobs := glist(t, jb, "jobs")
	for i := range jobs {
		mustEqBool(t, gelem(t, jobs, i), "acked", true)
	}
}

func TestAckGroupNotFound(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/groups/nonexistent/ack", nil)
	mustStatus(t, status, 404)
	mustEqStr(t, body, "error", "not_found")
}

func TestAckGroupCaseInsensitive(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "mygroup", "job": "job1", "status": "error"})

	status, body := postJSON(t, "/api/groups/MyGroup/ack", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 1)
}

func TestAckGroupNoErrors(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})

	status, body := postJSON(t, "/api/groups/test/ack", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 0)
}

func TestAckGroupSkipsAlreadyAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "error"})

	firstID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", firstID), nil)

	status, body := postJSON(t, "/api/groups/test/ack", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 1)
}

func TestAckGroupOnlyAffectsTargetGroup(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "group1", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "group2", "job": "job1", "status": "error"})

	_, body := postJSON(t, "/api/groups/group1/ack", nil)
	mustEqNum(t, body, "acked_count", 1)

	_, body = getJSON(t, "/api/groups")
	var group2 map[string]any
	for _, g := range glist(t, body, "groups") {
		gm := g.(map[string]any)
		if gm["name"] == "group2" {
			group2 = gm
			break
		}
	}
	if group2 == nil {
		t.Fatal("group2 not found")
	}
	mustEqNum(t, group2, "unhealthy_count", 1)
	mustEqNum(t, group2, "acked_count", 0)
}

func TestAckGroupHealthBecomesHealthy(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "success"})

	_, body := getJSON(t, "/api/groups")
	mustEqStr(t, gelem(t, glist(t, body, "groups"), 0), "health", "unhealthy")

	postJSON(t, "/api/groups/test/ack", nil)

	_, body = getJSON(t, "/api/groups")
	group := gelem(t, glist(t, body, "groups"), 0)
	mustEqStr(t, group, "health", "healthy")
	mustEqNum(t, group, "acked_count", 1)
}

// --- TestAckAllEndpoint ---

func TestAckAllSuccess(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "timeout"})
	postJSON(t, "/api/status", map[string]any{"group": "g2", "job": "job1", "status": "stale"})
	postJSON(t, "/api/status", map[string]any{"group": "g2", "job": "job2", "status": "success"})

	status, body := postJSON(t, "/api/ack-all", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 3)

	_, body = getJSON(t, "/api/health")
	mustEqStr(t, body, "status", "healthy")
	mustEqNum(t, body, "unhealthy", 0)
	mustEqNum(t, body, "acked", 3)
}

func TestAckAllNoErrors(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "success"})

	status, body := postJSON(t, "/api/ack-all", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 0)
}

func TestAckAllEmptyDatabase(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/ack-all", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 0)
}

func TestAckAllSkipsAlreadyAcked(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job2", "status": "error"})

	firstID := ackFirstJobID(t)
	postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", firstID), nil)

	status, body := postJSON(t, "/api/ack-all", nil)
	mustStatus(t, status, 200)
	mustEqNum(t, body, "acked_count", 1)
}

func TestAckAllAffectsAllGroups(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g2", "job": "job1", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g3", "job": "job1", "status": "error"})

	_, body := postJSON(t, "/api/ack-all", nil)
	mustEqNum(t, body, "acked_count", 3)

	_, body = getJSON(t, "/api/groups")
	groups := glist(t, body, "groups")
	for i := range groups {
		group := gelem(t, groups, i)
		mustEqNum(t, group, "unhealthy_count", 0)
		mustEqNum(t, group, "acked_count", 1)
	}
}

func TestAckAllIdempotent(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "test", "job": "job1", "status": "error"})

	_, body := postJSON(t, "/api/ack-all", nil)
	mustEqNum(t, body, "acked_count", 1)

	_, body = postJSON(t, "/api/ack-all", nil)
	mustEqNum(t, body, "acked_count", 0)
}
