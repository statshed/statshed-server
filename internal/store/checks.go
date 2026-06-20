package store

import (
	"context"
	"sort"
	"time"

	"github.com/statshed/statshed-server/internal/config"
)

// TimeoutResult is the structured result of a timeout/stale pass. Every id array is sorted
// ascending and non-nil (renders as [] when empty) for the deterministic cross-language
// tick-hook contract (spec.md §8.4). The native dict carries the combined affected_job_ids
// plus counts; timeout_job_ids/stale_job_ids expose the per-type split.
type TimeoutResult struct {
	AffectedJobIDs   []int
	AffectedGroupIDs []int
	TimeoutCount     int
	StaleCount       int
	TimeoutJobIDs    []int
	StaleJobIDs      []int
}

// ExpirationResult is the structured result of an expiration pass.
type ExpirationResult struct {
	ExpiredJobIDs    []int
	AffectedGroupIDs []int
	ExpiredCount     int
}

func (r TimeoutResult) APIMap() map[string]any {
	return map[string]any{
		"affected_job_ids":   r.AffectedJobIDs,
		"affected_group_ids": r.AffectedGroupIDs,
		"timeout_count":      r.TimeoutCount,
		"stale_count":        r.StaleCount,
		"timeout_job_ids":    r.TimeoutJobIDs,
		"stale_job_ids":      r.StaleJobIDs,
	}
}

func (r ExpirationResult) APIMap() map[string]any {
	return map[string]any{
		"expired_job_ids":    r.ExpiredJobIDs,
		"affected_group_ids": r.AffectedGroupIDs,
		"expired_count":      r.ExpiredCount,
	}
}

type jobGroup struct {
	id      int
	groupID int
}

// RunTimeoutPass transitions overdue progress jobs to timeout and overdue success jobs in
// staleness-enabled groups to stale (effective timeout = group override else global else
// default), clearing the ack on transition. Mirrors run_timeout_check.
func (s *Store) RunTimeoutPass(ctx context.Context, now time.Time) (TimeoutResult, error) {
	globalProgress, err := s.ConfigValue(ctx, "progress_timeout_minutes", config.DefaultProgressTimeoutMinutes)
	if err != nil {
		return TimeoutResult{}, err
	}
	globalStaleness, err := s.ConfigValue(ctx, "staleness_timeout_hours", config.DefaultStalenessTimeoutHours)
	if err != nil {
		return TimeoutResult{}, err
	}

	timeouts, err := s.overdue(ctx, "progress", "progress_timeout_minutes", globalProgress, time.Minute, false, now)
	if err != nil {
		return TimeoutResult{}, err
	}
	stales, err := s.overdue(ctx, "success", "staleness_timeout_hours", globalStaleness, time.Hour, true, now)
	if err != nil {
		return TimeoutResult{}, err
	}

	if err := s.applyTransition(ctx, ids(timeouts), "timeout", now); err != nil {
		return TimeoutResult{}, err
	}
	if err := s.applyTransition(ctx, ids(stales), "stale", now); err != nil {
		return TimeoutResult{}, err
	}

	timeoutIDs := sortedIDs(timeouts)
	staleIDs := sortedIDs(stales)
	return TimeoutResult{
		AffectedJobIDs:   sortedInts(append(append([]int{}, timeoutIDs...), staleIDs...)),
		AffectedGroupIDs: sortedGroupIDs(append(append([]jobGroup{}, timeouts...), stales...)),
		TimeoutCount:     len(timeoutIDs),
		StaleCount:       len(staleIDs),
		TimeoutJobIDs:    timeoutIDs,
		StaleJobIDs:      staleIDs,
	}, nil
}

// overdue returns the jobs of the given status whose updated_at is older than their
// effective timeout. requireStaleness restricts to staleness-enabled groups.
func (s *Store) overdue(ctx context.Context, status, overrideCol string, globalTimeout int, unit time.Duration, requireStaleness bool, now time.Time) ([]jobGroup, error) {
	query := "SELECT j.id, j.group_id, j.updated_at, COALESCE(g." + overrideCol + ", ?) " +
		"FROM jobs j JOIN groups g ON g.id = j.group_id WHERE j.status = ?"
	if requireStaleness {
		query += " AND g.staleness_enabled = 1"
	}
	rows, err := s.read.QueryContext(ctx, query, globalTimeout, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []jobGroup
	for rows.Next() {
		var id, groupID, timeout int
		var updatedAt string
		if err := rows.Scan(&id, &groupID, &updatedAt, &timeout); err != nil {
			return nil, err
		}
		t, err := parseStored(updatedAt)
		if err != nil {
			return nil, err
		}
		if t.Before(now.Add(-time.Duration(timeout) * unit)) {
			out = append(out, jobGroup{id: id, groupID: groupID})
		}
	}
	return out, rows.Err()
}

func (s *Store) applyTransition(ctx context.Context, jobIDs []int, newStatus string, now time.Time) error {
	if len(jobIDs) == 0 {
		return nil
	}
	args := make([]any, 0, len(jobIDs)+2)
	args = append(args, newStatus, formatStored(now))
	for _, id := range jobIDs {
		args = append(args, id)
	}
	_, err := s.write.ExecContext(ctx,
		"UPDATE jobs SET status = ?, acked = 0, acked_at = NULL, updated_at = ? "+
			"WHERE id IN ("+placeholders(len(jobIDs))+")", args...)
	return err
}

// RunExpirationPass deletes jobs whose expires_at has passed, in batches of 100, returning
// the deleted ids and affected groups. Mirrors run_expiration_check.
func (s *Store) RunExpirationPass(ctx context.Context, now time.Time) (ExpirationResult, error) {
	nowStored := formatStored(now)
	expired := []int{}
	groupSet := map[int]struct{}{}

	for {
		rows, err := s.write.QueryContext(ctx,
			"SELECT id, group_id FROM jobs WHERE expires_at IS NOT NULL AND expires_at <= ? LIMIT 100",
			nowStored)
		if err != nil {
			return ExpirationResult{}, err
		}
		var batch []int
		for rows.Next() {
			var id, groupID int
			if err := rows.Scan(&id, &groupID); err != nil {
				_ = rows.Close()
				return ExpirationResult{}, err
			}
			batch = append(batch, id)
			groupSet[groupID] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return ExpirationResult{}, err
		}
		_ = rows.Close()
		if len(batch) == 0 {
			break
		}

		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		if _, err := s.write.ExecContext(ctx,
			"DELETE FROM jobs WHERE id IN ("+placeholders(len(batch))+")", args...); err != nil {
			return ExpirationResult{}, err
		}
		expired = append(expired, batch...)
	}

	sort.Ints(expired)
	groups := make([]int, 0, len(groupSet))
	for g := range groupSet {
		groups = append(groups, g)
	}
	sort.Ints(groups)
	return ExpirationResult{ExpiredJobIDs: expired, AffectedGroupIDs: groups, ExpiredCount: len(expired)}, nil
}

func ids(jgs []jobGroup) []int {
	out := make([]int, len(jgs))
	for i, jg := range jgs {
		out[i] = jg.id
	}
	return out
}

func sortedIDs(jgs []jobGroup) []int {
	out := ids(jgs)
	sort.Ints(out)
	return out
}

func sortedInts(in []int) []int {
	sort.Ints(in)
	return in
}

func sortedGroupIDs(jgs []jobGroup) []int {
	set := map[int]struct{}{}
	for _, jg := range jgs {
		set[jg.groupID] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Ints(out)
	return out
}
