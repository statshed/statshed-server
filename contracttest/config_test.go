//go:build contract

// Ported from contract/test_config.py — GET/PUT /api/config (global) and
// GET/PUT /api/groups/<name>/config (per-group overrides + effective_* values).
package contracttest

import (
	"math"
	"testing"
	"time"
)

func TestGetConfigDefaults(t *testing.T) {
	begin(t, "default")
	status, data := getJSON(t, "/api/config")
	mustStatus(t, status, 200)
	mustEqNum(t, data, "progress_timeout_minutes", 5)
	mustEqNum(t, data, "staleness_timeout_hours", 24)
	mustEqNum(t, data, "expiration_timeout_hours", 24)
}

func TestUpdateConfig(t *testing.T) {
	begin(t, "default")
	status, data := putJSON(t, "/api/config", map[string]any{
		"progress_timeout_minutes": 10, "staleness_timeout_hours": 48,
	})
	mustStatus(t, status, 200)
	mustEqNum(t, data, "progress_timeout_minutes", 10)
	mustEqNum(t, data, "staleness_timeout_hours", 48)
}

func TestUpdateConfigInvalidValue(t *testing.T) {
	begin(t, "default")
	// progress_timeout_minutes valid range is 1-10080; 0 is below it.
	status, data := putJSON(t, "/api/config", map[string]any{"progress_timeout_minutes": 0})
	mustStatus(t, status, 400)
	mustEqStr(t, data, "error", "validation_error")
}

func TestUpdateConfigPartial(t *testing.T) {
	begin(t, "default")
	status, data := putJSON(t, "/api/config", map[string]any{"progress_timeout_minutes": 15})
	mustStatus(t, status, 200)
	mustEqNum(t, data, "progress_timeout_minutes", 15)
	mustEqNum(t, data, "staleness_timeout_hours", 24)
}

func TestGetGroupConfig(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	status, data := getJSON(t, "/api/groups/backups/config")
	mustStatus(t, status, 200)
	mustEqStr(t, data, "group_name", "backups")
	mustEqStr(t, data, "group", "backups") // legacy alias
	mustNull(t, data, "progress_timeout_minutes")
	mustEqBool(t, data, "staleness_enabled", false)
	mustNull(t, data, "staleness_timeout_hours")
	mustNull(t, data, "expiration_timeout_hours")
	mustEqNum(t, data, "effective_progress_timeout_minutes", 5)
	mustEqNum(t, data, "effective_staleness_timeout_hours", 24)
	mustEqNum(t, data, "effective_expiration_timeout_hours", 24)
}

func TestUpdateGroupConfig(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	status, data := putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 72})
	mustStatus(t, status, 200)
	mustEqNum(t, data, "staleness_timeout_hours", 72)
	mustEqNum(t, data, "effective_staleness_timeout_hours", 72)
}

func TestUpdateGroupConfigNullReverts(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	// Set override.
	putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 72})

	// Revert to null.
	status, data := putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": nil})
	mustStatus(t, status, 200)
	mustNull(t, data, "staleness_timeout_hours")
}

func TestUpdateGroupConfigStalenessValidation(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	// First enable staleness and set expiration to 24 hours, staleness to 12.
	status, _ := putJSON(t, "/api/groups/backups/config", map[string]any{
		"staleness_enabled":        true,
		"expiration_timeout_hours": 24,
		"staleness_timeout_hours":  12,
	})
	mustStatus(t, status, 200) // Ensure setup succeeded.

	// staleness == expiration -> fail.
	status, data := putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 24})
	mustStatus(t, status, 400)
	mustEqStr(t, data, "error", "validation_error")
	fields := gmap(t, data, "fields")
	if !has(fields, "staleness_timeout_hours") {
		t.Errorf("expected 'staleness_timeout_hours' in fields, got %v", fields)
	}

	// staleness > expiration -> also fail.
	status, _ = putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 48})
	mustStatus(t, status, 400)

	// Valid: staleness < expiration.
	status, _ = putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 12})
	mustStatus(t, status, 200)
}

func TestUpdateGroupConfigStalenessValidationSkippedWhenDisabled(t *testing.T) {
	begin(t, "default")
	postJSON(t, "/api/status", map[string]any{"group": "backups", "job": "job1", "status": "success"})

	// Set expiration to 24 hours, staleness disabled.
	status, _ := putJSON(t, "/api/groups/backups/config", map[string]any{
		"expiration_timeout_hours": 24, "staleness_enabled": false,
	})
	mustStatus(t, status, 200) // Ensure setup succeeded.

	// staleness >= expiration is fine while staleness is disabled.
	status, _ = putJSON(t, "/api/groups/backups/config", map[string]any{"staleness_timeout_hours": 48})
	mustStatus(t, status, 200)
}

