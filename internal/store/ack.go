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
// re-stamped). The caller is responsible for the unhealthy-status check.
func (s *Store) MarkAcked(ctx context.Context, id int, now time.Time) error {
	_, err := s.write.ExecContext(ctx,
		"UPDATE jobs SET acked = 1, acked_at = ? WHERE id = ? AND acked = 0",
		formatStored(now), id)
	return err
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
