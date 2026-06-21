//go:build contract

// Ported from contract/test_smoke.py — the harness reaches a live server on a pristine DB.
package contracttest

import "testing"

func TestHealthIsEmptyOnFreshDB(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/health")
	mustStatus(t, status, 200)
	mustEqStr(t, body, "status", "empty")
	mustEqNum(t, body, "total_jobs", 0)
}
