package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/statshed/statshed-server/internal/background"
	"github.com/statshed/statshed-server/internal/store"
)

// maxCleanupDays bounds older_than_days so the cutoff cannot overflow int64 nanoseconds (I1).
const maxCleanupDays = 100000

func (h *handlers) getAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		h.internalError(w, "admin stats", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_jobs":          stats.TotalJobs,
		"total_groups":        stats.TotalGroups,
		"jobs_by_status":      stats.JobsByStatus,
		"database_size_bytes": stats.DatabaseSizeBytes,
	})
}

// runChecks is the guarded test-only tick hook (POST /api/admin/run-checks): one timeout
// pass then one expiration pass, returning both structured results. Registered ONLY when
// STATSHED_TEST_HOOKS is set, so the contract suite can drive background transitions
// deterministically instead of waiting on the 60s scheduler (disabled under the same flag).
// When off, the route is absent and falls through to the JSON 404.
func (h *handlers) runChecks(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	timeoutResult, err := h.store.RunTimeoutPass(r.Context(), now)
	if err != nil {
		h.internalError(w, "run timeout pass", err)
		return
	}
	background.PublishTimeout(h.hub, timeoutResult, now)
	expirationResult, err := h.store.RunExpirationPass(r.Context(), now)
	if err != nil {
		h.internalError(w, "run expiration pass", err)
		return
	}
	background.PublishExpiration(h.hub, expirationResult, now)
	writeJSON(w, http.StatusOK, map[string]any{
		"timeout_result":    timeoutResult.APIMap(),
		"expiration_result": expirationResult.APIMap(),
	})
}

func (h *handlers) adminCleanup(w http.ResponseWriter, r *http.Request) {
	data, ok := readJSONObject(w, r) // lenient: {} is allowed, then validated below
	if !ok {
		return
	}

	rawDays, present := data["older_than_days"]
	if !present || rawDays == nil {
		writeFieldError(w, http.StatusBadRequest, slugValidation,
			"older_than_days is required", "older_than_days")
		return
	}
	days, valid := asConfigInt(rawDays)
	if !valid || days < 1 {
		writeFieldError(w, http.StatusBadRequest, slugValidation,
			"older_than_days must be a positive integer", "older_than_days")
		return
	}
	// Upper bound (I1): without it, a huge older_than_days makes
	// time.Duration(days)*24*time.Hour overflow int64 ns and wrap to a FUTURE cutoff, so the
	// `updated_at < cutoff` filter would match — and delete — EVERY job. 100000 days (~274y) is
	// far beyond any real retention and well under the ~106751-day overflow point. Reject (not
	// silently clamp) so an operator typo surfaces instead of mass-deleting.
	if days > maxCleanupDays {
		writeFieldError(w, http.StatusBadRequest, slugValidation,
			fmt.Sprintf("older_than_days must not exceed %d", maxCleanupDays), "older_than_days")
		return
	}

	statuses := []string{"stale", "timeout"} // default
	if raw, present := data["statuses"]; present {
		arr, isArr := raw.([]any)
		if !isArr {
			writeFieldError(w, http.StatusBadRequest, slugValidation,
				"statuses must be an array", "statuses")
			return
		}
		statuses = make([]string, 0, len(arr))
		for _, item := range arr {
			s, isStr := item.(string)
			if !isStr || !store.IsValidStatus(s) {
				writeFieldError(w, http.StatusBadRequest, slugValidation,
					fmt.Sprintf("Invalid status '%v'. Must be one of: %s", item, sortedStatusList),
					"statuses")
				return
			}
			statuses = append(statuses, s)
		}
	}

	dryRun := false
	if raw, present := data["dry_run"]; present {
		b, isBool := raw.(bool)
		if !isBool {
			writeFieldError(w, http.StatusBadRequest, slugValidation,
				"dry_run must be a boolean", "dry_run")
			return
		}
		dryRun = b
	}

	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	result, err := h.store.AdminCleanup(r.Context(), statuses, cutoff, dryRun)
	if err != nil {
		h.internalError(w, "admin cleanup", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted_jobs":   result.DeletedJobs,
		"deleted_groups": result.DeletedGroups,
		"dry_run":        dryRun,
	})
}
