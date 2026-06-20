package store

import "context"

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