func TestUpdateConfigBooleanRejected(t *testing.T) {
	begin(t, "default")
	status, data := putJSON(t, "/api/config", map[string]any{"progress_timeout_minutes": true})
	mustStatus(t, status, 400)
	mustEqStr(t, data, "error", "validation_error")
}

func TestUpdateConfigJSONArrayRejected(t *testing.T) {
	begin(t, "default")
	// A top-level JSON array body, content-type application/json.
	status, data := putJSON(t, "/api/config", []any{10, 20})
	mustStatus(t, status, 400)
	mustEqStr(t, data, "error", "bad_request")
}

func TestStatusJSONArrayRejected(t *testing.T) {
	begin(t, "default")
	status, data := postJSON(t, "/api/status", []any{"invalid"})
	mustStatus(t, status, 400)
	mustEqStr(t, data, "error", "bad_request")
}

func TestGetGroupConfigNotFound(t *testing.T) {
	begin(t, "default")
	status, _ := getJSON(t, "/api/groups/nonexistent/config")
	mustStatus(t, status, 404)
}

func TestUpdateConfigExpirationCascadesToNonOverrideGroups(t *testing.T) {
	begin(t, "default")
	// Group with no expiration override -> follows the global default.
	postJSON(t, "/api/status", map[string]any{"group": "noverride", "job": "a", "status": "success"})
	// Group with an explicit 10-hour expiration override.
	postJSON(t, "/api/status", map[string]any{"group": "override", "job": "b", "status": "success"})
	status, _ := putJSON(t, "/api/groups/override/config", map[string]any{"expiration_timeout_hours": 10})
	mustStatus(t, status, 200)

	// Change the global expiration from the 24h default to 48h.
	status, _ = putJSON(t, "/api/config", map[string]any{"expiration_timeout_hours": 48})
	mustStatus(t, status, 200)

	_, body := getJSON(t, "/api/jobs")
	jobsList := glist(t, body, "jobs")
	byName := map[string]map[string]any{}
	for i := range jobsList {
		jm := gelem(t, jobsList, i)
		byName[gstr(t, jm, "name")] = jm
	}
	jobA := byName["a"]
	jobB := byName["b"]

	const fmtTS = "2006-01-02T15:04:05Z"
	aUpdated, err := time.Parse(fmtTS, gstr(t, jobA, "updated_at"))
	if err != nil {
		t.Fatalf("parse a.updated_at: %v", err)
	}
	aExpires, err := time.Parse(fmtTS, gstr(t, jobA, "expires_at"))
	if err != nil {
		t.Fatalf("parse a.expires_at: %v", err)
	}
	bUpdated, err := time.Parse(fmtTS, gstr(t, jobB, "updated_at"))
	if err != nil {
		t.Fatalf("parse b.updated_at: %v", err)
	}
	bExpires, err := time.Parse(fmtTS, gstr(t, jobB, "expires_at"))
	if err != nil {
		t.Fatalf("parse b.expires_at: %v", err)
	}

	// Non-override group's job: expires_at refreshed to updated_at + 48h.
	if d := math.Abs(aExpires.Sub(aUpdated).Seconds() - 48*3600); d >= 2 {
		t.Errorf("job a: expires-updated = %.0fs, want ~%d (delta %.0f)", aExpires.Sub(aUpdated).Seconds(), 48*3600, d)
	}
	// Override group's job: untouched, still updated_at + 10h.
	if d := math.Abs(bExpires.Sub(bUpdated).Seconds() - 10*3600); d >= 2 {
		t.Errorf("job b: expires-updated = %.0fs, want ~%d (delta %.0f)", bExpires.Sub(bUpdated).Seconds(), 10*3600, d)
	}
}

// Ported from contract/test_config.py::TestCliConfigManagement.
func TestCliConfigManagement(t *testing.T) {
	begin(t, "default")
	// Get current config (verify it's readable).
	status, _ := getJSON(t, "/api/config")
	mustStatus(t, status, 200)

	// Update config.
	status, data := putJSON(t, "/api/config", map[string]any{
		"progress_timeout_minutes": 10, "staleness_timeout_hours": 48,
	})
	mustStatus(t, status, 200)
	mustEqNum(t, data, "progress_timeout_minutes", 10)
	mustEqNum(t, data, "staleness_timeout_hours", 48)

	// Verify config was persisted.
	_, data = getJSON(t, "/api/config")
	mustEqNum(t, data, "progress_timeout_minutes", 10)
}
