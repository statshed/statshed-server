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

	expirationChanged := false
	var newExpiration int

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
		if f.key == "expiration_timeout_hours" {
			old, err := h.store.ConfigValue(r.Context(), f.key, config.DefaultExpirationTimeoutHours)
			if err != nil {
				h.internalError(w, "read expiration config", err)
				return
			}
			if value != old {
				expirationChanged = true
				newExpiration = value
			}
		}
		if err := h.store.SetConfigValue(r.Context(), f.key, value); err != nil {
			h.internalError(w, "set config", err)
			return
		}
	}

	if expirationChanged {
		if err := h.store.CascadeGlobalExpiration(r.Context(), newExpiration); err != nil {
			h.internalError(w, "cascade expiration", err)
			return
		}
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
