package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/statshed/statshed-server/internal/config"
)

func TestHealthcheckURL(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"0.0.0.0", 7828, "http://127.0.0.1:7828/api/health"},
		{"", 7828, "http://127.0.0.1:7828/api/health"},
		{"127.0.0.1", 9000, "http://127.0.0.1:9000/api/health"},
		{"::", 7828, "http://[::1]:7828/api/health"},
	}
	for _, c := range cases {
		got := healthcheckURL(config.Config{Host: c.host, Port: c.port})
		if got != c.want {
			t.Errorf("healthcheckURL(%q, %d) = %q, want %q", c.host, c.port, got, c.want)
		}
	}
}

func TestHealthcheckURLOverride(t *testing.T) {
	t.Setenv("HEALTHCHECK_URL", "http://example.test/custom")
	if got := healthcheckURL(config.Config{Host: "0.0.0.0", Port: 7828}); got != "http://example.test/custom" {
		t.Errorf("override = %q", got)
	}
}

func TestRunHealthcheckSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	t.Setenv("HEALTHCHECK_URL", srv.URL+"/api/health")
	if rc := runHealthcheck(config.Config{}); rc != 0 {
		t.Errorf("runHealthcheck = %d, want 0", rc)
	}
}

func TestRunHealthcheckFailsOnConnRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // the port is now closed -> connection refused

	t.Setenv("HEALTHCHECK_URL", url+"/api/health")
	if rc := runHealthcheck(config.Config{}); rc != 1 {
		t.Errorf("runHealthcheck = %d, want 1", rc)
	}
}

func TestRunHealthcheckFailsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	t.Setenv("HEALTHCHECK_URL", srv.URL+"/api/health")
	if rc := runHealthcheck(config.Config{}); rc != 1 {
		t.Errorf("runHealthcheck = %d, want 1", rc)
	}
}
