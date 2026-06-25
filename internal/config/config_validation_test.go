package config

import "testing"

// TestLoadRejectsNonPositiveLimits verifies I6: a non-positive MAX_LOG_LINES or
// MAX_JOBS_PAGE_SIZE fails fast at startup, rather than later panicking in processLog
// (negative slice index) or silently disabling paging (SQLite LIMIT -1).
func TestLoadRejectsNonPositiveLimits(t *testing.T) {
	cases := []struct{ key, val string }{
		{"MAX_LOG_LINES", "-1"},
		{"MAX_LOG_LINES", "0"},
		{"MAX_JOBS_PAGE_SIZE", "-1"},
		{"MAX_JOBS_PAGE_SIZE", "0"},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.val, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "sqlite:///test.db")
			t.Setenv(c.key, c.val)
			if _, err := Load(); err == nil {
				t.Errorf("Load() with %s=%s = nil error, want a config error", c.key, c.val)
			}
		})
	}
}
