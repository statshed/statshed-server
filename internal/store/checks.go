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
	// Per-type group ids (sorted) for the per-transition-type health_update events; not in
	// the tick-hook JSON.
	TimeoutGroupIDs []int
	StaleGroupIDs   []int
}

// ExpiredJobInfo identifies one deleted job for its job_expired event.
type ExpiredJobInfo struct {
	ID        int
	Name      string
	GroupID   int
	GroupName string
}

// ExpirationResult is the structured result of an expiration pass.
type ExpirationResult struct {
	ExpiredJobIDs    []int
	AffectedGroupIDs []int
	ExpiredCount     int
	// Per-job details (sorted by id) for job_expired events; not in the tick-hook JSON.
	ExpiredJobs []ExpiredJobInfo
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
//
// AIDEV-NOTE: Each transition is a SINGLE atomic UPDATE whose WHERE re-evaluates the
// overdue predicate at execution time (status + updated_at < cutoff), with RETURNING for
// the affected ids. This closes the select-then-update race with a concurrent POST /status:
// a job refreshed to a healthy state (or a newer updated_at) between selection and write is
// no longer matched, so the pass never clobbers fresh work (codex review).
func (s *Store) RunTimeoutPass(ctx context.Context, now time.Time) (TimeoutResult, error) {
	globalProgress, err := s.ConfigValue(ctx, "progress_timeout_minutes", config.DefaultProgressTimeoutMinutes)
	if err != nil {
		return TimeoutResult{}, err
	}
	globalStaleness, err := s.ConfigValue(ctx, "staleness_timeout_hours", config.DefaultStalenessTimeoutHours)
	if err != nil {
		return TimeoutResult{}, err
	}
	nowStored := formatStored(now)

	timeouts, err := s.transitionOverdue(ctx, "timeout", "status = 'progress'",
		"progress_timeout_minutes", globalProgress, "minutes", nowStored)
	if err != nil {
		return TimeoutResult{}, err
	}
	stales, err := s.transitionOverdue(ctx, "stale",
		"status = 'success' AND group_id IN (SELECT id FROM groups WHERE staleness_enabled = 1)",
		"staleness_timeout_hours", globalStaleness, "hours", nowStored)
	if err != nil {
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
		TimeoutGroupIDs:  sortedGroupIDs(timeouts),
		StaleGroupIDs:    sortedGroupIDs(stales),
	}, nil
}

// transitionOverdue atomically transitions jobs matching baseCond whose updated_at is older
// than their effective timeout (group override of overrideCol else globalTimeout, in unit
// "minutes"/"hours"), returning the affected (id, group). overrideCol/unit/baseCond are
// fixed code constants (never user input).
func (s *Store) transitionOverdue(ctx context.Context, newStatus, baseCond, overrideCol string, globalTimeout int, unit, nowStored string) ([]jobGroup, error) {
	// cutoff per row = now - COALESCE(group override, global) <unit>; updated_at < cutoff.
	query := "UPDATE jobs SET status = ?, acked = 0, acked_at = NULL, updated_at = ? " +
		"WHERE " + baseCond +
		" AND updated_at < datetime(?, '-' || " +
		"COALESCE((SELECT " + overrideCol + " FROM groups WHERE id = jobs.group_id), ?) || ' " + unit + "') " +
		"RETURNING id, group_id"
	rows, err := s.write.QueryContext(ctx, query, newStatus, nowStored, nowStored, globalTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []jobGroup
	for rows.Next() {
		var id, groupID int
		if err := rows.Scan(&id, &groupID); err != nil {
			return nil, err
		}
		out = append(out, jobGroup{id: id, groupID: groupID})
	}
	return out, rows.Err()
}

// RunExpirationPass deletes jobs whose expires_at has passed, in batches of 100, returning
// the deleted ids and affected groups. Mirrors run_expiration_check.
//
// AIDEV-NOTE: The DELETE re-selects the still-expired ids in the same statement (subquery +
// RETURNING), so a job whose expires_at was just pushed into the future by a concurrent
// POST /status is not in the subquery and survives — closing the select-then-delete race
// (codex review). Such a job also drops out of the next batch's subquery, so the loop still
// terminates.
func (s *Store) RunExpirationPass(ctx context.Context, now time.Time) (ExpirationResult, error) {
	nowStored := formatStored(now)
	var jobs []ExpiredJobInfo
	groupSet := map[int]struct{}{}

	for {
		rows, err := s.write.QueryContext(ctx,
			"DELETE FROM jobs WHERE id IN "+
				"(SELECT id FROM jobs WHERE expires_at IS NOT NULL AND expires_at <= ? LIMIT 100) "+
				"RETURNING id, group_id, name",
			nowStored)
		if err != nil {
			return ExpirationResult{}, err
		}
		n := 0
		for rows.Next() {
			var id, groupID int
			var name string
			if err := rows.Scan(&id, &groupID, &name); err != nil {
				_ = rows.Close()
				return ExpirationResult{}, err
			}
			jobs = append(jobs, ExpiredJobInfo{ID: id, Name: name, GroupID: groupID})
			groupSet[groupID] = struct{}{}
			n++
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return ExpirationResult{}, err
		}
		_ = rows.Close()
		if n == 0 {
			break
		}
	}

	// The groups themselves survive expiration, so resolve their names for the events.
	names, err := s.groupNames(ctx, groupSet)
	if err != nil {
		return ExpirationResult{}, err
	}
	for i := range jobs {
		jobs[i].GroupName = names[jobs[i].GroupID]
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })

	expiredIDs := make([]int, len(jobs))
	for i, j := range jobs {
		expiredIDs[i] = j.ID
	}
	groups := make([]int, 0, len(groupSet))
	for g := range groupSet {
		groups = append(groups, g)
	}
	sort.Ints(groups)
	return ExpirationResult{
		ExpiredJobIDs:    expiredIDs,
		AffectedGroupIDs: groups,
		ExpiredCount:     len(jobs),
		ExpiredJobs:      jobs,
	}, nil
}

// groupNames maps each of the given group ids to its name.
func (s *Store) groupNames(ctx context.Context, ids map[int]struct{}) (map[int]string, error) {
	names := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	args := make([]any, 0, len(ids))
	for id := range ids {
		args = append(args, id)
	}
	rows, err := s.read.QueryContext(ctx,
		"SELECT id, name FROM groups WHERE id IN ("+placeholders(len(ids))+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

func sortedIDs(jgs []jobGroup) []int {
	out := make([]int, len(jgs))
	for i, jg := range jgs {
		out[i] = jg.id
	}
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
