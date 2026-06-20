package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/statshed/statshed-server/internal/store"
)

func (h *handlers) listJobs(w http.ResponseWriter, r *http.Request) {
	statuses, ok := parseStatusFilter(w, r.URL.Query().Get("status"))
	if !ok {
		return
	}
	limit, offset, errMsg, errField := parsePagination(r.URL.Query(), h.cfg.MaxJobsPageSize)
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, errField)
		return
	}

	list, err := h.store.ListJobs(r.Context(), store.JobFilter{
		Statuses: statuses, Limit: limit, Offset: offset,
	})
	if err != nil {
		slog.Error("list jobs", "err", err)
		writeError(w, http.StatusInternalServerError, slugInternal,
			"An internal server error occurred")
		return
	}
	writeJSON(w, http.StatusOK, jobListResponse(list))
}

// jobListResponse renders {"jobs": [...], "total": N}; jobs is always an array (never null).
func jobListResponse(list store.JobList) map[string]any {
	jobs := make([]map[string]any, len(list.Jobs))
	for i, j := range list.Jobs {
		jobs[i] = j.APIMap()
	}
	return map[string]any{"jobs": jobs, "total": list.Total}
}

// parseStatusFilter parses a comma-separated status filter. It returns ok=false after
// writing a 400 when a status is invalid; an empty/whitespace filter means "no filter".
func parseStatusFilter(w http.ResponseWriter, param string) ([]string, bool) {
	if strings.TrimSpace(param) == "" {
		return nil, true
	}
	var statuses []string
	for _, s := range strings.Split(param, ",") {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			statuses = append(statuses, s)
		}
	}
	for _, s := range statuses {
		if !store.IsValidStatus(s) {
			writeFieldError(w, http.StatusBadRequest, slugValidation,
				"Invalid status '"+s+"'. Must be one of: "+sortedStatusList, "status")
			return nil, false
		}
	}
	return statuses, true
}

// parsePagination parses opt-in limit/offset query params (errMsg != "" -> a 400
// validation_error on errField). A requested limit is clamped to maxPageSize.
func parsePagination(q url.Values, maxPageSize int) (limit *int, offset int, errMsg, errField string) {
	if p := strings.TrimSpace(q.Get("limit")); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, 0, "limit must be an integer", "limit"
		}
		if n < 1 {
			return nil, 0, "limit must be a positive integer", "limit"
		}
		if n > maxPageSize {
			n = maxPageSize
		}
		limit = &n
	}
	if p := strings.TrimSpace(q.Get("offset")); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, 0, "offset must be an integer", "offset"
		}
		if n < 0 {
			return nil, 0, "offset must be a non-negative integer", "offset"
		}
		offset = n
	}
	return limit, offset, "", ""
}
