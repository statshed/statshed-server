package store

import (
	"path/filepath"
	"testing"
)

func freshStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := Migrate(s.Write()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

func TestSchemaTablesAndIndexes(t *testing.T) {
	s := freshStore(t)

	for _, table := range []string{"groups", "jobs", "config"} {
		var name string
		err := s.Read().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}

	indexes := []string{
		"ix_jobs_status", "ix_jobs_updated_at", "ix_jobs_group_id",
		"ix_jobs_status_updated", "ix_jobs_acked", "ix_jobs_status_acked",
		"ix_jobs_expires_at",
	}
	for _, idx := range indexes {
		var name string
		err := s.Read().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q missing: %v", idx, err)
		}
	}
}

func TestPragmasApplied(t *testing.T) {
	s := freshStore(t)

	var fk int
	if err := s.Write().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ON)", fk)
	}

	var journal string
	if err := s.Write().QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want wal", journal)
	}

	var busy int
	if err := s.Write().QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if busy != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busy)
	}
}

func TestUniqueJobGroupName(t *testing.T) {
	s := freshStore(t)
	w := s.Write()

	if _, err := w.Exec("INSERT INTO groups (name, created_at) VALUES ('g', '2026-01-01 00:00:00.000000')"); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	insertJob := "INSERT INTO jobs (group_id, name, status, updated_at, created_at) " +
		"VALUES (1, 'j', 'success', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000')"
	if _, err := w.Exec(insertJob); err != nil {
		t.Fatalf("first job insert: %v", err)
	}
	// Second job with the same (group_id, name) must violate uq_job_group_name.
	if _, err := w.Exec(insertJob); err == nil {
		t.Fatal("duplicate (group_id, name) insert: want unique-constraint error, got nil")
	}
}

func TestForeignKeyCascadeDeletesJobs(t *testing.T) {
	s := freshStore(t)
	w := s.Write()
	if _, err := w.Exec("INSERT INTO groups (name, created_at) VALUES ('g', '2026-01-01 00:00:00.000000')"); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if _, err := w.Exec("INSERT INTO jobs (group_id, name, status, updated_at, created_at) VALUES (1, 'j', 'success', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000')"); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if _, err := w.Exec("DELETE FROM groups WHERE id = 1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	var n int
	if err := w.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&n); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if n != 0 {
		t.Errorf("jobs after group delete = %d, want 0 (ON DELETE CASCADE + foreign_keys=ON)", n)
	}
}

func TestMigrateFailsFastOnNonEmptyDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Pre-create a conflicting 'groups' table (mimics a foreign/existing DB).
	if _, err := s.Write().Exec("CREATE TABLE groups (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("pre-create table: %v", err)
	}
	// The plain CREATE TABLE in the migration must error (no IF NOT EXISTS), not panic.
	if err := Migrate(s.Write()); err == nil {
		t.Fatal("Migrate on a DB with an existing 'groups' table: want error, got nil")
	}
}

func TestIDsResetAfterDelete(t *testing.T) {
	// No AUTOINCREMENT: after deleting all rows, the next rowid is 1 again.
	s := freshStore(t)
	w := s.Write()

	insert := "INSERT INTO groups (name, created_at) VALUES (?, '2026-01-01 00:00:00.000000')"
	res, err := w.Exec(insert, "first")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id, _ := res.LastInsertId(); id != 1 {
		t.Fatalf("first id = %d, want 1", id)
	}
	if _, err := w.Exec("DELETE FROM groups"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	res, err = w.Exec(insert, "second")
	if err != nil {
		t.Fatalf("re-insert: %v", err)
	}
	if id, _ := res.LastInsertId(); id != 1 {
		t.Errorf("id after delete = %d, want 1 (no AUTOINCREMENT)", id)
	}
}
