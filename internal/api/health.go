package api

import "net/http"

// handleHealthStub is the Phase-2 placeholder for GET /api/health: it returns the
// empty-DB shape so the skeleton, the healthcheck subcommand, and the contract smoke test
// work. The real aggregate handler (single GROUP BY, precedence, by_status) replaces it in
// Task 3.1 once the store is wired in.
func handleHealthStub(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "empty",
		"total_jobs":  0,
		"healthy":     0,
		"unhealthy":   0,
		"acked":       0,
		"in_progress": 0,
		"by_status":   map[string]int{},
	})
}
