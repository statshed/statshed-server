package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAdminCleanupRejectsHugeOlderThanDays verifies the I1 overflow guard: an older_than_days
// large enough to overflow the cutoff is rejected with 400 instead of wrapping to a future
// cutoff that would match (and delete) every job.
func TestAdminCleanupRejectsHugeOlderThanDays(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/admin/cleanup",
		strings.NewReader(`{"older_than_days": 200000}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an overflow-prone older_than_days", resp.StatusCode)
	}
}
