package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// JobByID returns the rendered job with the given id (found=false when absent).
func (s *Store) JobByID(ctx context.Context, id int) (Job, bool, error) {
	job, err := scanJob(s.read.QueryRowContext(ctx,
		"SELECT "+jobColumns+" "+jobFrom+" WHERE j.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

// MarkAcked acks a single job (idempotent: a no-op when already acked, so acked_at is not
// re-stamped). It returns whether THIS call actually acked the row, so the caller only
// emits jobs_acked when it really transitioned the job (a concurrent ack that wins the race
// affects zero rows here).
//
// The WHERE also gates on an unhealthy status (matching AckUnhealthy): this closes the TOCTOU
// (I3) where a job recovers to success/progress — clearing its ack — between the handler's
// status read and this UPDATE; without the gate it would re-ack a now-healthy job (a state
// recovery is meant to prevent). The handler still does the user-facing IsUnhealthy check to
// return a 400 for an already-healthy job; on a lost recovery race it re-reads the real state.
func (s *Store) MarkAcked(ctx context.Context, id int, now time.Time) (bool, error) {
	res, err := s.write.ExecContext(ctx,
		"UPDATE jobs SET acked = 1, acked_at = ? "+
			"WHERE id = ? AND acked = 0 AND status IN ('error', 'timeout', 'stale')",
		formatStored(now), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AckUnhealthy acks all unacked unhealthy jobs, optionally scoped to one group, and
// returns the affected job ids (ascending).
func (s *Store) AckUnhealthy(ctx context.Context, groupID *int, now time.Time) ([]int, error) {
	query := "UPDATE jobs SET acked = 1, acked_at = ? " +
		"WHERE status IN ('error', 'timeout', 'stale') AND acked = 0"
	args := []any{formatStored(now)}
	if groupID != nil {
		query += " AND group_id = ?"
		args = append(args, *groupID)
	}
	query += " RETURNING id"

	rows, err := s.write.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteJob deletes a job, returning its rendered pre-delete state (found=false when absent).
func (s *Store) DeleteJob(ctx context.Context, id int) (Job, bool, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	job, err := scanJob(tx.QueryRowContext(ctx,
		"SELECT "+jobColumns+" "+jobFrom+" WHERE j.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs WHERE id = ?", id); err != nil {
		return Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}
