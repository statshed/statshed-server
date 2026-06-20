package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestEventsNoGzipAndCORS verifies GET /api/events is never gzip-compressed even when the
// client offers gzip (M2) and carries the reflected CORS headers for an allowed Origin (M1),
// through the full middleware stack.
func TestEventsNoGzipAndCORS(t *testing.T) {
	router, _ := testRouterWithHub(t)
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	// Setting Accept-Encoding manually disables the client's transparent decompression, so
	// any Content-Encoding the server set stays visible.
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Origin", allowedOrigin)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty (SSE must never be gzipped)", ce)
	}
	if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != allowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", acao, allowedOrigin)
	}
}
