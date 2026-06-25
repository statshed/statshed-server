package store

import (
	"context"
	"testing"
	"time"
)

// TestAdminCleanupEmptyStatusesDeletesNothing: an explicitly-empty status set must delete
// nothing (parity with Python's status.in_([]) false predicate) rather than error on
// `status IN ()` (I2 / adjacent guard).
func TestAdminCleanupEmptyStatusesDeletesNothing(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "stale"}, old); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	res, err := s.AdminCleanup(ctx, []string{}, cutoff, false)
	if err != nil {
		t.Fatalf("AdminCleanup with empty statuses errored: %v", err)
	}
	if res.DeletedJobs != 0 || res.DeletedGroups != 0 {
		t.Errorf("empty statuses deleted %+v, want {0 0}", res)
	}
}

// TestAdminCleanupAtomicVsConcurrentInsert proves the I2 fix: the count/emptied-group
// computation and the deletes run in one write transaction, so a fresh job inserted into a
// to-be-emptied group mid-cleanup is seen and the group (and the new job) survive. On the
// pre-fix read-then-write split the emptied set was computed from s.read (blind to the
// uncommitted insert), so the group was wrongly deleted and FK CASCADE dropped the new job.
func TestAdminCleanupAtomicVsConcurrentInsert(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	r, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "old", Status: "stale"}, old)
	if err != nil {
		t.Fatal(err)
	}
	groupID := r.Job.GroupID
	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC) // the 2020 stale job matches

	// Hold the single write connection with an uncommitted insert of a fresh, NON-matching job
	// into the same group — a POST /status landing mid-cleanup.
	tx, err := s.Write().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO jobs (group_id, name, status, updated_at, created_at) VALUES (?, ?, 'success', ?, ?)",
		groupID, "fresh", formatStored(cutoff), formatStored(cutoff)); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}

	type result struct {
		r   CleanupResult
		err error
	}
	done := make(chan result, 1)
	go func() {
		cr, err := s.AdminCleanup(ctx, []string{"stale", "timeout"}, cutoff, false)
		done <- result{cr, err}
	}()

	// Let the cleanup goroutine reach (and block on) its write BeginTx, then commit the fresh
	// job. The fix recomputes the emptied set in-tx after this commit; the bug computed it
	// earlier on s.read.
	time.Sleep(150 * time.Millisecond)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	out := <-done
	if out.err != nil {
		t.Fatal(out.err)
	}
	if out.r.DeletedGroups != 0 {
		t.Errorf("DeletedGroups = %d, want 0 (group still holds the fresh job)", out.r.DeletedGroups)
	}
	if _, found, _ := s.GroupByName(ctx, "g"); !found {
		t.Error("group g was deleted despite holding a fresh job (cascade would drop the new job)")
	}
	var freshCount int
	if err := s.Write().QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE name = 'fresh'").Scan(&freshCount); err != nil {
		t.Fatal(err)
	}
	if freshCount != 1 {
		t.Errorf("fresh job count = %d, want 1 (it must survive cleanup)", freshCount)
	}
}
