package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/statshed/statshed-server/internal/config"
)

// shutdownGrace bounds how long graceful shutdown waits for in-flight REST requests.
const shutdownGrace = 10 * time.Second

// Server wraps http.Server with the StatShed timeout policy and graceful shutdown.
type Server struct {
	http *http.Server
}

// NewServer builds the server. WriteTimeout is intentionally 0 so GET /api/events can
// stream indefinitely (D15). ReadHeaderTimeout bounds header-slowloris; IdleTimeout bounds idle
// keep-alives (S4).
//
// ReadTimeout (I8) is a generous 5m, not 30s: it bounds the WHOLE request including the body, so
// a too-tight value aborts a legitimate slow (but <1MB) multipart log upload mid-transfer — the
// Python (gevent) server imposed no read deadline. 5m still bounds a stalled connection while
// allowing a ~3.4 KB/s floor for a 1MB body; the body size itself is capped by bodyLimit +
// MaxBytesReader (MAX_CONTENT_LENGTH).
func NewServer(cfg config.Config, handler http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Run serves until ctx is cancelled (SIGINT/SIGTERM), then drains in-flight requests for
// up to shutdownGrace. It returns nil on a clean shutdown.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}

// Addr returns the configured listen address.
func (s *Server) Addr() string { return s.http.Addr }
