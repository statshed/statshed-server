package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/statshed/statshed-server/internal/config"
)

// TestServerTimeouts documents the deliberate timeout policy (I8): ReadTimeout must stay
// generous so slow-but-valid <1MB uploads aren't cut mid-body, while ReadHeaderTimeout stays
// set to bound header slowloris and WriteTimeout stays 0 for indefinite SSE streaming.
func TestServerTimeouts(t *testing.T) {
	s := NewServer(config.Config{Host: "127.0.0.1", Port: 0}, http.NewServeMux())
	if s.http.ReadTimeout < time.Minute {
		t.Errorf("ReadTimeout = %v, want >= 1m so slow <1MB uploads aren't aborted (I8)", s.http.ReadTimeout)
	}
	if s.http.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout = 0, want set to bound header slowloris")
	}
	if s.http.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0 for indefinite SSE streaming", s.http.WriteTimeout)
	}
}
