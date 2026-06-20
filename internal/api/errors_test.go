package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, slugBadRequest, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != slugBadRequest || body["message"] != "bad input" {
		t.Errorf("body = %v", body)
	}
	if _, ok := body["field"]; ok {
		t.Errorf("field key present on a non-field error: %v", body)
	}
}

func TestWriteFieldError(t *testing.T) {
	w := httptest.NewRecorder()
	writeFieldError(w, http.StatusBadRequest, slugValidation, "missing", "group")

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != slugValidation || body["field"] != "group" {
		t.Errorf("body = %v, want validation_error + field=group", body)
	}
}

func TestWriteHTTPErrorUsesSlugForStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeHTTPError(w, http.StatusNotFound, "nope")
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != slugNotFound {
		t.Errorf("error = %v, want not_found", body["error"])
	}
}

func TestSlugForStatusFallback(t *testing.T) {
	if got := slugForStatus(http.StatusTeapot); got != slugHTTPError {
		t.Errorf("slugForStatus(418) = %q, want %q", got, slugHTTPError)
	}
}
