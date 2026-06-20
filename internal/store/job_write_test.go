package store

import (
	"context"
	"testing"
	"time"
)

func TestUpsertJobCreateThenUpdate(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	msg := "hello"
	r1, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "error", Message: &msg}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !r1.GroupCreated {
		t.Error("GroupCreated = false, want true on first insert")
	}
	if r1.PreviousStatus != nil {
		t.Errorf("PreviousStatus = %q, want nil on create", *r1.PreviousStatus)
	}
	if r1.Job.ID != 1 || r1.Job.Status != "error" {
		t.Errorf("job = %+v", r1.Job)
	}
	// expires_at = now + 24h default (insert path).
	if r1.Job.ExpiresAt == nil || !r1.Job.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Errorf("expires_at = %v, want %v", r1.Job.ExpiresAt, now.Add(24*time.Hour))
	}

	now2 := now.Add(time.Hour)
	r2, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "success"}, now2)
	if err != nil {
		t.Fatal(err)
	}
	if r2.GroupCreated {
		t.Error("GroupCreated = true, want false on update")
	}
	if r2.PreviousStatus == nil || *r2.PreviousStatus != "error" {
		t.Errorf("PreviousStatus = %v, want error", r2.PreviousStatus)
	}
	if r2.Job.ID != 1 || r2.Job.Status != "success" {
		t.Errorf("job after update = %+v, want id 1 / success", r2.Job)
	}
	if r2.Job.Message != nil {
		t.Errorf("message = %q, want nil (omitted -> NULL)", *r2.Job.Message)
	}
}

func TestUpsertJobClearsAckOnRecovery(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if _, err := s.UpsertJob(ctx, UpsertParams{GroupName: "g", JobName: "j", Status: "error"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Write().Exec(
		"UPDATE jobs SET acked = 1, acked_at = ? WHERE name = 'j'", formatStored(now),
	); err != nil {
		t.Fatal(err)
	}

	r, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "success"}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if r.Job.Acked {
		t.Error("acked = true, want false after recovery to success")
	}
	if r.Job.AckedAt != nil {
		t.Error("acked_at not nil after ack clear")
	}
}

func TestUpsertJobLogReplaceAndPreserve(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	r, err := s.UpsertJob(ctx, UpsertParams{
		GroupName: "g", JobName: "j", Status: "success",
		Log: &LogInput{Content: "a\nb\n", LineCount: 2, Truncated: false},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Job.HasLog || r.Job.LogLineCount == nil || *r.Job.LogLineCount != 2 {
		t.Errorf("after log insert: %+v", r.Job)
	}

	// An update WITHOUT a log leaves the previous log intact.
	r2, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "error"}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Job.HasLog || r2.Job.LogLineCount == nil || *r2.Job.LogLineCount != 2 {
		t.Errorf("log not preserved on logless update: %+v", r2.Job)
	}
}
