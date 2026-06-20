package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/store"
)

func (h *handlers) ackJob(w http.ResponseWriter, r *http.Request) {
	id, ok := jobIDParam(w, r)
	if !ok {
		return
	}
	job, found, err := h.store.JobByID(r.Context(), id)
	if err != nil {
		h.internalError(w, "job lookup", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, slugNotFound, fmt.Sprintf("Job with id %d not found", id))
		return
	}
	if !store.IsUnhealthy(job.Status) {
		writeError(w, http.StatusBadRequest, slugInvalidState,
			fmt.Sprintf("Cannot ack job with status '%s'. "+
				"Only error, timeout, or stale jobs can be acknowledged.", job.Status))
		return
	}
	if !job.Acked {
		now := time.Now().UTC()
		if err := h.store.MarkAcked(r.Context(), id, now); err != nil {
			h.internalError(w, "ack job", err)
			return
		}
		job.Acked = true
		job.AckedAt = &now
		// AIDEV-NOTE: Phase 5 emits jobs_acked (single) here.
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job.APIMap()})
}

func (h *handlers) deleteJob(w http.ResponseWriter, r *http.Request) {
	id, ok := jobIDParam(w, r)
	if !ok {
		return
	}
	job, found, err := h.store.DeleteJob(r.Context(), id)
	if err != nil {
		h.internalError(w, "delete job", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, slugNotFound, fmt.Sprintf("Job with id %d not found", id))
		return
	}
	// AIDEV-NOTE: Phase 5 emits job_deleted here.
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted_job": job.APIMap(),
		"group_id":    job.GroupID,
		"group_name":  job.GroupName,
	})
}

func (h *handlers) ackGroup(w http.ResponseWriter, r *http.Request) {
	group, ok := h.groupOrNotFound(w, r, chi.URLParam(r, "name"))
	if !ok {
		return
	}
	ids, err := h.store.AckUnhealthy(r.Context(), &group.ID, time.Now().UTC())
	if err != nil {
		h.internalError(w, "ack group", err)
		return
	}
	// AIDEV-NOTE: Phase 5 emits jobs_acked (group) when len(ids) > 0.
	writeJSON(w, http.StatusOK, map[string]any{"acked_count": len(ids), "group": group.Name})
}

func (h *handlers) ackAll(w http.ResponseWriter, r *http.Request) {
	ids, err := h.store.AckUnhealthy(r.Context(), nil, time.Now().UTC())
	if err != nil {
		h.internalError(w, "ack all", err)
		return
	}
	// AIDEV-NOTE: Phase 5 emits jobs_acked (global, null group) when len(ids) > 0.
	writeJSON(w, http.StatusOK, map[string]any{"acked_count": len(ids)})
}

// jobIDParam parses the {id} URL param. The route constrains it to digits, so Atoi only
// fails on an out-of-range value; treat that as a 404 (no such job).
func jobIDParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusNotFound, slugNotFound, fmt.Sprintf("Job with id %s not found", idStr))
		return 0, false
	}
	return id, true
}
