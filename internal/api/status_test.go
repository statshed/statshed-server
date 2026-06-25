package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSplitLinesKeepEndsPythonParity covers the I13 break set: Python str.splitlines(keepends)
// also splits on \v \f \x1c \x1d \x1e \u0085 \u2028 \u2029, not just \n/\r/\r\n.
func TestSplitLinesKeepEndsPythonParity(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 1},
		{"a\nb\n", 2},
		{"a\nb", 2},
		{"a\r\nb\r\n", 2}, // \r\n is one terminator
		{"a\rb", 2},
		{"a\fb", 2},             // form feed
		{"a\vb", 2},             // vertical tab
		{"a\x1cb\x1dc\x1ed", 4}, // FS / GS / RS
		{"a\u0085b", 2},         // NEL
		{"a\u2028b", 2},         // LINE SEPARATOR
		{"a\u2029b", 2},         // PARAGRAPH SEPARATOR
	}
	for _, c := range cases {
		if got := len(splitLinesKeepEnds(c.in)); got != c.want {
			t.Errorf("splitLinesKeepEnds(%q) line count = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestStatusRejectsNonJSONContentType covers I11: a valid JSON body with a non-JSON Content-Type
// is rejected with 400 (Python's get_json(silent=True) behavior), while application/json (with
// parameters) is accepted.
func TestStatusRejectsNonJSONContentType(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()
	body := `{"group":"g","job":"j","status":"success"}`

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("valid JSON with text/plain = %d, want 400 (I11)", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/status", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Errorf("application/json; charset=utf-8 = %d, want 201", resp2.StatusCode)
	}
}

// TestEmptyMessageConsistentAcrossContentTypes covers I12: an explicitly-empty message yields
// "" (not null) on BOTH the JSON and multipart paths.
func TestEmptyMessageConsistentAcrossContentTypes(t *testing.T) {
	srv := httptest.NewServer(testRouter(t))
	defer srv.Close()

	jsonMsg := postStatusJSONMessage(t, srv.URL, `{"group":"g","job":"jj","status":"success","message":""}`)
	if jsonMsg != "" {
		t.Errorf("JSON empty message stored as %v, want \"\"", jsonMsg)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("group", "g")
	_ = mw.WriteField("job", "jm")
	_ = mw.WriteField("status", "success")
	_ = mw.WriteField("message", "")
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/status", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("multipart status = %d, want 201", resp.StatusCode)
	}
	if msg := jobMessage(t, resp); msg != "" {
		t.Errorf("multipart empty message stored as %v, want \"\" (matching JSON path)", msg)
	}
}

func postStatusJSONMessage(t *testing.T, base, body string) any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("JSON status = %d, want 201", resp.StatusCode)
	}
	return jobMessage(t, resp)
}

// jobMessage decodes a status response and returns the job's "message" field (nil if JSON null).
func jobMessage(t *testing.T, resp *http.Response) any {
	t.Helper()
	var out struct {
		Job map[string]any `json:"job"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Job["message"]
}
