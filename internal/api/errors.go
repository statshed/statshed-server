package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorEnvelope is the JSON error body returned on every error path:
// {"error","message","field"?}. Group-config cross-field errors use a `fields` map
// instead of `field` and are written with writeFieldsError.
type errorEnvelope struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// Error slugs (behavioral-map §2). Status codes 200/201/400/404/405/413/415/500.
const (
	slugValidation     = "validation_error"
	slugBadRequest     = "bad_request"
	slugNotFound       = "not_found"
	slugInvalidState   = "invalid_state"
	slugMethodNotAllow = "method_not_allowed"
	slugPayloadTooBig  = "payload_too_large"
	slugUnsupportedTy  = "unsupported_media_type"
	slugInternal       = "internal_server_error"
	slugHTTPError      = "http_error"
)

// httpStatusSlug maps a status code to its default slug for generic HTTP errors.
var httpStatusSlug = map[int]string{
	http.StatusBadRequest:            slugBadRequest,
	http.StatusNotFound:              slugNotFound,
	http.StatusMethodNotAllowed:      slugMethodNotAllow,
	http.StatusRequestEntityTooLarge: slugPayloadTooBig,
	http.StatusUnsupportedMediaType:  slugUnsupportedTy,
	http.StatusInternalServerError:   slugInternal,
}

func slugForStatus(status int) string {
	if s, ok := httpStatusSlug[status]; ok {
		return s
	}
	return slugHTTPError
}

// writeJSON marshals v and writes it with the given status as application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The header/status are already sent; nothing to do but log.
		slog.Error("encode JSON response", "err", err)
	}
}

// writeError writes the standard JSON error envelope.
func writeError(w http.ResponseWriter, status int, slug, message string) {
	writeJSON(w, status, errorEnvelope{Error: slug, Message: message})
}

// writeFieldError writes an error envelope naming the offending field.
func writeFieldError(w http.ResponseWriter, status int, slug, message, field string) {
	writeJSON(w, status, errorEnvelope{Error: slug, Message: message, Field: field})
}

// writeHTTPError writes a generic HTTP error using the slug mapped from the status.
func writeHTTPError(w http.ResponseWriter, status int, message string) {
	writeError(w, status, slugForStatus(status), message)
}
