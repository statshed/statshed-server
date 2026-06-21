//go:build contract

// Ported from contract/test_jobs.py — GET /api/jobs listing and pagination.
package contracttest

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// jobsSeed POSTs count jobs (job0..jobN-1) into group, asserting each 201 (mirrors _make).
func jobsSeed(t *testing.T, count int, group, status string) {
	t.Helper()
	for i := 0; i < count; i++ {
		st, _ := postJSON(t, "/api/status", map[string]any{"group": group, "job": fmt.Sprintf("job%d", i), "status": status})
		mustStatus(t, st, 201)
	}
}

func jobNames(t *testing.T, body map[string]any) []string {
	t.Helper()
	jobs := glist(t, body, "jobs")
	out := make([]string, len(jobs))
	for i := range jobs {
		out[i] = gstr(t, gelem(t, jobs, i), "name")
	}
	return out
}

func TestGetJobsEmpty(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs")
	mustStatus(t, status, 200)
	if jobs := glist(t, body, "jobs"); len(jobs) != 0 {
		t.Errorf("jobs = %v, want []", jobs)
	}
	mustEqNum(t, body, "total", 0)
}

func TestGetJobsAll(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g2", "job": "job1", "status": "progress"})

	status, body := getJSON(t, "/api/jobs")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total", 3)
	if jobs := glist(t, body, "jobs"); len(jobs) != 3 {
		t.Errorf("len(jobs) = %d, want 3", len(jobs))
	}
}

func TestGetJobsFilterSingleStatus(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job3", "status": "success"})

	status, body := getJSON(t, "/api/jobs?status=success")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total", 2)
	jobs := glist(t, body, "jobs")
	for i := range jobs {
		if s := gstr(t, gelem(t, jobs, i), "status"); s != "success" {
			t.Errorf("job %d status = %q, want success", i, s)
		}
	}
}

func TestGetJobsFilterMultipleStatuses(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job3", "status": "timeout"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job4", "status": "progress"})

	// This matches the "Errors" card behavior (error + timeout).
	status, body := getJSON(t, "/api/jobs?status=error,timeout")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total", 2)
	jobs := glist(t, body, "jobs")
	set := map[string]bool{}
	for i := range jobs {
		set[gstr(t, gelem(t, jobs, i), "status")] = true
	}
	if len(set) != 2 || !set["error"] || !set["timeout"] {
		t.Errorf("statuses = %v, want {error, timeout}", set)
	}
}

func TestGetJobsInvalidStatus(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?status=invalid")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "error", "validation_error")
	mustContains(t, gstr(t, body, "message"), "Invalid status")
	mustEqStr(t, body, "field", "status")
}

func TestGetJobsOneValidOneInvalidStatus(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?status=success,badstatus")
	mustStatus(t, status, 400)
	mustContains(t, gstr(t, body, "message"), "Invalid status 'badstatus'")
}

func TestGetJobsEmptyStatusReturnsAll(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})

	status, body := getJSON(t, "/api/jobs?status=")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total", 2)
}

func TestGetJobsIncludesGroupName(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "my-group", "job": "job1", "status": "success"})

	_, body := getJSON(t, "/api/jobs")
	jobs := glist(t, body, "jobs")
	mustEqStr(t, gelem(t, jobs, 0), "group_name", "my-group")
}

func TestGetJobsOrderedByUpdatedAtDesc(t *testing.T) {
	begin(t, "default")
	for _, name := range []string{"older-job", "newer-job", "newest-job"} {
		st, _ := postJSON(t, "/api/status", map[string]any{"group": "test-group", "job": name, "status": "success"})
		mustStatus(t, st, 201)
	}

	now := time.Now().UTC()
	offsets := map[string]int{"newest-job": 0, "newer-job": 1, "older-job": 2}
	for name, hours := range offsets {
		ts := now.Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05.000000")
		backdate(t, "jobs", "name='"+name+"'", map[string]any{"updated_at": ts})
	}

	_, body := getJSON(t, "/api/jobs")
	// Verify order: newest first.
	if got := jobNames(t, body); !slices.Equal(got, []string{"newest-job", "newer-job", "older-job"}) {
		t.Errorf("order = %v, want [newest-job newer-job older-job]", got)
	}
}

