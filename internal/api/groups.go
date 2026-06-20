package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/store"
)

func (h *handlers) listGroups(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.store.ListGroups(r.Context())
	if err != nil {
		slog.Error("list groups", "err", err)
		writeError(w, http.StatusInternalServerError, slugInternal,
			"An internal server error occurred")
		return
	}
	groups := make([]map[string]any, len(summaries))
	for i, gs := range summaries {
		groups[i] = gs.APIMap()
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// groupOrNotFound looks up a group by its normalized name, writing a 404 (ok=false) with
// the raw name in the message when it does not exist.
func (h *handlers) groupOrNotFound(w http.ResponseWriter, r *http.Request, rawName string) (store.Group, bool) {
	name := strings.ToLower(strings.TrimSpace(rawName))
	group, found, err := h.store.GroupByName(r.Context(), name)
	if err != nil {
		h.internalError(w, "group lookup", err)
		return store.Group{}, false
	}
	if !found {
		writeError(w, http.StatusNotFound, slugNotFound, "Group '"+rawName+"' not found")
		return store.Group{}, false
	}
	return group, true
}

func (h *handlers) getGroupJobs(w http.ResponseWriter, r *http.Request) {
	group, ok := h.groupOrNotFound(w, r, chi.URLParam(r, "name"))
	if !ok {
		return
	}

	limit, offset, errMsg, errField := parsePagination(r.URL.Query(), h.cfg.MaxJobsPageSize)
	if errMsg != "" {
		writeFieldError(w, http.StatusBadRequest, slugValidation, errMsg, errField)
		return
	}

	list, err := h.store.ListJobs(r.Context(), store.JobFilter{
		GroupID: &group.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		slog.Error("list group jobs", "err", err)
		writeError(w, http.StatusInternalServerError, slugInternal,
			"An internal server error occurred")
		return
	}

	resp := jobListResponse(list)
	resp["group"] = group.APIMap()
	writeJSON(w, http.StatusOK, resp)
}
