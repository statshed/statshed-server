package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/store"
)

const allowedOrigin = "http://localhost:5173"

// testRouter builds the router backed by a fresh, migrated temp store.
func testRouter(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := store.Migrate(st.Write()); err != nil {
		t.Fatalf("store.Migrate: %v", err)
	}
	return NewRouter(config.Config{CORSOrigins: []string{allowedOrigin}}, st)
}

func TestSecurityHeadersAndCSP(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	want := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'; " +
			"script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; " +
			"style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; " +
			"connect-src 'self'; object-src 'none'; base-uri 'self'; " +
			"frame-ancestors 'none'; form-action 'self'",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "not_found" {
		t.Errorf("error = %v, want not_found", body["error"])
	}
}

func TestBodyLimitReturns413(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	oversized := make([]byte, config.MaxContentLength+1024)
	resp, err := http.Post(srv.URL+"/api/status", "application/json", bytes.NewReader(oversized))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "payload_too_large" {
		t.Errorf("error = %v, want payload_too_large", body["error"])
	}
}

func TestCORSReflectsAllowedOrigin(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/health", nil)
	req.Header.Set("Origin", allowedOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("ACAO = %q, want %q", got, allowedOrigin)
	}
	if !containsToken(resp.Header.Values("Vary"), "Origin") {
		t.Errorf("Vary = %v, want to contain Origin", resp.Header.Values("Vary"))
	}
}

func TestCORSDoesNotReflectDisallowedOrigin(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO = %q, want empty for a disallowed origin", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/status", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("ACAO = %q, want %q", got, allowedOrigin)
	}
	if m := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(m, "POST") {
		t.Errorf("Allow-Methods = %q, want to contain POST", m)
	}
	if h := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(h, "Content-Type") {
		t.Errorf("Allow-Headers = %q, want to contain Content-Type", h)
	}
}

func TestGzipNegotiation(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	// DisableCompression so we see the raw gzipped response (no transparent decode).
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if !containsToken(resp.Header.Values("Vary"), "Accept-Encoding") {
		t.Errorf("Vary = %v, want to contain Accept-Encoding", resp.Header.Values("Vary"))
	}
	// The body must be valid gzip decoding to the health JSON.
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode decompressed body: %v", err)
	}
	if body["status"] != "empty" {
		t.Errorf("decompressed status = %v, want empty", body["status"])
	}
}

// recordingHandler captures slog records for assertions.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func TestRequestLoggerRecordsStatusAndDuration(t *testing.T) {
	rec := &recordingHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/does-not-exist") // -> 404
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	var found bool
	for _, r := range rec.records {
		if r.Message != "request" {
			continue
		}
		found = true
		attrs := map[string]slog.Value{}
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value
			return true
		})
		if attrs["status"].Int64() != http.StatusNotFound {
			t.Errorf("logged status = %d, want 404", attrs["status"].Int64())
		}
		if _, ok := attrs["duration_ms"]; !ok {
			t.Errorf("missing duration_ms attr")
		}
		if attrs["path"].String() != "/api/does-not-exist" {
			t.Errorf("logged path = %q", attrs["path"].String())
		}
	}
	if !found {
		t.Fatal("no 'request' log record captured")
	}
}

func containsToken(values []string, token string) bool {
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if strings.TrimSpace(part) == token {
				return true
			}
		}
	}
	return false
}
