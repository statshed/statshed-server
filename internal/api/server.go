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
// stream indefinitely (D15); request reads are still bounded by ReadHeaderTimeout/
// ReadTimeout, and idle keep-alive connections by IdleTimeout (S4).
func NewServer(cfg config.Config, handler http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
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
