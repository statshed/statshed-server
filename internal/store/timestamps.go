package store

import (
	"fmt"
	"time"
)

// Timestamp formats (D13). App-set timestamps are stored as microsecond UTC text so
// `ORDER BY updated_at DESC` stays stable across rapid writes (lexically sortable,
// matching Python). The API renders whole-second UTC.
const (
	storedLayout = "2006-01-02 15:04:05.000000" // 'YYYY-MM-DD HH:MM:SS.ffffff'
	apiLayout    = "2006-01-02T15:04:05Z"       // 'YYYY-MM-DDTHH:MM:SSZ'
	// parseStored uses the whole-second layout; Go's time.Parse additionally accepts an
	// optional trailing fractional second, so this parses BOTH the microsecond form and
	// the whole-second form the config cascade writes via SQLite datetime().
	parseLayout = "2006-01-02 15:04:05"
)

// formatStored renders t as the stored timestamp ('YYYY-MM-DD HH:MM:SS.ffffff', UTC).
func formatStored(t time.Time) string {
	return t.UTC().Format(storedLayout)
}

// formatAPI renders t as the API timestamp ('YYYY-MM-DDTHH:MM:SSZ', UTC, whole-second).
func formatAPI(t time.Time) string {
	return t.UTC().Format(apiLayout)
}

// FormatAPITime exposes the whole-second API/event timestamp format to other packages (the
// SSE event payloads' top-level "timestamp" uses the same render as the API).
func FormatAPITime(t time.Time) string { return formatAPI(t) }

// parseStored parses a stored timestamp (either microsecond or whole-second form), UTC.
func parseStored(s string) (time.Time, error) {
	t, err := time.Parse(parseLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("unparseable stored timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

// expiresInsert computes expires_at on the insert/update path: updated + hours, in Go at
// microsecond precision, mirroring Python's compute_expires_at = updated_at + timedelta
// (app.py:154). The config-change cascade uses a different path — SQLite
// datetime(updated_at,'+N hours') through the driver (whole-second) — applied in the
// store's UPDATE so the two stay observably equivalent after the API's whole-second render.
func expiresInsert(updated time.Time, hours int) time.Time {
	return updated.Add(time.Duration(hours) * time.Hour)
}
