package store

import (
	"context"
	"testing"
	"time"
)

func TestConfigDefaultsAndSet(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()

	cv, err := s.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cv.ProgressTimeoutMinutes != 5 || cv.StalenessTimeoutHours != 24 || cv.ExpirationTimeoutHours != 24 {
		t.Errorf("defaults = %+v, want 5/24/24", cv)
	}

	if err := s.SetConfigValue(ctx, "progress_timeout_minutes", 10); err != nil {
		t.Fatal(err)
	}
	// Upsert overwrites the existing row.
	if err := s.SetConfigValue(ctx, "progress_timeout_minutes", 15); err != nil {
		t.Fatal(err)
	}
	cv, _ = s.Config(ctx)
	if cv.ProgressTimeoutMinutes != 15 {
		t.Errorf("progress = %d, want 15", cv.ProgressTimeoutMinutes)
	}
}

func TestCascadeGlobalExpiration(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := s.UpsertJob(ctx, UpsertParams{GroupName: "noverride", JobName: "a", Status: "success"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertJob(ctx, UpsertParams{GroupName: "override", JobName: "b", Status: "success"}, now); err != nil {
		t.Fatal(err)
	}
	// Give the override group its own expiration (the group-config cascade is Task 3.6).
	if _, err := s.Write().Exec("UPDATE groups SET expiration_timeout_hours = 10 WHERE name = 'override'"); err != nil {
		t.Fatal(err)
	}

	if err := s.CascadeGlobalExpiration(ctx, 48); err != nil {
		t.Fatal(err)
	}

	jobs, err := s.ListJobs(ctx, JobFilter{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Job{}
	for _, j := range jobs.Jobs {
		byName[j.Name] = j
	}
	// The no-override group's job follows the new global expiration: updated_at + 48h.
	if got := byName["a"].ExpiresAt; got == nil || !got.Equal(now.Add(48*time.Hour)) {
		t.Errorf("no-override expires_at = %v, want %v", got, now.Add(48*time.Hour))
	}
	// The override group's job is untouched (still the insert-path now + 24h default).
	if got := byName["b"].ExpiresAt; got == nil || !got.Equal(now.Add(24*time.Hour)) {
		t.Errorf("override expires_at = %v, want %v (unchanged)", got, now.Add(24*time.Hour))
	}
}
