package store

import (
	"context"
	"testing"
	"time"
)

// TestMarkAckedRejectsRecoveredJob verifies the status gate (I3): a job that recovered to a
// healthy status must NOT be ackable, closing the TOCTOU where a recovery between the handler's
// status read and MarkAcked would otherwise re-ack a now-healthy job.
func TestMarkAckedRejectsRecoveredJob(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	// A success job has acked=0 (UpsertJob clears the ack on a healthy status).
	r, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "success"}, now)
	if err != nil {
		t.Fatal(err)
	}
	acked, err := s.MarkAcked(ctx, r.Job.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if acked {
		t.Error("MarkAcked on a success job = true, want false (healthy jobs are not ackable)")
	}
	job, found, err := s.JobByID(ctx, r.Job.ID)
	if err != nil || !found {
		t.Fatalf("JobByID found=%v err=%v", found, err)
	}
	if job.Acked {
		t.Error("success job became acked=true, want false")
	}
}

// TestMarkAckedAcksUnhealthyJob confirms the gate still acks a genuinely unhealthy job.
func TestMarkAckedAcksUnhealthyJob(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	r, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "error"}, now)
	if err != nil {
		t.Fatal(err)
	}
	acked, err := s.MarkAcked(ctx, r.Job.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if !acked {
		t.Fatal("MarkAcked on an error job = false, want true")
	}
	if job, _, _ := s.JobByID(ctx, r.Job.ID); !job.Acked {
		t.Error("error job not acked")
	}
}
