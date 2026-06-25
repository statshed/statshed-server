package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// deadlineRecorder is a fake base ResponseWriter exposing SetWriteDeadline, so we can assert
// http.ResponseController reaches the base through the middleware wrappers (I4).
type deadlineRecorder struct {
	http.ResponseWriter
	setCalled bool
}

func (d *deadlineRecorder) SetWriteDeadline(time.Time) error {
	d.setCalled = true
	return nil
}

// TestWrappersUnwrapForResponseController verifies SetWriteDeadline reaches the base writer
// through both statusRecorder and gzipResponseWriter (the SSE deadline was a silent no-op
// before Unwrap was added, I4).
func TestWrappersUnwrapForResponseController(t *testing.T) {
	base := &deadlineRecorder{ResponseWriter: httptest.NewRecorder()}
	// Wrap as the middleware stack does: base -> statusRecorder -> gzipResponseWriter.
	wrapped := &gzipResponseWriter{ResponseWriter: &statusRecorder{ResponseWriter: base}}

	rc := http.NewResponseController(wrapped)
	if err := rc.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline through wrappers = %v, want nil (Unwrap missing?)", err)
	}
	if !base.setCalled {
		t.Error("SetWriteDeadline did not reach the base writer through the Unwrap chain")
	}
}

// TestGzipSkipsRangeResponse: a 206 with Content-Range must NOT be gzipped — the Content-Range
// describes uncompressed bytes, so compressing the slice corrupts it (I5).
func TestGzipSkipsRangeResponse(t *testing.T) {
	h := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Range", "bytes 0-9/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("0123456789"))
	}))
	rec := serveGzip(h)
	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty for a 206 range response", ce)
	}
	if rec.Body.String() != "0123456789" {
		t.Errorf("body = %q, want the raw uncompressed slice", rec.Body.String())
	}
}

// TestGzipSkipsMultipartByteRanges: a 206 multipart/byteranges response has no top-level
// Content-Range but still must not be gzipped — the 206 status alone excludes it (I5).
func TestGzipSkipsMultipartByteRanges(t *testing.T) {
	h := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "multipart/byteranges; boundary=abc")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("--abc\r\n...\r\n--abc--\r\n"))
	}))
	if ce := serveGzip(h).Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty for a 206 multipart/byteranges response", ce)
	}
}

// TestGzipSkipsBodilessStatuses: 304/204/205/416 must not get a Content-Encoding.
func TestGzipSkipsBodilessStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNotModified, http.StatusNoContent,
		http.StatusResetContent, http.StatusRequestedRangeNotSatisfiable} {
		h := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(status)
		}))
		rec := serveGzip(h)
		if ce := rec.Header().Get("Content-Encoding"); ce != "" {
			t.Errorf("status %d: Content-Encoding = %q, want empty", status, ce)
		}
	}
}

// TestGzipCompresses200 confirms a normal full-body 200 still compresses (no regression).
func TestGzipCompresses200(t *testing.T) {
	h := gzipResponses(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	rec := serveGzip(h)
	if ce := rec.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip for a full 200", ce)
	}
}

// serveGzip drives h with a gzip-accepting request and returns the recorder.
func serveGzip(h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
