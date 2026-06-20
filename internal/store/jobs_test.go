package store

import (
	"context"
	"database/sql"
	"testing"
)

func insertGroup(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO groups (name, created_at) VALUES (?, '2026-01-01 00:00:00.000000')",
		name,
	)
	if err != nil {
		t.Fatalf("insert group %q: %v", name, err)
	}
}

func insertJob(t *testing.T, db *sql.DB, group, name, status string, acked bool) {
	t.Helper()
	var gid int
	if err := db.QueryRow("SELECT id FROM groups WHERE name=?", group).Scan(&gid); err != nil {
		t.Fatalf("lookup group %q: %v", group, err)
	}
	a := 0
	if acked {
		a = 1
	}
	_, err := db.Exec(
		"INSERT INTO jobs (group_id, name, status, acked, updated_at, created_at) "+
			"VALUES (?, ?, ?, ?, '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000')",
		gid, name, status, a,
	)
	if err != nil {
		t.Fatalf("insert job %q: %v", name, err)
	}
}

func TestHealthEmpty(t *testing.T) {
	s := freshStore(t)
	h, err := s.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != "empty" || h.TotalJobs != 0 {
		t.Errorf("health = %+v, want empty/0", h)
	}
	for _, st := range ValidStatuses {
		if h.ByStatus[st] != 0 {
			t.Errorf("by_status[%s] = %d, want 0", st, h.ByStatus[st])
		}
	}
}

func TestHealthCountsAndPrecedence(t *testing.T) {
	s := freshStore(t)
	w := s.Write()
	insertGroup(t, w, "g")
	insertJob(t, w, "g", "s1", "success", false)
	insertJob(t, w, "g", "p1", "progress", false)
	insertJob(t, w, "g", "e1", "error", false)  // unacked unhealthy
	insertJob(t, w, "g", "t1", "timeout", true) // acked -> excluded from unhealthy

	h, err := s.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.TotalJobs != 4 {
		t.Errorf("total = %d, want 4", h.TotalJobs)
	}
	if h.Healthy != 1 {
		t.Errorf("healthy = %d, want 1 (success count)", h.Healthy)
	}
	if h.Unhealthy != 1 {
		t.Errorf("unhealthy = %d, want 1 (error; acked timeout excluded)", h.Unhealthy)
	}
	if h.Acked != 1 {
		t.Errorf("acked = %d, want 1", h.Acked)
	}
	if h.InProgress != 1 {
		t.Errorf("in_progress = %d, want 1", h.InProgress)
	}
	// by_status holds RAW counts including the acked timeout.
	if h.ByStatus["timeout"] != 1 {
		t.Errorf("by_status[timeout] = %d, want 1 (raw, includes acked)", h.ByStatus["timeout"])
	}
	if h.Status != "unhealthy" {
		t.Errorf("status = %q, want unhealthy", h.Status)
	}
}

func TestHealthAllUnhealthyAckedIsHealthy(t *testing.T) {
	s := freshStore(t)
	w := s.Write()
	insertGroup(t, w, "g")
	insertJob(t, w, "g", "e1", "error", true) // acked

	h, err := s.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// unhealthy excludes the acked error -> 0; total 1 (not empty); no progress -> "healthy".
	if h.Status != "healthy" {
		t.Errorf("status = %q, want healthy (only an acked error)", h.Status)
	}
	if h.Unhealthy != 0 || h.Acked != 1 {
		t.Errorf("unhealthy=%d acked=%d, want 0/1", h.Unhealthy, h.Acked)
	}
}

func TestHealthInProgressPrecedence(t *testing.T) {
	s := freshStore(t)
	w := s.Write()
	insertGroup(t, w, "g")
	insertJob(t, w, "g", "p1", "progress", false)
	insertJob(t, w, "g", "s1", "success", false)
	h, _ := s.Health(context.Background())
	// no unhealthy, has progress -> "in_progress" beats "healthy".
	if h.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", h.Status)
	}
}
