package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/realtime"
	"github.com/statshed/statshed-server/internal/store"
)

// TestInternalErrorEnvelope verifies an unexpected store failure surfaces as the JSON 500
// envelope (not an empty body or HTML). The read handle is closed so Health's query fails.
func TestInternalErrorEnvelope(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "err.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := store.Migrate(st.Write()); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(config.Config{CORSOrigins: []string{allowedOrigin}}, st, realtime.NewHub())

	// Inject a failure: a closed read handle makes the health aggregate error out.
	if err := st.Read().Close(); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "internal_server_error" {
		t.Errorf("error = %v, want internal_server_error", body["error"])
	}
	if body["message"] != "An internal server error occurred" {
		t.Errorf("message = %v, want the generic internal message", body["message"])
	}
}
