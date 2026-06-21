//go:build contract

// Ported from contract/test_groups.py — GET /api/groups and group-jobs pagination.
package contracttest

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// grpMake POSTs count jobs (job0..jobN-1) into group, asserting each 201.
func grpMake(t *testing.T, count int, group, status string) {
	t.Helper()
	for i := 0; i < count; i++ {
		s, _ := postJSON(t, "/api/status", map[string]any{
			"group": group, "job": "job" + strconv.Itoa(i), "status": status,
		})
		mustStatus(t, s, 201)
	}
}

// grpByName returns the group object in groups whose "name" == name (fatal if absent).
func grpByName(t *testing.T, groups []any, name string) map[string]any {
	t.Helper()
	for i := range groups {
		g := gelem(t, groups, i)
		if gstr(t, g, "name") == name {
			return g
		}
	}
	t.Fatalf("group %q not found", name)
	return nil
}

// grpJobNames extracts the "name" of each job, in order.
func grpJobNames(t *testing.T, jobs []any) []string {
	t.Helper()
	names := make([]string, len(jobs))
	for i := range jobs {
		names[i] = gstr(t, gelem(t, jobs, i), "name")
	}
	return names
}

func TestGetGroupsEmpty(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/groups")
	mustStatus(t, status, 200)
	if n := len(glist(t, body, "groups")); n != 0 {
		t.Errorf("groups = %d items, want 0", n)
	}
}

func TestGetGroupsWithData(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job2", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "monitoring", "job": "job1", "status": "error"})

	_, body := getJSON(t, "/api/groups")
	groups := glist(t, body, "groups")
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}

	backups := grpByName(t, groups, "backups")
	mustEqNum(t, backups, "job_count", 2)
	mustEqStr(t, backups, "health", "healthy")
	mustEqNum(t, gmap(t, backups, "status_counts"), "success", 2)

	monitoring := grpByName(t, groups, "monitoring")
	mustEqNum(t, monitoring, "job_count", 1)
	mustEqStr(t, monitoring, "health", "unhealthy")
}

func TestGetGroupsIncludesZeroJobGroup(t *testing.T) {
	begin(t, "default")
	// Bucket 2: a bare group cannot be made via the API, so insert it directly.
	insertGroup(t, "empty-group")
	postJSON(t, "/api/status", map[string]any{"group": "busy", "job": "job1", "status": "success"})

	_, body := getJSON(t, "/api/groups")
	groups := glist(t, body, "groups")

	names := map[string]bool{}
	for i := range groups {
		names[gstr(t, gelem(t, groups, i), "name")] = true
	}
	if len(names) != 2 || !names["empty-group"] || !names["busy"] {
		t.Errorf("group names = %v, want {empty-group, busy}", names)
	}

	empty := grpByName(t, groups, "empty-group")
	mustEqNum(t, empty, "job_count", 0)
	mustEqStr(t, empty, "health", "empty")
	mustEqNum(t, empty, "unhealthy_count", 0)
	mustEqNum(t, empty, "acked_count", 0)
	counts := gmap(t, empty, "status_counts")
	for _, s := range []string{"success", "error", "progress", "timeout", "stale"} {
		mustEqNum(t, counts, s, 0)
	}
}

func TestGetGroupJobs(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job2", "status": "progress"})

	status, body := getJSON(t, "/api/groups/backups/jobs")
	mustStatus(t, status, 200)
	mustEqStr(t, gmap(t, body, "group"), "name", "backups")
	if n := len(glist(t, body, "jobs")); n != 2 {
		t.Errorf("jobs = %d, want 2", n)
	}
}

func TestGetGroupJobsNotFound(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/groups/nonexistent/jobs")
	mustStatus(t, status, 404)
	mustEqStr(t, body, "error", "not_found")
}

func TestGetGroupJobsCaseInsensitive(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	status, _ := getJSON(t, "/api/groups/BACKUPS/jobs")
	mustStatus(t, status, 200)
}

