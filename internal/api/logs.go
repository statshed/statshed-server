package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// defaultLogTail is the retrieval tail when ?tail is absent/invalid/non-positive.
const defaultLogTail = 1000

func (h *handlers) getJobLog(w http.ResponseWriter, r *http.Request) {
	group, ok := h.groupOrNotFound(w, r, chi.URLParam(r, "name"))
	if !ok {
		return
	}
	rawGroup := chi.URLParam(r, "name")
	rawJob := chi.URLParam(r, "job")
	jobName := strings.ToLower(strings.TrimSpace(rawJob))

	content, jobFound, err := h.store.JobLog(r.Context(), group.ID, jobName)
	if err != nil {
		h.internalError(w, "job log", err)
		return
	}
	if !jobFound {
		writeError(w, http.StatusNotFound, slugNotFound,
			fmt.Sprintf("Job '%s' not found in group '%s'", rawJob, rawGroup))
		return
	}
	if content == nil {
		writeError(w, http.StatusNotFound, slugNotFound,
			fmt.Sprintf("No log available for job '%s'", rawJob))
		return
	}

	returnAll := strings.EqualFold(r.URL.Query().Get("all"), "true")
	tail := defaultLogTail
	if t := r.URL.Query().Get("tail"); t != "" {
		// A non-integer or non-positive tail falls back to the default.
		if n, err := strconv.Atoi(t); err == nil && n >= 1 {
			tail = n
		}
	}

	lines := splitLinesKeepEnds(*content)
	total := len(lines)
	out := *content
	truncated := false
	if !returnAll && total > tail {
		lines = lines[total-tail:]
		out = strings.Join(lines, "")
		truncated = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"log":              out,
		"line_count":       len(lines),
		"truncated":        truncated,
		"total_line_count": total,
	})
}
