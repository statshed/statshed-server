package api

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
)

// decodeJSONObject reads the request body as a JSON object. It returns ok=false (after
// writing a 400 bad_request "JSON object required") for malformed JSON, a non-object, or
// an empty object — matching Python's `not data or not isinstance(data, dict)`.
func decodeJSONObject(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, slugBadRequest, "JSON object required")
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		writeError(w, http.StatusBadRequest, slugBadRequest, "JSON object required")
		return nil, false
	}
	m, isObj := parsed.(map[string]any)
	if !isObj || len(m) == 0 {
		writeError(w, http.StatusBadRequest, slugBadRequest, "JSON object required")
		return nil, false
	}
	return m, true
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