func TestGroupJobsNoParamsReturnsAllWithTotal(t *testing.T) {
	begin(t, "default")
	grpMake(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs")
	if n := len(glist(t, body, "jobs")); n != 5 {
		t.Errorf("jobs = %d, want 5", n)
	}
	mustEqNum(t, body, "total", 5)
}

func TestGroupJobsLimitReturnsSliceWithFullTotal(t *testing.T) {
	begin(t, "default")
	grpMake(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs?limit=2")
	if n := len(glist(t, body, "jobs")); n != 2 {
		t.Errorf("jobs = %d, want 2", n)
	}
	mustEqNum(t, body, "total", 5)
}

func TestGroupJobsOffsetPagesThroughInOrder(t *testing.T) {
	begin(t, "default")
	// Bucket 3: backdate to distinct descending updated_at (newest first: job0..job4 oldest).
	grpMake(t, 5, "g1", "success")
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).Format("2006-01-02 15:04:05.000000")
		backdate(t, "jobs", "name='job"+strconv.Itoa(i)+"'", map[string]any{"updated_at": ts})
	}

	_, page1 := getJSON(t, "/api/groups/g1/jobs?limit=2&offset=0")
	_, page2 := getJSON(t, "/api/groups/g1/jobs?limit=2&offset=2")

	if got := strings.Join(grpJobNames(t, glist(t, page1, "jobs")), ","); got != "job0,job1" {
		t.Errorf("page1 = %q, want job0,job1", got)
	}
	if got := strings.Join(grpJobNames(t, glist(t, page2, "jobs")), ","); got != "job2,job3" {
		t.Errorf("page2 = %q, want job2,job3", got)
	}
	mustEqNum(t, page1, "total", 5)
	mustEqNum(t, page2, "total", 5)
}

func TestGroupJobsOffsetBeyondEndReturnsEmpty(t *testing.T) {
	begin(t, "default")
	grpMake(t, 3, "g1", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs?limit=10&offset=100")
	if n := len(glist(t, body, "jobs")); n != 0 {
		t.Errorf("jobs = %d, want 0", n)
	}
	mustEqNum(t, body, "total", 3)
}

func TestGroupJobsLimitIsClampedToMax(t *testing.T) {
	begin(t, "max_page_size")
	// Bucket 4: MAX_JOBS_PAGE_SIZE=2, so limit=100 clamps to 2 while total stays full.
	grpMake(t, 5, "g1", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs?limit=100")
	if n := len(glist(t, body, "jobs")); n != 2 {
		t.Errorf("jobs = %d, want 2", n)
	}
	mustEqNum(t, body, "total", 5)
}

func TestGroupJobsTotalIsScopedToGroup(t *testing.T) {
	begin(t, "default")
	grpMake(t, 3, "g1", "success")
	grpMake(t, 4, "g2", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs?limit=1")
	if n := len(glist(t, body, "jobs")); n != 1 {
		t.Errorf("jobs = %d, want 1", n)
	}
	mustEqNum(t, body, "total", 3)
}

func TestGroupJobsInvalidLimitReturns400(t *testing.T) {
	begin(t, "default")
	grpMake(t, 1, "g1", "success")
	status, body := getJSON(t, "/api/groups/g1/jobs?limit=abc")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "limit")
}

func TestGroupJobsZeroLimitReturns400(t *testing.T) {
	begin(t, "default")
	grpMake(t, 1, "g1", "success")
	status, body := getJSON(t, "/api/groups/g1/jobs?limit=0")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "limit")
}

func TestGroupJobsNegativeOffsetReturns400(t *testing.T) {
	begin(t, "default")
	grpMake(t, 1, "g1", "success")
	status, body := getJSON(t, "/api/groups/g1/jobs?offset=-1")
	mustStatus(t, status, 400)
	mustEqStr(t, body, "field", "offset")
}

// Ported from contract/test_groups.py::TestCliIntegration.
func TestCliListGroups(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "daily", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "weekly", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "monitoring", "job": "check", "status": "error"})

	status, body := getJSON(t, "/api/groups")
	mustStatus(t, status, 200)
	mustHave(t, body, "groups")
	groups := glist(t, body, "groups")
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}

	backups := grpByName(t, groups, "backups")
	mustHave(t, backups, "job_count")
	mustHave(t, backups, "health")
	mustHave(t, backups, "status_counts")
	mustEqStr(t, backups, "health", "healthy")
}

func TestCliGetJobs(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "builds", "job": "frontend", "status": "success"})
	postJSON(t, "/api/status", map[string]any{"group": "builds", "job": "backend", "status": "progress"})

	status, body := getJSON(t, "/api/groups/builds/jobs")
	mustStatus(t, status, 200)
	mustHave(t, body, "group")
	mustHave(t, body, "jobs")
	jobs := glist(t, body, "jobs")
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	job := gelem(t, jobs, 0)
	mustHave(t, job, "name")
	mustHave(t, job, "status")
	mustHave(t, job, "message")
	mustHave(t, job, "updated_at")
}

// Ported HTTP-observable slice of test_performance.py: a limit page returns the page size
// while total stays the full match count.
func TestGroupJobsLimitPageReturnsPageSizeWithFullTotal(t *testing.T) {
	begin(t, "default")
	grpMake(t, 3, "g1", "success")
	_, body := getJSON(t, "/api/groups/g1/jobs?limit=2")
	if n := len(glist(t, body, "jobs")); n != 2 {
		t.Errorf("jobs = %d, want 2", n)
	}
	mustEqNum(t, body, "total", 3)
}
