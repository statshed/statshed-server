//go:build contract

// Ported from contract/test_concurrency.py — concurrency / integrity invariants over HTTP.
// Rapid sequential writes don't corrupt state; concurrent POSTs to the same new group/job
// yield no 5xx and exactly one group/one job. All default profile.
package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// ccRawPost posts to /api/status from a goroutine WITHOUT the t.Fatalf-bearing helpers
// (t.Fatalf is illegal off the test goroutine); it returns the status and any transport error.
func ccRawPost(payload map[string]any) (int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/status", bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// ccConcurrentPosts fires all payloads at /api/status concurrently and returns their statuses.
func ccConcurrentPosts(t *testing.T, payloads []map[string]any) []int {
	t.Helper()
	codes := make([]int, len(payloads))
	errs := make([]error, len(payloads))
	var wg sync.WaitGroup
	for i := range payloads {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], errs[i] = ccRawPost(payloads[i])
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent POST %d: %v", i, err)
		}
	}
	return codes
}

func ccGroupsNamed(t *testing.T, name string) []map[string]any {
	t.Helper()
	_, body := getJSON(t, "/api/groups")
	var out []map[string]any
	for _, g := range glist(t, body, "groups") {
		if gm, ok := g.(map[string]any); ok && gm["name"] == name {
			out = append(out, gm)
		}
	}
	return out
}

// --- Rapid sequential writes ---

func TestConcurrencyRapidStatusSubmissionsSameJob(t *testing.T) {
	begin(t, "default")
	for i := 0; i < 20; i++ {
		code, _ := postJSON(t, "/api/status", map[string]any{
			"group": "rapid", "job": "shared-job", "status": "success",
			"message": fmt.Sprintf("Update %d", i),
		})
		mustStatus(t, code, 201)
	}
	_, body := getJSON(t, "/api/groups/rapid/jobs")
	jobs := glist(t, body, "jobs")
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	j0 := gelem(t, jobs, 0)
	mustEqStr(t, j0, "status", "success")
	mustEqStr(t, j0, "message", "Update 19") // last write wins
}

func TestConcurrencyRapidJobCreationSameGroup(t *testing.T) {
	begin(t, "default")
	for i := 0; i < 10; i++ {
		code, _ := postJSON(t, "/api/status", map[string]any{"group": "rapid", "job": fmt.Sprintf("job-%d", i), "status": "success"})
		mustStatus(t, code, 201)
	}
	_, body := getJSON(t, "/api/groups/rapid/jobs")
	jobs := glist(t, body, "jobs")
	if len(jobs) != 10 { // exactly 10 rows (a set comparison alone would hide a dup)
		t.Fatalf("len(jobs) = %d, want 10", len(jobs))
	}
	names := map[string]bool{}
	for _, j := range jobs {
		names[gstr(t, j.(map[string]any), "name")] = true
	}
	for i := 0; i < 10; i++ {
		if !names[fmt.Sprintf("job-%d", i)] {
			t.Errorf("missing job-%d", i)
		}
	}
}

func TestConcurrencyRapidSubmissionsMultipleGroups(t *testing.T) {
	begin(t, "default")
	for g := 0; g < 5; g++ {
		for j := 0; j < 4; j++ {
			code, _ := postJSON(t, "/api/status", map[string]any{"group": fmt.Sprintf("group-%d", g), "job": fmt.Sprintf("job-%d", j), "status": "success"})
			mustStatus(t, code, 201)
		}
	}
	_, body := getJSON(t, "/api/groups")
	groups := glist(t, body, "groups")
	if len(groups) != 5 {
		t.Fatalf("len(groups) = %d, want 5", len(groups))
	}
	for _, g := range groups {
		mustEqNum(t, g.(map[string]any), "job_count", 4)
	}
}

func TestConcurrencyRapidConfigUpdates(t *testing.T) {
	begin(t, "default")
	for _, v := range []int{5, 10, 15, 20, 25} {
		code, _ := putJSON(t, "/api/config", map[string]any{"progress_timeout_minutes": v})
		mustStatus(t, code, 200)
	}
	_, body := getJSON(t, "/api/config")
	mustEqNum(t, body, "progress_timeout_minutes", 25)
}

func TestConcurrencyRapidStatusTransitions(t *testing.T) {
	begin(t, "default")
	for _, status := range []string{"progress", "success", "error", "progress", "success"} {
		code, body := postJSON(t, "/api/status", map[string]any{"group": "t", "job": "j", "status": status})
		mustStatus(t, code, 201)
		mustEqStr(t, gmap(t, body, "job"), "status", status)
	}
	_, body := getJSON(t, "/api/jobs")
	mustEqStr(t, gelem(t, glist(t, body, "jobs"), 0), "status", "success")
}

// --- Concurrent invariants ---

func TestConcurrencySameGroupDifferentJobs(t *testing.T) {
	begin(t, "default")
	// Concurrent POSTs to the same NEW group -> exactly one group row, all jobs, no 5xx.
	var payloads []map[string]any
	for i := 0; i < 10; i++ {
		payloads = append(payloads, map[string]any{"group": "race", "job": fmt.Sprintf("job-%d", i), "status": "success"})
	}
	for i, code := range ccConcurrentPosts(t, payloads) {
		if code != 201 {
			t.Errorf("concurrent POST %d status = %d, want 201", i, code)
		}
	}
	groups := ccGroupsNamed(t, "race")
	if len(groups) != 1 {
		t.Fatalf("race groups = %d, want 1", len(groups))
	}
	mustEqNum(t, groups[0], "job_count", 10)
}

func TestConcurrencySameGroupSameJob(t *testing.T) {
	begin(t, "default")
	// Concurrent POSTs to the same NEW group AND job -> one group, one job, no 5xx.
	var payloads []map[string]any
	for i := 0; i < 10; i++ {
		payloads = append(payloads, map[string]any{"group": "race2", "job": "shared", "status": "success"})
	}
	for i, code := range ccConcurrentPosts(t, payloads) {
		if code != 201 {
			t.Errorf("concurrent POST %d status = %d, want 201", i, code)
		}
	}
	if groups := ccGroupsNamed(t, "race2"); len(groups) != 1 { // exactly one group row...
		t.Fatalf("race2 groups = %d, want 1", len(groups))
	}
	_, body := getJSON(t, "/api/groups/race2/jobs")
	if jobs := glist(t, body, "jobs"); len(jobs) != 1 { // ...and one job
		t.Fatalf("race2 jobs = %d, want 1", len(jobs))
	}
}
