package api

import (
	"compress/gzip"
	"log/slog"
	"net/http"
	"strings"
)

// contentSecurityPolicy is byte-for-byte identical to the Python server's CSP
// (behavioral-map §6). The script-src sha256 pins the inline theme-bootstrap script in
// index.html; if that script changes, this hash must be recomputed (D17).
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'sha256-7XUvd2lh/AE0pEp1W/qIkAQfU1nZDBEYKp8MFD3USaI='; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'"

// recoverer turns a panic in any inner middleware/handler into the JSON 500 envelope,
// so a bug never leaks a stack trace or an empty 200 (spec.md §11).
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, slugInternal,
					"An internal server error occurred")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders sets the exact security headers + CSP on every response (set before the
// handler runs, so they survive even a recovered 500 or a 413).
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}

// bodyLimit rejects a request whose body exceeds maxBytes with a JSON 413, matching the
// Python MAX_CONTENT_LENGTH behavior (reject up front on Content-Length). MaxBytesReader
// is a backstop for chunked/unknown-length bodies.
func bodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeError(w, http.StatusRequestEntityTooLarge, slugPayloadTooBig,
					"Request body exceeds the maximum allowed size")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// gzipResponses compresses responses with gzip when the client accepts it (matching
// flask-compress: gzip only — no br/deflate/zstd, S3). text/event-stream is never
// compressed (it would break SSE streaming, M2/D15). Vary: Accept-Encoding is always set.
//
// AIDEV-NOTE: flask-compress's ~500-byte minimum is intentionally not implemented;
// gzipping small bodies is harmless (clients decompress transparently) and the contract
// suite does not pin compression. Excluding text/event-stream IS load-bearing.
func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gz := &gzipResponseWriter{ResponseWriter: w}
		defer gz.finish()
		next.ServeHTTP(gz, r)
	})
}

func acceptsGzip(r *http.Request) bool {
	for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(enc, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wroteHeader {
		g.ResponseWriter.WriteHeader(status)
		return
	}
	g.wroteHeader = true
	if g.compressible(status) {
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Del("Content-Length") // compressed length differs
		g.gz = gzip.NewWriter(g.ResponseWriter)
	}
	g.ResponseWriter.WriteHeader(status)
}

// compressible reports whether a response with this status should be gzipped. It excludes:
// an already-encoded body; SSE (compression breaks streaming, M2/D15); range/partial responses
// (a Content-Range describes UNCOMPRESSED bytes, so gzipping the slice corrupts it, I5); and
// bodiless statuses (1xx, 204, 205, 206, 304, 416) where wrapping would emit a stray gzip
// stream onto a body that must be empty or is a byte range.
func (g *gzipResponseWriter) compressible(status int) bool {
	if g.Header().Get("Content-Encoding") != "" {
		return false
	}
	if strings.HasPrefix(g.Header().Get("Content-Type"), "text/event-stream") {
		return false
	}
	if g.Header().Get("Content-Range") != "" {
		return false
	}
	switch {
	case status < 200: // 1xx informational
		return false
	case status == http.StatusNoContent, status == http.StatusResetContent,
		status == http.StatusPartialContent, status == http.StatusNotModified,
		status == http.StatusRequestedRangeNotSatisfiable:
		return false
	}
	return true
}

// Unwrap exposes the wrapped writer so http.ResponseController can reach the base connection
// (e.g. the SSE per-write deadline, I4). Without it the controller stops here.
func (g *gzipResponseWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.gz != nil {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Flush forwards to the gzip writer (if active) and the underlying writer so streaming
// (SSE, which is never gzipped) keeps working.
func (g *gzipResponseWriter) Flush() {
	if g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponseWriter) finish() {
	if g.gz != nil {
		_ = g.gz.Close()
	}
}
