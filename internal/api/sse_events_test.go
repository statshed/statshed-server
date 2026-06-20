package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/realtime"
	"github.com/statshed/statshed-server/internal/store"
)

// sseHarness builds a test server with the tick hook enabled and returns it plus the store
// (for backdating jobs to drive timeout/expiration).
func sseHarness(t *testing.T) (*httptest.Server, *store.Store, *realtime.Hub) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "sse.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := store.Migrate(st.Write()); err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHub()
	cfg := config.Config{CORSOrigins: []string{allowedOrigin}, TestHooks: true}
	srv := httptest.NewServer(NewRouter(cfg, st, hub))
	t.Cleanup(srv.Close)
	return srv, st, hub
}

type sseFrame struct {
	event string
	data  map[string]any
}

type frameCollector struct {
	mu     sync.Mutex
	frames []sseFrame
	resp   *http.Response
	client *http.Client
	cancel context.CancelFunc
}

func connectSSE(t *testing.T, baseURL string, hub *realtime.Hub) *frameCollector {
	t.Helper()
	before := hub.ClientCount()
	// A dedicated client + cancelable request so close() tears the long-lived stream down
	// cleanly: cancelling closes the connection, which cancels the server handler's context
	// (it returns + unregisters), so httptest's Close does not block.
	client := &http.Client{}
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/events", nil)
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("/api/events status = %d, want 200", resp.StatusCode)
	}
	fc := &frameCollector{resp: resp, client: client, cancel: cancel}
	go fc.read()
	// ServeEvents flushes the 200 before it registers with the hub, so wait until the client
	// is actually subscribed; otherwise an immediately-following Broadcast reaches no one.
	for i := 0; i < 200; i++ {
		if hub.ClientCount() > before {
			return fc
		}
		time.Sleep(5 * time.Millisecond)
	}
	fc.close()
	t.Fatal("SSE client did not register with the hub within timeout")
	return nil
}

func (fc *frameCollector) read() {
	sc := bufio.NewScanner(fc.resp.Body)
	var buf []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(buf) > 0 {
				if f, ok := parseFrame(strings.Join(buf, "\n")); ok {
					fc.mu.Lock()
					fc.frames = append(fc.frames, f)
					fc.mu.Unlock()
				}
				buf = nil
			}
			continue
		}
		buf = append(buf, line)
	}
}

func (fc *frameCollector) close() {
	fc.cancel()
	_ = fc.resp.Body.Close()
	fc.client.CloseIdleConnections()
}

func parseFrame(raw string) (sseFrame, bool) {
	var f sseFrame
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "event: "):
			f.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &f.data)
		}
	}
	return f, f.event != ""
}

