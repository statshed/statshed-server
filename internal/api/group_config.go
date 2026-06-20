package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/store"
)

func (h *handlers) getGroupConfig(w http.ResponseWriter, r *http.Request) {
	group, ok := h.groupOrNotFound(w, r, chi.URLParam(r, "name"))
	if !ok {
		return
	}
	global, err := h.store.Config(r.Context())
	if err != nil {
		h.internalError(w, "get config", err)
		return
	}
	writeJSON(w, http.StatusOK, groupConfigResponse(group, global))
}

// groupConfigResponse renders the group overrides + effective_* values + the legacy
// `group` alias (behavioral-map §2). Override fields are nullable (present, not absent).
func groupConfigResponse(g store.Group, global store.ConfigValues) map[string]any {
	effProgress := global.ProgressTimeoutMinutes
	if g.ProgressTimeoutMinutes != nil {
		effProgress = *g.ProgressTimeoutMinutes
	}
	effStaleness := global.StalenessTimeoutHours
	if g.StalenessTimeoutHours != nil {
		effStaleness = *g.StalenessTimeoutHours
	}
	effExpiration := global.ExpirationTimeoutHours
	if g.ExpirationTimeoutHours != nil {
		effExpiration = *g.ExpirationTimeoutHours
	}
	return map[string]any{
		"group":                              g.Name, // legacy alias for group_name
		"group_name":                         g.Name,
		"progress_timeout_minutes":           intPtrOrNil(g.ProgressTimeoutMinutes),
		"staleness_enabled":                  g.StalenessEnabled,
		"staleness_timeout_hours":            intPtrOrNil(g.StalenessTimeoutHours),
		"expiration_timeout_hours":           intPtrOrNil(g.ExpirationTimeoutHours),
		"effective_progress_timeout_minutes": effProgress,
		"effective_staleness_timeout_hours":  effStaleness,
		"effective_expiration_timeout_hours": effExpiration,
	}
}

func (h *handlers) updateGroupConfig(w http.ResponseWriter, r *http.Request) {
	rawName := chi.URLParam(r, "name")
	name := strings.ToLower(strings.TrimSpace(rawName))
	group, ok := h.groupOrNotFound(w, r, rawName)
	if !ok {
		return
	}
	data, ok := decodeJSONObject(w, r)
	if !ok {
		return
	}

	var patch store.GroupConfigPatch
	// Track the resulting effective overrides for the cross-field check + cascade.
	newStaleness := group.StalenessTimeoutHours
	newExpiration := group.ExpirationTimeoutHours
	newEnabled := group.StalenessEnabled
	expirationChanged := false

	if raw, present := data["progress_timeout_minutes"]; present {
		v, errMsg := validateNullableTimeout(raw, "progress_timeout_minutes",
			config.MinProgressTimeoutMinutes, config.MaxProgressTimeoutMinutes)
		if errMsg != "" {
			writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "progress_timeout_minutes")
			return
		}
		patch.ProgressSet, patch.Progress = true, v
	}
	if raw, present := data["staleness_enabled"]; present {
		b, isBool := raw.(bool)
		if !isBool {
			writeFieldError(w, http.StatusBadRequest, slugValidation,
				"staleness_enabled must be a boolean", "staleness_enabled")
			return
		}
		patch.Enabled, newEnabled = &b, b
	}
	if raw, present := data["staleness_timeout_hours"]; present {
		v, errMsg := validateNullableTimeout(raw, "staleness_timeout_hours",
			config.MinStalenessTimeoutHours, config.MaxStalenessTimeoutHours)
		if errMsg != "" {
			writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "staleness_timeout_hours")
			return
		}
		patch.StalenessSet, patch.Staleness, newStaleness = true, v, v
	}
	if raw, present := data["expiration_timeout_hours"]; present {
		v, errMsg := validateNullableTimeout(raw, "expiration_timeout_hours",
			config.MinExpirationTimeoutHours, config.MaxExpirationTimeoutHours)
		if errMsg != "" {
			writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, "expiration_timeout_hours")
			return
		}
		patch.ExpirationSet, patch.Expiration, newExpiration = true, v, v
		if !intPtrEqual(group.ExpirationTimeoutHours, v) {
			expirationChanged = true
		}
	}

	global, err := h.store.Config(r.Context())
	if err != nil {
		h.internalError(w, "get config", err)
		return
	}
	effStaleness := effectiveOr(newStaleness, global.StalenessTimeoutHours)
	effExpiration := effectiveOr(newExpiration, global.ExpirationTimeoutHours)
	// Cross-field rule: when staleness is enabled, effective staleness must be strictly
	// less than effective expiration. Uses a `fields` map, not `field`.
	if newEnabled && effStaleness >= effExpiration {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   slugValidation,
			"message": "staleness_timeout_hours must be less than expiration_timeout_hours",
			"fields": map[string]any{
				"staleness_timeout_hours": "must be less than expiration_timeout_hours",
			},
		})
		return
	}

	if err := h.store.SetGroupConfig(r.Context(), group.ID, patch); err != nil {
		h.internalError(w, "set group config", err)
		return
	}
	if expirationChanged {
		if err := h.store.CascadeGroupExpiration(r.Context(), group.ID, effExpiration); err != nil {
			h.internalError(w, "cascade group expiration", err)
			return
		}
	}

	updated, _, err := h.store.GroupByName(r.Context(), name)
	if err != nil {
		h.internalError(w, "re-read group", err)
		return
	}
	writeJSON(w, http.StatusOK, groupConfigResponse(updated, global))
}

// effectiveOr returns *override when set, else the global fallback.
func effectiveOr(override *int, global int) int {
	if override != nil {
		return *override
	}
	return global
}

// validateNullableTimeout validates an optional, nullable timeout override.
func validateNullableTimeout(raw any, field string, minVal, maxVal int) (*int, string) {
	if raw == nil {
		return nil, "" // null clears the override
	}
	v, ok := asConfigInt(raw)
	if !ok || v < minVal || v > maxVal {
		return nil, fmt.Sprintf("%s must be between %d and %d", field, minVal, maxVal)
	}
	return &v, ""
}
