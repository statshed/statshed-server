package api

import (
	"log/slog"
	"net/http"
)

// healthResponse is the GET /api/health body (behavioral-map §2). by_status holds raw
// counts (including acked); unhealthy excludes acked; status follows the precedence
// empty > unhealthy > in_progress > healthy.
type healthResponse struct {
	Status     string         `json:"status"`
	TotalJobs  int            `json:"total_jobs"`
	Healthy    int            `json:"healthy"`
	Unhealthy  int            `json:"unhealthy"`
	Acked      int            `json:"acked"`
	InProgress int            `json:"in_progress"`
	ByStatus   map[string]int `json:"by_status"`
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	summary, err := h.store.Health(r.Context())
	if err != nil {
		slog.Error("health summary", "err", err)
		writeError(w, http.StatusInternalServerError, slugInternal,
			"An internal server error occurred")
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:     summary.Status,
		TotalJobs:  summary.TotalJobs,
		Healthy:    summary.Healthy,
		Unhealthy:  summary.Unhealthy,
		Acked:      summary.Acked,
		InProgress: summary.InProgress,
		ByStatus:   summary.ByStatus,
	})
}
