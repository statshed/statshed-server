package api

import "testing"

// TestIsJSONContentType covers the I11 media-type check: application/json and application/*+json
// (with parameters) are accepted; everything else — including a non-application +json type, an
// empty/absent type, and a malformed value — is rejected, matching Flask's Request.is_json.
func TestIsJSONContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/problem+json", true},
		{"application/problem+json; charset=utf-8", true},
		{"", false},
		{"text/plain", false},
		{"application/x-www-form-urlencoded", false},
		{"multipart/form-data; boundary=x", false},
		{"text/foo+json", false}, // +json but not application/* -> rejected
		{"application/", false},
		{"not-a-media-type/", false},
	}
	for _, c := range cases {
		if got := isJSONContentType(c.ct); got != c.want {
			t.Errorf("isJSONContentType(%q) = %v, want %v", c.ct, got, c.want)
		}
	}
}
