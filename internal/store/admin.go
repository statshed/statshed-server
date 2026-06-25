package store

import (
	"context"
	"database/sql"
	"time"
)

// Stats is the GET /api/admin/stats payload.
type Stats struct {
	TotalJobs         int
	TotalGroups       int
	JobsByStatus      map[string]int
	DatabaseSizeBytes int64
}

// Stats counts jobs/groups in SQL (no row loads) and reports the SQLite DB size.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	st := Stats{JobsByStatus: zeroStatusCounts()}
	if err := s.read.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&st.TotalJobs); err != nil {
		return Stats{}, err
	}
	if err := s.read.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups").Scan(&st.TotalGroups); err != nil {
		return Stats{}, err
	}

	rows, err := s.read.QueryContext(ctx, "SELECT status, COUNT(*) FROM jobs GROUP BY status")
	if err != nil {
		return Stats{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return Stats{}, err
		}
		if _, ok := st.JobsByStatus[status]; ok {
			st.JobsByStatus[status] = count
		}
	}
	if err := rows.Err(); err != nil {
		return Stats{}, err
	}

	var pageCount, pageSize int64
	if err := s.read.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return Stats{}, err
	}
	if err := s.read.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return Stats{}, err
	}
	st.DatabaseSizeBytes = pageCount * pageSize
	return st, nil
}

// CleanupResult is the outcome of admin cleanup.
type CleanupResult struct {
	DeletedJobs   int
	DeletedGroups int
}

// AdminCleanup deletes jobs whose status is in statuses and whose updated_at is before
// cutoff, and removes any group whose entire job set was deleted (a group with some
// surviving job is kept). When dryRun is true it only counts.
func (s *Store) AdminCleanup(ctx context.Context, statuses []string, cutoff time.Time, dryRun bool) (CleanupResult, error) {
	// Build the match predicate. An empty status set deletes nothing — parity with the Python
	// server, where status.in_([]) renders a false predicate — which also avoids `status IN ()`.
	var matchCond string
	var matchArgs []any
	if len(statuses) == 0 {
		matchCond = "0 = 1"
	} else {
		matchCond = "status IN (" + placeholders(len(statuses)) + ") AND updated_at < ?"
		matchArgs = make([]any, 0, len(statuses)+1)
		for _, st := range statuses {
			matchArgs = append(matchArgs, st)
		}
		matchArgs = append(matchArgs, formatStored(cutoff))
	}

	// The count, the emptied-group computation, AND the deletes all run in ONE write
	// transaction. The write handle is SetMaxOpenConns(1), so this is atomic vs. every other
	// writer: it closes the I2 race where a concurrent POST /status adds a fresh job to a group
	// between selection and deletion — previously the group could still appear in the
	// emptied-set (read on s.read) and be deleted, dropping the new job via FK CASCADE. dryRun
	// computes inside the tx too, then rolls back (no writes).
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return CleanupResult{}, err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit; the dryRun path relies on it

	var deletedJobs int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM jobs WHERE "+matchCond, matchArgs...,
	).Scan(&deletedJobs); err != nil {
		return CleanupResult{}, err
	}

	emptyGroupIDs, err := emptiedGroupIDs(ctx, tx, matchCond, matchArgs)
	if err != nil {
		return CleanupResult{}, err
	}

	result := CleanupResult{DeletedJobs: deletedJobs, DeletedGroups: len(emptyGroupIDs)}
	if dryRun {
		return result, nil
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs WHERE "+matchCond, matchArgs...); err != nil {
		return CleanupResult{}, err
	}
	if len(emptyGroupIDs) > 0 {
		gArgs := make([]any, len(emptyGroupIDs))
		for i, id := range emptyGroupIDs {
			gArgs[i] = id
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM groups WHERE id IN ("+placeholders(len(emptyGroupIDs))+")", gArgs...,
		); err != nil {
			return CleanupResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return CleanupResult{}, err
	}
	return result, nil
}

// emptiedGroupIDs returns the ids of groups whose every job matches matchCond (so the group
// becomes empty once those jobs are deleted). Pre-existing zero-job groups are excluded (they
// do not appear in the GROUP BY over jobs). It runs on the cleanup transaction so the result is
// consistent with the deletes (I2).
func emptiedGroupIDs(ctx context.Context, tx *sql.Tx, matchCond string, matchArgs []any) ([]int, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT group_id FROM jobs GROUP BY group_id "+
			"HAVING COUNT(*) = SUM(CASE WHEN "+matchCond+" THEN 1 ELSE 0 END)",
		matchArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
