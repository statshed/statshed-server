package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/store"
)

func configResponse(cv store.ConfigValues) map[string]any {
	return map[string]any{
		"progress_timeout_minutes": cv.ProgressTimeoutMinutes,
		"staleness_timeout_hours":  cv.StalenessTimeoutHours,
		"expiration_timeout_hours": cv.ExpirationTimeoutHours,
	}
}

func (h *handlers) getConfig(w http.ResponseWriter, r *http.Request) {
	cv, err := h.store.Config(r.Context())
	if err != nil {
		h.internalError(w, "get config", err)
		return
	}
	writeJSON(w, http.StatusOK, configResponse(cv))
}

// configField is a validatable global config setting.
type configField struct {
	key      string
	min, max int
}

var configFields = []configField{
	{"progress_timeout_minutes", config.MinProgressTimeoutMinutes, config.MaxProgressTimeoutMinutes},
	{"staleness_timeout_hours", config.MinStalenessTimeoutHours, config.MaxStalenessTimeoutHours},
	{"expiration_timeout_hours", config.MinExpirationTimeoutHours, config.MaxExpirationTimeoutHours},
}

func (h *handlers) updateConfig(w http.ResponseWriter, r *http.Request) {
	data, ok := decodeJSONObject(w, r)
	if !ok {
		return
	}

	// Validate ALL present fields BEFORE any write (I7): a later invalid field must not leave an
	// earlier valid one persisted. The store then applies the collected values (and any
	// expiration cascade) in a single transaction.
	updates := make(map[string]int, len(configFields))
	for _, f := range configFields {
		raw, present := data[f.key]
		if !present {
			continue
		}
		value, valid := asConfigInt(raw)
		if !valid || value < f.min || value > f.max {
			writeFieldError(w, http.StatusBadRequest, slugValidation,
				fmt.Sprintf("%s must be between %d and %d", f.key, f.min, f.max), f.key)
			return
		}
		updates[f.key] = value
	}

	if err := h.store.SetGlobalConfig(r.Context(), updates); err != nil {
		h.internalError(w, "set config", err)
		return
	}

	cv, err := h.store.Config(r.Context())
	if err != nil {
		h.internalError(w, "get config", err)
		return
	}
	writeJSON(w, http.StatusOK, configResponse(cv))
}

// internalError logs err and writes the JSON 500 envelope.
func (h *handlers) internalError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, "err", err)
	writeError(w, http.StatusInternalServerError, slugInternal,
		"An internal server error occurred")
}
