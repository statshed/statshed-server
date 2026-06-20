package store

import (
	"context"
	"strings"
)

// ValidStatuses are the five job statuses, in canonical order (behavioral-map §5).
var ValidStatuses = []string{"success", "error", "progress", "timeout", "stale"}

// unhealthyStatuses count toward "unhealthy" (when unacked).
var unhealthyStatuses = map[string]bool{"error": true, "timeout": true, "stale": true}

// IsValidStatus reports whether status is one of the five valid statuses.
func IsValidStatus(status string) bool {
	for _, s := range ValidStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsUnhealthy reports whether a status is one of the unhealthy statuses.
func IsUnhealthy(status string) bool { return unhealthyStatuses[status] }

// zeroStatusCounts returns a map of every valid status -> 0.
func zeroStatusCounts() map[string]int {
	m := make(map[string]int, len(ValidStatuses))
	for _, s := range ValidStatuses {
		m[s] = 0
	}
	return m
}

// HealthSummary is the aggregate health snapshot (behavioral-map §2).
type HealthSummary struct {
	Status     string
	TotalJobs  int
	Healthy    int
	Unhealthy  int
	Acked      int
	InProgress int
	ByStatus   map[string]int
}

// Health computes the aggregate health in a single GROUP BY (no per-row loads, spec §5.1).
// by_status holds raw counts (including acked); unhealthy excludes acked jobs; the overall
// status follows the precedence empty > unhealthy > in_progress > healthy.
func (s *Store) Health(ctx context.Context) (HealthSummary, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT status, COUNT(*), COALESCE(SUM(CASE WHEN acked THEN 1 ELSE 0 END), 0) "+
			"FROM jobs GROUP BY status")
	if err != nil {
		return HealthSummary{}, err
	}
	defer func() { _ = rows.Close() }()

	byStatus := zeroStatusCounts()
	total, acked, unhealthy := 0, 0, 0
	for rows.Next() {
		var status string
		var count, ackedCount int
		if err := rows.Scan(&status, &count, &ackedCount); err != nil {
			return HealthSummary{}, err
		}
		if _, ok := byStatus[status]; ok {
			byStatus[status] = count
		}
		total += count
		acked += ackedCount
		if IsUnhealthy(status) {
			unhealthy += count - ackedCount
		}
	}
	if err := rows.Err(); err != nil {
		return HealthSummary{}, err
	}

	return HealthSummary{
		Status:     healthFromCounts(total, unhealthy, byStatus["progress"]),
		TotalJobs:  total,
		Healthy:    byStatus["success"],
		Unhealthy:  unhealthy,
		Acked:      acked,
		InProgress: byStatus["progress"],
		ByStatus:   byStatus,
	}, nil
}

func healthFromCounts(total, unhealthy, inProgress int) string {
	switch {
	case total == 0:
		return "empty"
	case unhealthy > 0:
		return "unhealthy"
	case inProgress > 0:
		return "in_progress"
	default:
		return "healthy"
	}
}

// JobFilter selects and pages jobs for GET /api/jobs.
type JobFilter struct {
	Statuses []string // empty -> no status filter
	Limit    *int     // nil -> no limit (return all)
	Offset   int
}

// JobList is a page of jobs plus the full matching total.
type JobList struct {
	Jobs  []Job
	Total int
}

// ListJobs returns jobs ordered updated_at DESC (id DESC tiebreak), optionally filtered by
// status and windowed by limit/offset. log_content is never selected (spec §5.1). Total is
// the full matching count; on the default (no limit, no offset) path it is len(jobs) with
// NO extra COUNT query (spec §5.1).
func (s *Store) ListJobs(ctx context.Context, f JobFilter) (JobList, error) {
	where, whereArgs := statusWhere(f.Statuses)

	query := "SELECT " + jobColumns + " " + jobFrom + where +
		" ORDER BY j.updated_at DESC, j.id DESC"
	args := append([]any(nil), whereArgs...)
	switch {
	case f.Limit != nil:
		query += " LIMIT ?"
		args = append(args, *f.Limit)
	case f.Offset > 0:
		query += " LIMIT -1" // SQLite requires a LIMIT before OFFSET
	}
	if f.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, f.Offset)
	}

	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return JobList{}, err
	}
	defer func() { _ = rows.Close() }()

	jobs := make([]Job, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return JobList{}, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return JobList{}, err
	}

	total := len(jobs)
	if f.Limit != nil || f.Offset > 0 {
		if err := s.read.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM jobs j"+where, whereArgs...,
		).Scan(&total); err != nil {
			return JobList{}, err
		}
	}
	return JobList{Jobs: jobs, Total: total}, nil
}

// statusWhere builds a `WHERE j.status IN (...)` clause (empty when no statuses).
func statusWhere(statuses []string) (string, []any) {
	if len(statuses) == 0 {
		return "", nil
	}
	args := make([]any, len(statuses))
	for i, st := range statuses {
		args[i] = st
	}
	return " WHERE j.status IN (" + placeholders(len(statuses)) + ")", args
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?, ", n-1) + "?"
}