// await polls for up to ~3s for a frame of the given event satisfying match, returning its
// payload.
func (fc *frameCollector) await(t *testing.T, event string, match func(map[string]any) bool) map[string]any {
	t.Helper()
	for i := 0; i < 300; i++ {
		fc.mu.Lock()
		for _, f := range fc.frames {
			if f.event == event && (match == nil || match(f.data)) {
				fc.mu.Unlock()
				return f.data
			}
		}
		fc.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s frame matched within timeout", event)
	return nil
}

func (fc *frameCollector) count(event string) int {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	n := 0
	for _, f := range fc.frames {
		if f.event == event {
			n++
		}
	}
	return n
}

// postStatusJSON submits a status report and returns the created job's id.
func postStatusJSON(t *testing.T, baseURL, group, job, status string) int {
	t.Helper()
	body := fmt.Sprintf(`{"group":%q,"job":%q,"status":%q}`, group, job, status)
	resp, err := http.Post(baseURL+"/api/status", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /status %s/%s = %d, want 201", group, job, resp.StatusCode)
	}
	var decoded struct {
		Job struct {
			ID int `json:"id"`
		} `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Job.ID
}

func httpDo(t *testing.T, method, url string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func idSet(v any) map[int]bool {
	out := map[int]bool{}
	arr, _ := v.([]any)
	for _, x := range arr {
		if f, ok := x.(float64); ok {
			out[int(f)] = true
		}
	}
	return out
}

// TestSSEEventOracle drives each of the six events over HTTP and asserts the frame name +
// payload against the §8.4 oracle (id arrays compared as sets), plus no-duplicate
// group_created.
func TestSSEEventOracle(t *testing.T) {
	srv, st, hub := sseHarness(t)
	fc := connectSSE(t, srv.URL, hub)
	defer fc.close()

	// group_created (alpha) + status_update on the first report to a new group.
	buildID := postStatusJSON(t, srv.URL, "alpha", "build", "progress")
	gc := fc.await(t, "group_created", nil)
	if group, _ := gc["group"].(map[string]any); group["name"] != "alpha" {
		t.Errorf("group_created group = %v, want name alpha", gc["group"])
	}
	if v, _ := gc["schema_version"].(float64); v != 1 {
		t.Errorf("group_created schema_version = %v, want 1", gc["schema_version"])
	}
	su := fc.await(t, "status_update", func(d map[string]any) bool { return d["group_name"] == "alpha" })
	if su["previous_status"] != nil {
		t.Errorf("first status_update previous_status = %v, want null", su["previous_status"])
	}

	// A second job in the SAME group -> status_update, NO second group_created.
	errID := postStatusJSON(t, srv.URL, "alpha", "verify", "error")

	// jobs_acked on acking the error job.
	httpDo(t, http.MethodPost, fmt.Sprintf("%s/api/jobs/%d/ack", srv.URL, errID))
	ja := fc.await(t, "jobs_acked", func(d map[string]any) bool { return idSet(d["job_ids"])[errID] })
	if c, _ := ja["acked_count"].(float64); c != 1 {
		t.Errorf("jobs_acked acked_count = %v, want 1", ja["acked_count"])
	}

	// job_deleted on deleting the build job.
	httpDo(t, http.MethodDelete, fmt.Sprintf("%s/api/jobs/%d", srv.URL, buildID))
	fc.await(t, "job_deleted", func(d map[string]any) bool {
		id, _ := d["job_id"].(float64)
		return int(id) == buildID
	})

	// health_update (timeout): a backdated progress job, then a tick.
	deployID := postStatusJSON(t, srv.URL, "alpha", "deploy", "progress")
	backdateUpdatedAt(t, st, deployID, time.Now().UTC().Add(-2*time.Hour))
	httpDo(t, http.MethodPost, srv.URL+"/api/admin/run-checks")
	hu := fc.await(t, "health_update", func(d map[string]any) bool { return d["transition_type"] == "timeout" })
	if !idSet(hu["affected_job_ids"])[deployID] {
		t.Errorf("health_update(timeout) affected_job_ids = %v, want to include %d", hu["affected_job_ids"], deployID)
	}

	// job_expired: a backdated expires_at, then a tick.
	cleanupID := postStatusJSON(t, srv.URL, "alpha", "cleanup", "success")
	backdateExpiresAt(t, st, cleanupID, "2000-01-01 00:00:00.000000")
	httpDo(t, http.MethodPost, srv.URL+"/api/admin/run-checks")
	fc.await(t, "job_expired", func(d map[string]any) bool {
		id, _ := d["job_id"].(float64)
		return int(id) == cleanupID
	})

	if n := fc.count("group_created"); n != 1 {
		t.Errorf("group_created emitted %d times, want exactly 1 (no duplicates for an existing group)", n)
	}
}

// TestSSEReconnectReceivesEvents verifies a freshly (re)connected client receives events
// published after it connects — the basis for EventSource auto-reconnect resync.
func TestSSEReconnectReceivesEvents(t *testing.T) {
	srv, _, hub := sseHarness(t)

	first := connectSSE(t, srv.URL, hub)
	postStatusJSON(t, srv.URL, "g", "j", "success")
	first.await(t, "status_update", nil)
	first.close() // simulate a dropped connection

	second := connectSSE(t, srv.URL, hub)
	defer second.close()
	postStatusJSON(t, srv.URL, "g", "j", "error")
	second.await(t, "status_update", func(d map[string]any) bool {
		job, _ := d["job"].(map[string]any)
		return job["status"] == "error"
	})
}

func backdateUpdatedAt(t *testing.T, st *store.Store, id int, when time.Time) {
	t.Helper()
	if _, err := st.Write().Exec("UPDATE jobs SET updated_at = ? WHERE id = ?",
		when.UTC().Format("2006-01-02 15:04:05.000000"), id); err != nil {
		t.Fatal(err)
	}
}

func backdateExpiresAt(t *testing.T, st *store.Store, id int, stamp string) {
	t.Helper()
	if _, err := st.Write().Exec("UPDATE jobs SET expires_at = ? WHERE id = ?", stamp, id); err != nil {
		t.Fatal(err)
	}
}
