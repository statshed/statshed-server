package background

import (
	"time"

	"github.com/statshed/statshed-server/internal/realtime"
	"github.com/statshed/statshed-server/internal/store"
)

// PublishTimeout emits one health_update per transition type that actually occurred (matching
// the Python worker): each carries only its own ids and the correct transition_type, so a
// stale job is never reported as a timeout.
func PublishTimeout(hub *realtime.Hub, r store.TimeoutResult, now time.Time) {
	ts := store.FormatAPITime(now)
	if len(r.TimeoutJobIDs) > 0 {
		realtime.Publish(hub, "health_update", map[string]any{
			"schema_version":     1,
			"affected_job_ids":   r.TimeoutJobIDs,
			"affected_group_ids": r.TimeoutGroupIDs,
			"transition_type":    "timeout",
			"timestamp":          ts,
		})
	}
	if len(r.StaleJobIDs) > 0 {
		realtime.Publish(hub, "health_update", map[string]any{
			"schema_version":     1,
			"affected_job_ids":   r.StaleJobIDs,
			"affected_group_ids": r.StaleGroupIDs,
			"transition_type":    "stale",
			"timestamp":          ts,
		})
	}
}

// PublishExpiration emits one job_expired per deleted job.
func PublishExpiration(hub *realtime.Hub, r store.ExpirationResult, now time.Time) {
	ts := store.FormatAPITime(now)
	for _, j := range r.ExpiredJobs {
		realtime.Publish(hub, "job_expired", map[string]any{
			"schema_version": 1,
			"job_id":         j.ID,
			"job_name":       j.Name,
			"group_id":       j.GroupID,
			"group_name":     j.GroupName,
			"timestamp":      ts,
		})
	}
}
