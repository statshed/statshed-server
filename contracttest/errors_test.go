//go:build contract

// Ported from contract/test_errors.py — HTTP error envelopes. Every error path returns a
// JSON {error, message, field?} envelope, never HTML (behavioral-map §6). The forced-500 is
// per-language (cannot be forced over HTTP) and is not here — see coverage-map.md.
package contracttest

import (
	"net/http"
	"strings"
	"testing"
)

const errJSONCT = "application/json"

// errAssertJSON mirrors the Python _assert_json_error: status + a JSON {error, message}
// envelope. The harness decodes the body as JSON (failing the test otherwise), so a decoded
// map with error+message present is equivalent to the Python content-type + dict assertions.
func errAssertJSON(t *testing.T, status, want int, body map[string]any) {
	t.Helper()
	mustStatus(t, status, want)
	mustHave(t, body, "error")
	mustHave(t, body, "message")
}

func TestErrorsUnknownRouteReturnsJSON404(t *testing.T) {
	begin(t, "default")
	status, body := getJSON(t, "/api/no-such-route")
	errAssertJSON(t, status, 404, body)
}

func TestErrorsWrongMethodReturnsJSON405(t *testing.T) {
	begin(t, "default")
	// /api/health is GET-only.
	status, body := deleteJSON(t, "/api/health")
	errAssertJSON(t, status, 405, body)
}

func TestErrorsOversizedBodyReturnsJSON413(t *testing.T) {
	begin(t, "default")
	// MAX_CONTENT_LENGTH is 1 MB; exceed it.
	oversized := strings.Repeat("x", 1024*1024+1024)
	status, body := postRawJSON(t, "/api/status", errJSONCT, oversized)
	errAssertJSON(t, status, 413, body)
}

func TestErrorsMalformedJSONStatusReturnsJSON400(t *testing.T) {
	begin(t, "default")
	status, body := postRawJSON(t, "/api/status", errJSONCT, "{not valid json")
	errAssertJSON(t, status, 400, body)
}

func TestErrorsMalformedJSONConfigReturnsJSON400(t *testing.T) {
	begin(t, "default")
	// PUT /api/config with a non-JSON body; the harness has no putRaw helper, so compose
	// the request and run it through sendDecode (the same path the typed helpers use).
	req, err := http.NewRequest(http.MethodPut, baseURL+"/api/config", strings.NewReader("garbage"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", errJSONCT)
	status, body := sendDecode(t, req)
	errAssertJSON(t, status, 400, body)
}

func TestErrorsWrongContentTypeStatusReturnsJSON400(t *testing.T) {
	begin(t, "default")
	// Form-encoded body (not JSON, not multipart-with-log) must surface as a 400
	// "JSON object required" — NOT a 415 (behavioral-map §2 endpoint note).
	status, body := postRawJSON(t, "/api/status", "application/x-www-form-urlencoded", "group=a&job=b&status=success")
	errAssertJSON(t, status, 400, body)
}

// test_non_string_field_returns_json_400 is parametrized over group/job/status in Python;
// expanded here into one test per field so the case count matches the baseline.

func TestErrorsNonStringFieldGroupReturnsJSON400(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": 123, "job": "j", "status": "success"})
	errAssertJSON(t, status, 400, body)
	mustEqStr(t, body, "field", "group")
}

func TestErrorsNonStringFieldJobReturnsJSON400(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "g", "job": 123, "status": "success"})
	errAssertJSON(t, status, 400, body)
	mustEqStr(t, body, "field", "job")
}

func TestErrorsNonStringFieldStatusReturnsJSON400(t *testing.T) {
	begin(t, "default")
	status, body := postJSON(t, "/api/status", map[string]any{"group": "g", "job": "j", "status": 123})
	errAssertJSON(t, status, 400, body)
	mustEqStr(t, body, "field", "status")
}
