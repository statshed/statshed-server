// Command statshed-server is the StatShed status-dashboard server: it serves the REST API
// under /api, the Server-Sent Events stream at /api/events, and the embedded React SPA at
// /, backed by a single SQLite database.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/statshed/statshed-server/internal/api"
	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/store"
)

// version is the build version, injected at link time via -ldflags "-X main.version=<tag>"
// (S7). It defaults to "dev" for un-stamped local builds.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	healthcheck := flag.Bool("healthcheck", false,
		"probe the local /api/health endpoint; exit 0 if healthy, 1 otherwise")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	if *healthcheck {
		os.Exit(runHealthcheck(cfg))
	}

	os.Exit(run(cfg))
}

// run configures logging, opens + migrates the store (fail-fast on a non-empty DB), and
// serves until a signal arrives. It returns the process exit code.
func run(cfg config.Config) int {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	// Fresh-DB-only (C1): a non-empty or incompatible DB makes the plain CREATE statements
	// fail; we log clearly and exit non-zero rather than mutate an existing DB.
	if err := store.Migrate(st.Write()); err != nil {
		slog.Error("migrate database (fresh-DB-only; an existing/incompatible DB is rejected)", "err", err)
		return 1
	}

	srv := api.NewServer(cfg, api.NewRouter(cfg, st))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("statshed-server starting", "version", version, "addr", srv.Addr())
	if err := srv.Run(ctx); err != nil {
		slog.Error("server error", "err", err)
		return 1
	}
	slog.Info("statshed-server stopped")
	return 0
}

func runHealthcheck(cfg config.Config) int {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthcheckURL(cfg))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck: request failed:", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}

// healthcheckURL builds the loopback health URL. The container binds the wildcard, so the
// probe targets loopback: 127.0.0.1 for empty/0.0.0.0, ::1 for :: (D16). HEALTHCHECK_URL
// overrides the whole URL.
func healthcheckURL(cfg config.Config) string {
	if u := os.Getenv("HEALTHCHECK_URL"); u != "" {
		return u
	}
	host := cfg.Host
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(cfg.Port)) + "/api/health"
}
