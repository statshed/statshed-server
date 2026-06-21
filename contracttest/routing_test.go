//go:build contract

// Re-authored from contract/test_routing.py — API under /api, no SPA fallback (no_spa profile).
package contracttest

import "testing"

func TestHealthIsUnderAPI(t *testing.T) {
	begin(t, "no_spa")
	status, _ := getJSON(t, "/api/health")
	mustStatus(t, status, 200)
}

func TestHealthNotAtRoot(t *testing.T) {
	begin(t, "no_spa")
	// Bare /health is not the API and there is no SPA fallback -> 404.
	status, _ := getJSON(t, "/health")
	mustStatus(t, status, 404)
}

func TestStatusIsUnderAPI(t *testing.T) {
	begin(t, "no_spa")
	status, _ := postJSON(t, "/status", map[string]any{})
	mustStatus(t, status, 404)
}
