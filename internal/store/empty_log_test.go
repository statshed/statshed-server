package store

import (
	"context"
	"testing"
	"time"
)

// TestEmptyLogInsertNullMetadataUpdateZero covers I14: an empty (zero-byte) log file on INSERT
// stores empty content (so has_log is true) but NULL log_line_count / log_updated_at — matching
// Python's `if log_content` guard. The UPDATE path stores 0 / now (Go already matched Python
// there); this asserts both paths.
func TestEmptyLogInsertNullMetadataUpdateZero(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	empty := &LogInput{Content: "", LineCount: 0, Truncated: false}

	r, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "success", Log: empty}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Job.HasLog {
		t.Error("insert: HasLog = false, want true (empty content is stored, not NULL)")
	}
	if r.Job.LogLineCount != nil {
		t.Errorf("insert: LogLineCount = %d, want nil for an empty log", *r.Job.LogLineCount)
	}
	if r.Job.LogUpdatedAt != nil {
		t.Errorf("insert: LogUpdatedAt = %v, want nil for an empty log", r.Job.LogUpdatedAt)
	}

	now2 := now.Add(time.Hour)
	r2, err := s.UpsertJob(ctx,
		UpsertParams{GroupName: "g", JobName: "j", Status: "error", Log: empty}, now2)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Job.LogLineCount == nil || *r2.Job.LogLineCount != 0 {
		t.Errorf("update: LogLineCount = %v, want 0 (Python parity)", r2.Job.LogLineCount)
	}
	if r2.Job.LogUpdatedAt == nil {
		t.Error("update: LogUpdatedAt = nil, want non-null")
	}
}
