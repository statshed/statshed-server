package store

import (
	"context"
	"testing"
	"time"
)

func intsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRunTimeoutPassProgressToTimeout(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	// progress job updated 10 min ago -> past the 5 min default -> timeout
	overdue, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "overdue", Status: "progress"}, base.Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	// progress job updated 1 min ago -> stays progress
	fresh, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "fresh", Status: "progress"}, base.Add(-1*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.RunTimeoutPass(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if !intsEqual(res.TimeoutJobIDs, []int{overdue.Job.ID}) {
		t.Errorf("timeout_job_ids = %v, want [%d]", res.TimeoutJobIDs, overdue.Job.ID)
	}
	if len(res.StaleJobIDs) != 0 {
		t.Errorf("stale_job_ids = %v, want []", res.StaleJobIDs)
	}
	if res.TimeoutCount != 1 || res.StaleCount != 0 {
		t.Errorf("counts = %d/%d, want 1/0", res.TimeoutCount, res.StaleCount)
	}

	o, _, err := s.JobByID(ctx, overdue.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "timeout" {
		t.Errorf("overdue status = %q, want timeout", o.Status)
	}
	f, _, err := s.JobByID(ctx, fresh.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != "progress" {
		t.Errorf("fresh status = %q, want progress", f.Status)
	}
}

func TestRunTimeoutPassStaleNeedsEnabledGroup(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	on, err := s.UpsertJob(ctx, UpsertParams{GroupName: "on", JobName: "j", Status: "success"}, base.Add(-25*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write().Exec("UPDATE groups SET staleness_enabled = 1 WHERE name = 'on'"); err != nil {
		t.Fatal(err)
	}
	off, err := s.UpsertJob(ctx, UpsertParams{GroupName: "off", JobName: "j", Status: "success"}, base.Add(-25*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write().Exec("UPDATE groups SET staleness_enabled = 0 WHERE name = 'off'"); err != nil {
		t.Fatal(err)
	}

	res, err := s.RunTimeoutPass(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if !intsEqual(res.StaleJobIDs, []int{on.Job.ID}) {
		t.Errorf("stale_job_ids = %v, want [%d]", res.StaleJobIDs, on.Job.ID)
	}
	o, _, err := s.JobByID(ctx, off.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "success" {
		t.Errorf("staleness-disabled job status = %q, want success", o.Status)
	}
}

// TestTimeoutPassSkipsRefreshedJob locks the codex race fix: a job that was overdue but
// has since been refreshed (updated_at advanced) must NOT be transitioned — the atomic
// UPDATE re-checks updated_at < cutoff at execution time.
func TestTimeoutPassSkipsRefreshedJob(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	if _, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "progress"}, base.Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Refreshed to "now" (the concurrent POST /status case) just before the pass.
	r, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "progress"}, base)
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.RunTimeoutPass(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.TimeoutJobIDs) != 0 {
		t.Errorf("timeout_job_ids = %v, want [] (a refreshed job must not be timed out)", res.TimeoutJobIDs)
	}
	j, _, err := s.JobByID(ctx, r.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != "progress" {
		t.Errorf("refreshed job status = %q, want progress", j.Status)
	}
}

// TestExpirationPassSkipsRefreshedJob locks the codex race fix: a job whose expires_at was
// pushed into the future is not deleted — the atomic DELETE re-selects only still-expired
// rows.
func TestExpirationPassSkipsRefreshedJob(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	r, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "success"}, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write().Exec(
		"UPDATE jobs SET expires_at = ? WHERE id = ?", "2099-01-01 00:00:00.000000", r.Job.ID,
	); err != nil {
		t.Fatal(err)
	}

	res, err := s.RunExpirationPass(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ExpiredJobIDs) != 0 {
		t.Errorf("expired_job_ids = %v, want [] (a refreshed job must survive)", res.ExpiredJobIDs)
	}
	if _, found, err := s.JobByID(ctx, r.Job.ID); err != nil || !found {
		t.Errorf("refreshed job was wrongly deleted (found=%v, err=%v)", found, err)
	}
}

func TestRunExpirationPass(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	expired, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "old", Status: "success"}, base)
	if err != nil {
		t.Fatal(err)
	}
	live, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "new", Status: "success"}, base)
	if err != nil {
		t.Fatal(err)
	}
	// Backdate the one job's expires_at into the past (the other keeps base + 24h).
	if _, err := s.Write().Exec(
		"UPDATE jobs SET expires_at = ? WHERE id = ?", "2026-01-01 00:00:00.000000", expired.Job.ID,
	); err != nil {
		t.Fatal(err)
	}

	res, err := s.RunExpirationPass(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if !intsEqual(res.ExpiredJobIDs, []int{expired.Job.ID}) {
		t.Errorf("expired_job_ids = %v, want [%d]", res.ExpiredJobIDs, expired.Job.ID)
	}
	if res.ExpiredCount != 1 {
		t.Errorf("expired_count = %d, want 1", res.ExpiredCount)
	}
	if _, found, err := s.JobByID(ctx, expired.Job.ID); err != nil || found {
		t.Errorf("expired job still present (found=%v, err=%v)", found, err)
	}
	if _, found, err := s.JobByID(ctx, live.Job.ID); err != nil || !found {
		t.Errorf("live job missing (found=%v, err=%v)", found, err)
	}
}
