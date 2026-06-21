//go:build contract

// Ported from contract/test_background.py — cross-language background transitions via the
// guarded tick hook POST /api/admin/run-checks (registered under STATSHED_TEST_HOOKS, which
// the harness always sets). Rows are backdated, then the structured result + the resulting
// state (via GET) are asserted, including the per-type id split. All default profile.
package contracttest

import (
	"fmt"
	"testing"
	"time"
)

// bgAgo returns a stored-format ('YYYY-MM-DD HH:MM:SS.ffffff') timestamp d in the past.
func bgAgo(d time.Duration) string {
	return time.Now().UTC().Add(-d).Format("2006-01-02 15:04:05.000000")
}

func bgSubmit(t *testing.T, group, job, status string) int {
	t.Helper()
	code, body := postJSON(t, "/api/status", map[string]any{"group": group, "job": job, "status": status})
	mustStatus(t, code, 201)
	return gint(t, gmap(t, body, "job"), "id")
}

func bgRunChecks(t *testing.T) map[string]any {
	t.Helper()
	code, body := postJSON(t, "/api/admin/run-checks", nil)
	mustStatus(t, code, 200)
	return body
}

func bgJobByName(t *testing.T, name string) (map[string]any, bool) {
	t.Helper()
	_, body := getJSON(t, "/api/jobs")
	for _, j := range glist(t, body, "jobs") {
		if jm, ok := j.(map[string]any); ok && jm["name"] == name {
			return jm, true
		}
	}
	return nil, false
}

func bgRequireJob(t *testing.T, name string) map[string]any {
	t.Helper()
	j, ok := bgJobByName(t, name)
	if !ok {
		t.Fatalf("job %q not found", name)
	}
	return j
}

func bgContainsID(list []any, id int) bool {
	for _, v := range list {
		if n, ok := v.(float64); ok && int(n) == id {
			return true
		}
	}
	return false
}

func bgMustIDs(t *testing.T, list []any, want ...int) {
	t.Helper()
	if len(list) != len(want) {
		t.Fatalf("ids = %v, want %v", list, want)
	}
	for i, w := range want {
		if n, _ := list[i].(float64); int(n) != w {
			t.Errorf("ids[%d] = %v, want %d", i, list[i], w)
		}
	}
}

func TestBackgroundProgressTimesOut(t *testing.T) {
	begin(t, "default")
	id := bgSubmit(t, "g", "slow", "progress")
	backdate(t, "jobs", "name='slow'", map[string]any{"updated_at": bgAgo(10 * time.Minute)}) // past 5-min default
	tr := gmap(t, bgRunChecks(t), "timeout_result")
	mustEqNum(t, tr, "timeout_count", 1)
	if !bgContainsID(glist(t, tr, "timeout_job_ids"), id) {
		t.Errorf("timeout_job_ids %v missing id %d", tr["timeout_job_ids"], id)
	}
	mustEqStr(t, bgRequireJob(t, "slow"), "status", "timeout")
}

func TestBackgroundSuccessGoesStaleWhenEnabled(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "ok", "success")
	// Enable staleness with a 1h window (must be < the 24h expiration to pass validation).
	code, _ := putJSON(t, "/api/groups/g/config", map[string]any{"staleness_enabled": true, "staleness_timeout_hours": 1})
	mustStatus(t, code, 200)
	backdate(t, "jobs", "name='ok'", map[string]any{"updated_at": bgAgo(2 * time.Hour)})
	tr := gmap(t, bgRunChecks(t), "timeout_result")
	mustEqNum(t, tr, "stale_count", 1)
	mustEqStr(t, bgRequireJob(t, "ok"), "status", "stale")
}

func TestBackgroundErrorNeverTransitions(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "bad", "error")
	backdate(t, "jobs", "name='bad'", map[string]any{"updated_at": bgAgo(48 * time.Hour)})
	tr := gmap(t, bgRunChecks(t), "timeout_result")
	mustEqNum(t, tr, "timeout_count", 0)
	mustEqNum(t, tr, "stale_count", 0)
	mustEqStr(t, bgRequireJob(t, "bad"), "status", "error")
}

func TestBackgroundStalenessOffByDefault(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "ok", "success")
	backdate(t, "jobs", "name='ok'", map[string]any{"updated_at": bgAgo(48 * time.Hour)})
	tr := gmap(t, bgRunChecks(t), "timeout_result")
	mustEqNum(t, tr, "stale_count", 0)
	mustEqStr(t, bgRequireJob(t, "ok"), "status", "success")
}

