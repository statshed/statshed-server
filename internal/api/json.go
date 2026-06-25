package api

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strings"
)

// writeIfTooLarge writes the 413 payload_too_large envelope and returns true when err is
// the over-limit error from http.MaxBytesReader — i.e. a body that exceeded
// MaxContentLength without a Content-Length the bodyLimit precheck could catch (e.g.
// chunked transfer). Otherwise it returns false and writes nothing.
func writeIfTooLarge(w http.ResponseWriter, err error) bool {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeError(w, http.StatusRequestEntityTooLarge, slugPayloadTooBig,
			"Request body exceeds the maximum allowed size")
		return true
	}
	return false
}

func writeBadJSON(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, slugBadRequest, "JSON object required")
}

// isJSONContentType reports whether ct is application/json or an application/*+json type
// (ignoring parameters like charset). Empty/absent or any other type is rejected, matching
// Flask's is_json used by get_json (I11).
func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	// application/json or an application/*+json structured-syntax suffix (e.g.
	// application/problem+json); a non-application +json type like text/foo+json is rejected,
	// matching Flask's Request.is_json.
	return mt == "application/json" ||
		(strings.HasPrefix(mt, "application/") && strings.HasSuffix(mt, "+json"))
}

// readJSONObject reads the body as a JSON object (any object, including empty). ok=false
// (after writing a 400 bad_request, or 413 for an over-limit body) for malformed JSON or a
// non-object. This is the lenient form used by admin cleanup, which validates required
// fields rather than rejecting {} up front (Python's `data is None or not dict`).
func readJSONObject(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	// Require a JSON Content-Type (I11): the Python server's get_json(silent=True) returned None
	// -> 400 for a non-application/json body. Without this a valid JSON body sent as text/plain
	// (or with no type) would be silently accepted and mutate state.
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		writeBadJSON(w)
		return nil, false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if writeIfTooLarge(w, err) {
			return nil, false
		}
		writeBadJSON(w)
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		writeBadJSON(w)
		return nil, false
	}
	m, isObj := parsed.(map[string]any)
	if !isObj {
		writeBadJSON(w)
		return nil, false
	}
	return m, true
}

// decodeJSONObject reads the body as a NON-EMPTY JSON object, rejecting an empty object too
// (Python's `not data or not isinstance(data, dict)`). Used by /status and /config.
func decodeJSONObject(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	m, ok := readJSONObject(w, r)
	if !ok {
		return nil, false
	}
	if len(m) == 0 {
		writeBadJSON(w)
		return nil, false
	}
	return m, true
}

// intPtrOrNil renders a nullable int as the value or JSON null.
func intPtrOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// intPtrEqual reports whether two nullable ints are equal (both nil counts as equal).
func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// asConfigInt converts a decoded JSON value to an int, accepting only a whole-number JSON
// number (rejecting bool/string/fractional) — the equivalent of Python's is_valid_int
// (an int that is not a bool). ok=false means the value is not a valid integer.
func asConfigInt(value any) (int, bool) {
	f, ok := value.(float64) // JSON numbers decode to float64; bool/string fail here
	if !ok {
		return 0, false
	}
	if f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}
