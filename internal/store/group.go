package store

import (
	"database/sql"
	"time"
)

// groupColumns selects a group in the API shape (group.to_dict, behavioral-map §2).
const groupColumns = "id, name, progress_timeout_minutes, staleness_timeout_hours, " +
	"staleness_enabled, expiration_timeout_hours, created_at"

// Group is a group row rendered for the API.
type Group struct {
	ID                     int
	Name                   string
	ProgressTimeoutMinutes *int
	StalenessTimeoutHours  *int
	StalenessEnabled       bool
	ExpirationTimeoutHours *int
	CreatedAt              time.Time
}

// APIMap renders the group in the exact shape of group.to_dict (created_at whole-second).
func (g Group) APIMap() map[string]any {
	return map[string]any{
		"id":                       g.ID,
		"name":                     g.Name,
		"progress_timeout_minutes": intOrNil(g.ProgressTimeoutMinutes),
		"staleness_timeout_hours":  intOrNil(g.StalenessTimeoutHours),
		"staleness_enabled":        g.StalenessEnabled,
		"expiration_timeout_hours": intOrNil(g.ExpirationTimeoutHours),
		"created_at":               formatAPI(g.CreatedAt),
	}
}

// GroupSummary is a group plus its aggregate health counts (GET /api/groups).
type GroupSummary struct {
	Group
	JobCount       int
	Health         string
	UnhealthyCount int
	AckedCount     int
	StatusCounts   map[string]int
}

// APIMap extends the group fields with the aggregate health fields.
func (gs GroupSummary) APIMap() map[string]any {
	m := gs.Group.APIMap()
	m["job_count"] = gs.JobCount
	m["health"] = gs.Health
	m["unhealthy_count"] = gs.UnhealthyCount
	m["acked_count"] = gs.AckedCount
	m["status_counts"] = gs.StatusCounts
	return m
}

func scanGroup(row interface{ Scan(...any) error }) (Group, error) {
	var g Group
	var prog, stale, exp sql.NullInt64
	var enabled int
	var createdAt string
	if err := row.Scan(&g.ID, &g.Name, &prog, &stale, &enabled, &exp, &createdAt); err != nil {
		return Group{}, err
	}
	g.ProgressTimeoutMinutes = nullIntPtr(prog)
	g.StalenessTimeoutHours = nullIntPtr(stale)
	g.ExpirationTimeoutHours = nullIntPtr(exp)
	g.StalenessEnabled = enabled != 0
	var err error
	if g.CreatedAt, err = parseStored(createdAt); err != nil {
		return Group{}, err
	}
	return g, nil
}