func TestBackgroundGroupOverrideSuppressesTimeout(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "slow", "progress")
	code, _ := putJSON(t, "/api/groups/g/config", map[string]any{"progress_timeout_minutes": 30})
	mustStatus(t, code, 200)
	backdate(t, "jobs", "name='slow'", map[string]any{"updated_at": bgAgo(10 * time.Minute)}) // < the 30-min override
	tr := gmap(t, bgRunChecks(t), "timeout_result")
	mustEqNum(t, tr, "timeout_count", 0)
	mustEqStr(t, bgRequireJob(t, "slow"), "status", "progress")
}

func TestBackgroundExpiryDeletesJob(t *testing.T) {
	begin(t, "default")
	id := bgSubmit(t, "g", "gone", "success")
	backdate(t, "jobs", "name='gone'", map[string]any{"expires_at": bgAgo(1 * time.Hour)})
	er := gmap(t, bgRunChecks(t), "expiration_result")
	if !bgContainsID(glist(t, er, "expired_job_ids"), id) {
		t.Errorf("expired_job_ids %v missing id %d", er["expired_job_ids"], id)
	}
	if _, ok := bgJobByName(t, "gone"); ok {
		t.Errorf("job 'gone' still present after expiry")
	}
}

func TestBackgroundExpiryDeletesAckedJob(t *testing.T) {
	begin(t, "default")
	// Acking does not shield a job from expiry.
	id := bgSubmit(t, "g", "acked", "error")
	code, _ := postJSON(t, fmt.Sprintf("/api/jobs/%d/ack", id), nil)
	mustStatus(t, code, 200)
	backdate(t, "jobs", "name='acked'", map[string]any{"expires_at": bgAgo(1 * time.Hour)})
	er := gmap(t, bgRunChecks(t), "expiration_result")
	if !bgContainsID(glist(t, er, "expired_job_ids"), id) {
		t.Errorf("expired_job_ids %v missing id %d", er["expired_job_ids"], id)
	}
	if _, ok := bgJobByName(t, "acked"); ok {
		t.Errorf("job 'acked' still present after expiry")
	}
}

func TestBackgroundExpiryPreservesUnexpired(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "fresh", "success") // default expires_at is ~24h in the future
	er := gmap(t, bgRunChecks(t), "expiration_result")
	mustEqNum(t, er, "expired_count", 0)
	if _, ok := bgJobByName(t, "fresh"); !ok {
		t.Errorf("job 'fresh' should be preserved")
	}
}

func TestBackgroundTimeoutAndStaleSplitByType(t *testing.T) {
	begin(t, "default")
	timeoutID := bgSubmit(t, "g1", "prog", "progress")
	staleID := bgSubmit(t, "g2", "succ", "success")
	code, _ := putJSON(t, "/api/groups/g2/config", map[string]any{"staleness_enabled": true, "staleness_timeout_hours": 1})
	mustStatus(t, code, 200)
	backdate(t, "jobs", "name='prog'", map[string]any{"updated_at": bgAgo(10 * time.Minute)})
	backdate(t, "jobs", "name='succ'", map[string]any{"updated_at": bgAgo(2 * time.Hour)})

	tr := gmap(t, bgRunChecks(t), "timeout_result")
	bgMustIDs(t, glist(t, tr, "timeout_job_ids"), timeoutID)
	bgMustIDs(t, glist(t, tr, "stale_job_ids"), staleID)
	// The split must be clean: a stale job is never reported as a timeout.
	if bgContainsID(glist(t, tr, "timeout_job_ids"), staleID) {
		t.Errorf("stale id %d wrongly in timeout_job_ids", staleID)
	}
	if bgContainsID(glist(t, tr, "stale_job_ids"), timeoutID) {
		t.Errorf("timeout id %d wrongly in stale_job_ids", timeoutID)
	}
}

func TestBackgroundExpiresAtRefreshedOnUpdate(t *testing.T) {
	begin(t, "default")
	bgSubmit(t, "g", "j", "success")
	// Age both timestamps into the past, then update: the update must recompute expires_at
	// to the future (insert path: updated_at + expiration), proving it was refreshed.
	backdate(t, "jobs", "name='j'", map[string]any{"updated_at": bgAgo(1 * time.Hour), "expires_at": bgAgo(1 * time.Hour)})
	bgSubmit(t, "g", "j", "success")
	expiresAt := gstr(t, bgRequireJob(t, "j"), "expires_at")
	parsed, err := time.Parse("2006-01-02T15:04:05Z", expiresAt)
	if err != nil {
		t.Fatalf("parse expires_at %q: %v", expiresAt, err)
	}
	if !parsed.After(time.Now().UTC()) {
		t.Errorf("expires_at %v not in the future", parsed)
	}
}
