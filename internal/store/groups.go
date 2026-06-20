package store

import (
	"context"
	"database/sql"
	"errors"
)

// ListGroups returns every group (including zero-job groups) with its aggregate health
// counts, computed N+1-free in a bounded number of queries (one per group + three grouped
// aggregates, spec §5.1). Never inner-joins jobs, so empty groups are preserved.
func (s *Store) ListGroups(ctx context.Context) ([]GroupSummary, error) {
	groups, err := s.allGroups(ctx)
	if err != nil {
		return nil, err
	}

	statusByGroup := map[int]map[string]int{}
	rows, err := s.read.QueryContext(ctx,
		"SELECT group_id, status, COUNT(*) FROM jobs GROUP BY group_id, status")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var gid, count int
		var status string
		if err := rows.Scan(&gid, &status, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if IsValidStatus(status) {
			if statusByGroup[gid] == nil {
				statusByGroup[gid] = zeroStatusCounts()
			}
			statusByGroup[gid][status] = count
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()

	// unhealthy excludes acked (consistent with /health); acked counts acked jobs.
	unhealthyByGroup, err := s.groupCounts(ctx,
		"SELECT group_id, COUNT(*) FROM jobs "+
			"WHERE status IN ('error', 'timeout', 'stale') AND acked = 0 GROUP BY group_id")
	if err != nil {
		return nil, err
	}
	ackedByGroup, err := s.groupCounts(ctx,
		"SELECT group_id, COUNT(*) FROM jobs WHERE acked = 1 GROUP BY group_id")
	if err != nil {
		return nil, err
	}

	summaries := make([]GroupSummary, 0, len(groups))
	for _, g := range groups {
		statusCounts := statusByGroup[g.ID]
		if statusCounts == nil {
			statusCounts = zeroStatusCounts()
		}
		jobCount := 0
		for _, c := range statusCounts {
			jobCount += c
		}
		unhealthy := unhealthyByGroup[g.ID]
		summaries = append(summaries, GroupSummary{
			Group:          g,
			JobCount:       jobCount,
			Health:         healthFromCounts(jobCount, unhealthy, statusCounts["progress"]),
			UnhealthyCount: unhealthy,
			AckedCount:     ackedByGroup[g.ID],
			StatusCounts:   statusCounts,
		})
	}
	return summaries, nil
}

// GroupByName returns the group with the given (already-normalized) name. found=false when
// it does not exist.
func (s *Store) GroupByName(ctx context.Context, name string) (Group, bool, error) {
	g, err := scanGroup(s.read.QueryRowContext(ctx,
		"SELECT "+groupColumns+" FROM groups WHERE name = ?", name))
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, false, nil
	}
	if err != nil {
		return Group{}, false, err
	}
	return g, true, nil
}

func (s *Store) allGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.read.QueryContext(ctx, "SELECT "+groupColumns+" FROM groups ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	groups := make([]Group, 0)
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) groupCounts(ctx context.Context, query string) (map[int]int, error) {
	rows, err := s.read.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int]int{}
	for rows.Next() {
		var gid, count int
		if err := rows.Scan(&gid, &count); err != nil {
			return nil, err
		}
		out[gid] = count
	}
	return out, rows.Err()
}
