//go:build contract

// Ported from contract/test_security.py — exact security headers + CSP on /api/health.
package contracttest

import "testing"

// AIDEV-NOTE: The CSP string is byte-for-byte load-bearing; the script-src sha256 pins the
// inline theme-bootstrap script in index.html (behavioral-map §6).
const expectedCSP = "default-src 'self'; " +
	"script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'"

func TestSecurityHeadersPresent(t *testing.T) {
	begin(t, "default")
	resp, _ := getRaw(t, "/api/health")
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestCSPMatches(t *testing.T) {
	begin(t, "default")
	resp, _ := getRaw(t, "/api/health")
	if got := resp.Header.Get("Content-Security-Policy"); got != expectedCSP {
		t.Errorf("CSP =\n  %q\nwant\n  %q", got, expectedCSP)
	}
}
