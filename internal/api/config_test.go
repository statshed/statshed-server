package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/statshed/statshed-server/internal/config"
)

// TestUpdateConfigRejectsWithoutPartialWrite verifies I7: when a PUT /api/config mixes a valid
// and an invalid field, it returns 400 and persists NOTHING — the earlier valid field must not
// be written (the pre-fix loop wrote each field as it went).
func TestUpdateConfigRejectsWithoutPartialWrite(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	// progress_timeout_minutes (10) is valid and validated first; staleness_timeout_hours (0)
	// is below its min of 1, so the request is rejected.
	put := strings.NewReader(`{"progress_timeout_minutes": 10, "staleness_timeout_hours": 0}`)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/config", put)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("PUT status = %d, want 400", resp.StatusCode)
	}

	// progress_timeout_minutes must still be the default — the rejected PUT wrote nothing.
	gresp, err := http.Get(srv.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gresp.Body.Close() }()
	var cfg map[string]any
	if err := json.NewDecoder(gresp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg["progress_timeout_minutes"]; got != float64(config.DefaultProgressTimeoutMinutes) {
		t.Errorf("progress_timeout_minutes = %v after a rejected PUT, want %d (no partial write)",
			got, config.DefaultProgressTimeoutMinutes)
	}
}
