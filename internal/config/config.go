// Package config loads the StatShed server configuration from the environment,
// preserving the env-var surface of the Python server (behavioral-map §8).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the resolved server configuration.
type Config struct {
	Host             string
	Port             int
	DBPath           string // filesystem path parsed from DATABASE_URL
	Debug            bool
	CORSOrigins      []string
	LogUploadEnabled bool
	MaxLogLines      int
	MaxJobsPageSize  int
	StaticDir        string
	StaticDisabled   bool
	TestHooks        bool
}

// Hardcoded limits and defaults (behavioral-map §8); not env-overridable.
const (
	DefaultProgressTimeoutMinutes = 5
	DefaultStalenessTimeoutHours  = 24
	DefaultExpirationTimeoutHours = 24

	MinProgressTimeoutMinutes = 1
	MaxProgressTimeoutMinutes = 10080 // 7 days
	MinStalenessTimeoutHours  = 1
	MaxStalenessTimeoutHours  = 8760 // 1 year
	MinExpirationTimeoutHours = 1
	MaxExpirationTimeoutHours = 8760 // 1 year

	MaxGroupNameLength = 255
	MaxJobNameLength   = 255
	MaxMessageLength   = 4096
	MaxContentLength   = 1 << 20 // 1 MB (413 over this)

	// SSEHeartbeatSeconds mirrors the old Socket.IO ping_interval (spec.md §7.2).
	SSEHeartbeatSeconds = 25
)

var defaultCORSOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:7827",
	"http://127.0.0.1:7827",
}

// Load reads the configuration from the process environment, applying defaults.
// It returns an error for an unparseable PORT/limit or a non-sqlite DATABASE_URL.
func Load() (Config, error) {
	dbPath, err := ParseDatabaseURL(getenv("DATABASE_URL", "sqlite:///statshed.db"))
	if err != nil {
		return Config{}, err
	}
	port, err := atoiEnv("PORT", "7828")
	if err != nil {
		return Config{}, err
	}
	maxLogLines, err := atoiEnv("MAX_LOG_LINES", "1000")
	if err != nil {
		return Config{}, err
	}
	maxJobsPageSize, err := atoiEnv("MAX_JOBS_PAGE_SIZE", "500")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Host:   getenv("HOST", "127.0.0.1"),
		Port:   port,
		DBPath: dbPath,
		Debug:  truthy(os.Getenv("DEBUG")),
		// CORS_ORIGINS is a comma-separated allowlist; empty/unset -> the localhost defaults.
		CORSOrigins: corsOrigins(os.Getenv("CORS_ORIGINS")),
		// LOG_UPLOAD_ENABLED defaults true; only an explicit false-y value disables it.
		LogUploadEnabled: !falsey(os.Getenv("LOG_UPLOAD_ENABLED")),
		MaxLogLines:      maxLogLines,
		MaxJobsPageSize:  maxJobsPageSize,
		StaticDir:        getenv("STATIC_DIR", "./static"),
		StaticDisabled:   truthy(os.Getenv("STATIC_DISABLED")),
		TestHooks:        truthy(os.Getenv("STATSHED_TEST_HOOKS")),
	}, nil
}

// ParseDatabaseURL converts a SQLAlchemy-style sqlite URL to a filesystem path (D12).
//
//	sqlite:///statshed.db    -> "statshed.db"      (3 slashes = relative)
//	sqlite:////data/x.db     -> "/data/x.db"       (4 slashes = absolute)
//
// Non-sqlite URLs are rejected: the Go server targets SQLite only (spec.md §13).
func ParseDatabaseURL(u string) (string, error) {
	const prefix = "sqlite://"
	if !strings.HasPrefix(u, prefix) {
		return "", fmt.Errorf("unsupported DATABASE_URL %q (sqlite only)", u)
	}
	rest := strings.TrimPrefix(u, prefix) // "/statshed.db" or "//data/x.db"
	path := strings.TrimPrefix(rest, "/") // drop exactly one leading slash
	if path == "" {
		return "", fmt.Errorf("DATABASE_URL %q has no database path", u)
	}
	return path, nil
}

// getenv returns the env value for key, or def when unset/empty.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// atoiEnv parses an integer env var (def when unset/empty), wrapping parse errors.
func atoiEnv(key, def string) (int, error) {
	raw := getenv(key, def)
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return n, nil
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func falsey(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func corsOrigins(v string) []string {
	if strings.TrimSpace(v) == "" {
		out := make([]string, len(defaultCORSOrigins))
		copy(out, defaultCORSOrigins)
		return out
	}
	var origins []string
	for _, part := range strings.Split(v, ",") {
		if s := strings.TrimSpace(part); s != "" {
			origins = append(origins, s)
		}
	}
	return origins
}
