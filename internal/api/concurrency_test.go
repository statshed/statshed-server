package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestConcurrentStatusUpsertsConverge fires many simultaneous POST /status for the SAME
// (group, job): the serialized write handle must resolve them to exactly one group and one
// job, with no 5xx (spec §5.2 race-safety).
func TestConcurrentStatusUpsertsConverge(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	const n = 24
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/status",
				strings.NewReader(`{"group":"builds","job":"nightly","status":"success"}`))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				codes[i] = -1
				return
			}
			codes[i] = resp.StatusCode
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusCreated {
			t.Errorf("request %d: status %d, want 201 (no 5xx under concurrency)", i, c)
		}
	}

	if got := countArray(t, srv.URL+"/api/groups", "groups"); got != 1 {
		t.Errorf("groups = %d, want 1 (concurrent upserts must not duplicate the group)", got)
	}
	if got := countArray(t, srv.URL+"/api/jobs", "jobs"); got != 1 {
		t.Errorf("jobs = %d, want 1 (concurrent upserts must not duplicate the job)", got)
	}
}

func countArray(t *testing.T, url, key string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	arr, _ := body[key].([]any)
	return len(arr)
}
