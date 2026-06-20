package config

import "testing"

func TestParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		// SQLAlchemy form: 3 slashes = relative, 4 slashes = absolute (D12).
		{"relative", "sqlite:///statshed.db", "statshed.db", false},
		{"absolute compose path", "sqlite:////data/statshed.db", "/data/statshed.db", false},
		{"relative nested", "sqlite:///var/db/x.db", "var/db/x.db", false},
		{"postgres rejected", "postgresql://localhost/db", "", true},
		{"mysql rejected", "mysql://localhost/db", "", true},
		{"empty rejected", "", "", true},
		{"bare path rejected", "/data/statshed.db", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDatabaseURL(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDatabaseURL(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDatabaseURL(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseDatabaseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	// Clear every honored env var so we observe pure defaults.
	for _, k := range []string{
		"HOST", "PORT", "DATABASE_URL", "DEBUG", "CORS_ORIGINS", "LOG_UPLOAD_ENABLED",
		"MAX_LOG_LINES", "MAX_JOBS_PAGE_SIZE", "STATIC_DIR", "STATIC_DISABLED",
		"STATSHED_TEST_HOOKS",
	} {
		t.Setenv(k, "")
		// t.Setenv sets to ""; unset semantics are covered by treating "" as absent.
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 7828 {
		t.Errorf("Port = %d, want 7828", cfg.Port)
	}
	if cfg.DBPath != "statshed.db" {
		t.Errorf("DBPath = %q, want statshed.db", cfg.DBPath)
	}
	if cfg.Debug {
		t.Errorf("Debug = true, want false")
	}
	if !cfg.LogUploadEnabled {
		t.Errorf("LogUploadEnabled = false, want true (default)")
	}
	if cfg.MaxLogLines != 1000 {
		t.Errorf("MaxLogLines = %d, want 1000", cfg.MaxLogLines)
	}
	if cfg.MaxJobsPageSize != 500 {
		t.Errorf("MaxJobsPageSize = %d, want 500", cfg.MaxJobsPageSize)
	}
	if cfg.StaticDir != "./static" {
		t.Errorf("StaticDir = %q, want ./static", cfg.StaticDir)
	}
	if cfg.StaticDisabled {
		t.Errorf("StaticDisabled = true, want false")
	}
	if cfg.TestHooks {
		t.Errorf("TestHooks = true, want false")
	}
	wantOrigins := []string{
		"http://localhost:5173", "http://127.0.0.1:5173",
		"http://localhost:7827", "http://127.0.0.1:7827",
	}
	if len(cfg.CORSOrigins) != len(wantOrigins) {
		t.Fatalf("CORSOrigins = %v, want %v", cfg.CORSOrigins, wantOrigins)
	}
	for i, o := range wantOrigins {
		if cfg.CORSOrigins[i] != o {
			t.Errorf("CORSOrigins[%d] = %q, want %q", i, cfg.CORSOrigins[i], o)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("HOST", "0.0.0.0")
	t.Setenv("PORT", "9000")
	t.Setenv("DATABASE_URL", "sqlite:////data/statshed.db")
	t.Setenv("DEBUG", "1")
	t.Setenv("CORS_ORIGINS", "http://a.example:7827, http://b.example:7827 ")
	t.Setenv("LOG_UPLOAD_ENABLED", "false")
	t.Setenv("MAX_LOG_LINES", "10")
	t.Setenv("MAX_JOBS_PAGE_SIZE", "2")
	t.Setenv("STATIC_DIR", "/srv/spa")
	t.Setenv("STATIC_DISABLED", "1")
	t.Setenv("STATSHED_TEST_HOOKS", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "0.0.0.0" || cfg.Port != 9000 {
		t.Errorf("Host/Port = %q/%d", cfg.Host, cfg.Port)
	}
	if cfg.DBPath != "/data/statshed.db" {
		t.Errorf("DBPath = %q, want /data/statshed.db", cfg.DBPath)
	}
	if !cfg.Debug || cfg.LogUploadEnabled || !cfg.StaticDisabled || !cfg.TestHooks {
		t.Errorf("bool flags wrong: %+v", cfg)
	}
	if cfg.MaxLogLines != 10 || cfg.MaxJobsPageSize != 2 {
		t.Errorf("limits = %d/%d", cfg.MaxLogLines, cfg.MaxJobsPageSize)
	}
	if cfg.StaticDir != "/srv/spa" {
		t.Errorf("StaticDir = %q", cfg.StaticDir)
	}
	// CORS origins are comma-split and trimmed.
	want := []string{"http://a.example:7827", "http://b.example:7827"}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != want[0] || cfg.CORSOrigins[1] != want[1] {
		t.Errorf("CORSOrigins = %v, want %v", cfg.CORSOrigins, want)
	}
}

func TestLoadRejectsBadPort(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with bad PORT: want error, got nil")
	}
}

func TestLoadRejectsNonSQLite(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://localhost/db")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with postgres URL: want error, got nil")
	}
}
