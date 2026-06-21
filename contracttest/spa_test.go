//go:build contract

// Re-authored from contract/test_spa.py — SPA serving + precedence (with_spa profile).
// runner points STATIC_DIR at a synthetic dist (index.html with "StatShed", assets/app.js
// with "console.log"). The /api namespace must never be shadowed by the SPA fallback.
package contracttest

import "testing"

func TestSPAServedAtRoot(t *testing.T) {
	begin(t, "with_spa")
	resp, body := getRaw(t, "/")
	mustStatus(t, resp.StatusCode, 200)
	mustContains(t, body, "StatShed")
}

func TestSPAFallbackDoesNotShadowJobsAPI(t *testing.T) {
	begin(t, "with_spa")
	// An un-prefixed client route falls back to the SPA shell, not the API JSON.
	resp, body := getRaw(t, "/jobs")
	mustStatus(t, resp.StatusCode, 200)
	mustContains(t, body, "StatShed")
	// The real API still answers under /api.
	status, _ := getJSON(t, "/api/health")
	mustStatus(t, status, 200)
}

func TestRealAssetIsServed(t *testing.T) {
	begin(t, "with_spa")
	resp, body := getRaw(t, "/assets/app.js")
	mustStatus(t, resp.StatusCode, 200)
	mustContains(t, body, "console.log")
}

func TestUnknownAPIPathIs404NotSPA(t *testing.T) {
	begin(t, "with_spa")
	resp, body := getRaw(t, "/api/does-not-exist")
	mustStatus(t, resp.StatusCode, 404)
	mustNotContains(t, body, "StatShed") // JSON error envelope, not the SPA HTML
}