func TestGetJobsResponseStructure(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{
		"group": "backups", "job": "daily-backup", "status": "error", "message": "Connection failed",
	})

	_, body := getJSON(t, "/api/jobs")
	mustHave(t, body, "jobs")
	mustHave(t, body, "total")

	job := gelem(t, glist(t, body, "jobs"), 0)
	for _, k := range []string{"id", "name", "group_id", "group_name", "status", "message", "updated_at", "created_at"} {
		mustHave(t, job, k)
	}
	mustEqStr(t, job, "name", "daily-backup")
	mustEqStr(t, job, "group_name", "backups")
	mustEqStr(t, job, "status", "error")
	mustEqStr(t, job, "message", "Connection failed")
}

func TestGetJobsFilterWhitespaceHandling(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "g1", "job": "job2", "status": "error"})

	// Status with extra whitespace (encoded): " success , error ".
	status, body := getJSON(t, "/api/jobs?status=%20success%20,%20error%20")
	mustStatus(t, status, 200)
	mustEqNum(t, body, "total", 2)
}

func TestJobsNoParamsReturnsAll(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/jobs")
	if jobs := glist(t, body, "jobs"); len(jobs) != 5 {
		t.Errorf("len(jobs) = %d, want 5", len(jobs))
	}
	mustEqNum(t, body, "total", 5)
}

func TestJobsLimitReturnsSliceWithFullTotal(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/jobs?limit=2")
	if jobs := glist(t, body, "jobs"); len(jobs) != 2 {
		t.Errorf("len(jobs) = %d, want 2", len(jobs))
	}
	mustEqNum(t, body, "total", 5)
}

func TestJobsOffsetPagesThroughInOrder(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 5, "g1", "success")
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format("2006-01-02 15:04:05.000000")
		backdate(t, "jobs", fmt.Sprintf("name='job%d'", i), map[string]any{"updated_at": ts})
	}

	_, page1 := getJSON(t, "/api/jobs?limit=2&offset=0")
	_, page2 := getJSON(t, "/api/jobs?limit=2&offset=2")
	_, page3 := getJSON(t, "/api/jobs?limit=2&offset=4")

	if got := jobNames(t, page1); !slices.Equal(got, []string{"job0", "job1"}) {
		t.Errorf("page1 = %v, want [job0 job1]", got)
	}
	if got := jobNames(t, page2); !slices.Equal(got, []string{"job2", "job3"}) {
		t.Errorf("page2 = %v, want [job2 job3]", got)
	}
	if got := jobNames(t, page3); !slices.Equal(got, []string{"job4"}) {
		t.Errorf("page3 = %v, want [job4]", got)
	}
	for _, page := range []map[string]any{page1, page2, page3} {
		mustEqNum(t, page, "total", 5)
	}
}

func TestJobsOffsetBeyondEndReturnsEmpty(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 3, "g1", "success")
	_, body := getJSON(t, "/api/jobs?limit=10&offset=100")
	if jobs := glist(t, body, "jobs"); len(jobs) != 0 {
		t.Errorf("jobs = %v, want []", jobs)
	}
	mustEqNum(t, body, "total", 3)
}

func TestJobsLimitIsClampedToMax(t *testing.T) {
	begin(t, "max_page_size")
	jobsSeed(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/jobs?limit=100")
	if jobs := glist(t, body, "jobs"); len(jobs) != 2 {
		t.Errorf("len(jobs) = %d, want 2 (clamped)", len(jobs))
	}
	mustEqNum(t, body, "total", 5)
}

func TestJobsPaginationRespectsStatusFilter(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 3, "g1", "success")
	jobsSeed(t, 2, "g1", "error")
	_, body := getJSON(t, "/api/jobs?status=error&limit=1")
	jobs := glist(t, body, "jobs")
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	mustEqStr(t, gelem(t, jobs, 0), "status", "error")
	// total reflects the filtered set, not all jobs.
	mustEqNum(t, body, "total", 2)
}

func TestJobsInvalidLimitReturns400(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?limit=abc")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "error", "validation_error")
	mustEqStr(t, body, "field", "limit")
}

func TestJobsZeroLimitReturns400(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?limit=0")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "limit")
}

func TestJobsNegativeOffsetReturns400(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?offset=-1")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "offset")
}

func TestJobsInvalidOffsetReturns400(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/jobs?offset=xyz")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "offset")
}

func TestJobsLimitPageReturnsPageSizeWithFullTotal(t *testing.T) {
	begin(t, "default")
	jobsSeed(t, 3, "g1", "success")
	_, body := getJSON(t, "/api/jobs?limit=2")
	if jobs := glist(t, body, "jobs"); len(jobs) != 2 {
		t.Errorf("len(jobs) = %d, want 2", len(jobs))
	}
	mustEqNum(t, body, "total", 3)
}
