package store

import (
	"testing"
	"time"
)

var sampleTime = time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC) // .123456 µs

func TestFormatStored(t *testing.T) {
	if got, want := formatStored(sampleTime), "2026-01-02 03:04:05.123456"; got != want {
		t.Errorf("formatStored = %q, want %q", got, want)
	}
}

func TestFormatAPITruncatesToWholeSecond(t *testing.T) {
	if got, want := formatAPI(sampleTime), "2026-01-02T03:04:05Z"; got != want {
		t.Errorf("formatAPI = %q, want %q", got, want)
	}
}

func TestFormatForcesUTC(t *testing.T) {
	// A non-UTC instant must still render with its UTC wall-clock + Z, not the local time.
	loc := time.FixedZone("X", 5*3600)
	ts := time.Date(2026, 1, 2, 8, 4, 5, 0, loc) // == 03:04:05 UTC
	if got, want := formatAPI(ts), "2026-01-02T03:04:05Z"; got != want {
		t.Errorf("formatAPI(non-UTC) = %q, want %q", got, want)
	}
}

func TestParseStoredBothForms(t *testing.T) {
	micro, err := parseStored("2026-01-02 03:04:05.123456")
	if err != nil {
		t.Fatalf("parseStored(micro): %v", err)
	}
	if !micro.Equal(sampleTime) {
		t.Errorf("parseStored(micro) = %v, want %v", micro, sampleTime)
	}

	// Whole-second form (written by the config cascade's SQLite datetime()).
	whole, err := parseStored("2026-01-02 03:04:05")
	if err != nil {
		t.Fatalf("parseStored(whole): %v", err)
	}
	if !whole.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("parseStored(whole) = %v", whole)
	}
}

func TestStoredRoundTrip(t *testing.T) {
	parsed, err := parseStored(formatStored(sampleTime))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if !parsed.Equal(sampleTime) {
		t.Errorf("round-trip = %v, want %v", parsed, sampleTime)
	}
}

func TestExpiresInsertPreservesMicroseconds(t *testing.T) {
	exp := expiresInsert(sampleTime, 24)
	if want := sampleTime.Add(24 * time.Hour); !exp.Equal(want) {
		t.Errorf("expiresInsert = %v, want %v", exp, want)
	}
	// Stored form keeps the microseconds; API form truncates to whole-second.
	if got, want := formatStored(exp), "2026-01-03 03:04:05.123456"; got != want {
		t.Errorf("stored expires = %q, want %q", got, want)
	}
	if got, want := formatAPI(exp), "2026-01-03T03:04:05Z"; got != want {
		t.Errorf("api expires = %q, want %q", got, want)
	}
}

func TestStoredFormIsLexicallyOrdered(t *testing.T) {
	// Microsecond precision keeps ORDER BY updated_at stable for sub-second-apart writes.
	a := time.Date(2026, 1, 2, 3, 4, 5, 1000, time.UTC) // .000001
	b := time.Date(2026, 1, 2, 3, 4, 5, 2000, time.UTC) // .000002
	if formatStored(a) >= formatStored(b) {
		t.Errorf("stored form not lexically ordered: %q >= %q", formatStored(a), formatStored(b))
	}
}

func TestCascadeDatetimeTruncatesToWholeSecond(t *testing.T) {
	// The config cascade computes expires_at via SQLite datetime() through the driver.
	// Verify it truncates the microsecond input to a whole-second result (matching
	// Python's raw cascade UPDATEs), so the second expires_at path is observably
	// equivalent after the API's whole-second render.
	s := freshStore(t)
	var got string
	err := s.Read().QueryRow(
		"SELECT datetime(?, '+24 hours')", "2026-01-02 03:04:05.123456",
	).Scan(&got)
	if err != nil {
		t.Fatalf("datetime cascade query: %v", err)
	}
	if want := "2026-01-03 03:04:05"; got != want {
		t.Errorf("datetime cascade = %q, want %q (whole-second)", got, want)
	}
	// And it parses back cleanly + renders to the same whole-second API value as the
	// insert path would for an aligned instant.
	parsed, err := parseStored(got)
	if err != nil {
		t.Fatalf("parseStored(cascade): %v", err)
	}
	if api := formatAPI(parsed); api != "2026-01-03T03:04:05Z" {
		t.Errorf("cascade API render = %q", api)
	}
}
