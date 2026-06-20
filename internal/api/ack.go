package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/realtime"
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
		acked, err := h.store.MarkAcked(r.Context(), id, now)
		if err != nil {
			h.internalError(w, "ack job", err)
			return
		}
		if acked {
			job.Acked = true
			job.AckedAt = &now
			realtime.Publish(h.hub, "jobs_acked", map[string]any{
				"schema_version": 1,
				"job_ids":        []int{id},
				"group_id":       job.GroupID,
				"group_name":     job.GroupName,
				"acked_count":    1,
				"timestamp":      store.FormatAPITime(now),
			})
		} else if fresh, found, ferr := h.store.JobByID(r.Context(), id); ferr == nil && found {
			// A concurrent request acked it first: reflect the real state, emit no event.
			job = fresh
		}
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
	realtime.Publish(h.hub, "job_deleted", map[string]any{
		"schema_version": 1,
		"job_id":         id,
		"job_name":       job.Name,
		"group_id":       job.GroupID,
		"group_name":     job.GroupName,
		"timestamp":      store.FormatAPITime(time.Now().UTC()),
	})
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
	now := time.Now().UTC()
	ids, err := h.store.AckUnhealthy(r.Context(), &group.ID, now)
	if err != nil {
		h.internalError(w, "ack group", err)
		return
	}
	if len(ids) > 0 {
		sort.Ints(ids)
		realtime.Publish(h.hub, "jobs_acked", map[string]any{
			"schema_version": 1,
			"job_ids":        ids,
			"group_id":       group.ID,
			"group_name":     group.Name,
			"acked_count":    len(ids),
			"timestamp":      store.FormatAPITime(now),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"acked_count": len(ids), "group": group.Name})
}

func (h *handlers) ackAll(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	ids, err := h.store.AckUnhealthy(r.Context(), nil, now)
	if err != nil {
		h.internalError(w, "ack all", err)
		return
	}
	if len(ids) > 0 {
		sort.Ints(ids)
		realtime.Publish(h.hub, "jobs_acked", map[string]any{
			"schema_version": 1,
			"job_ids":        ids,
			"group_id":       nil,
			"group_name":     nil,
			"acked_count":    len(ids),
			"timestamp":      store.FormatAPITime(now),
		})
	}
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
