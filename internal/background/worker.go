// Package background runs the periodic maintenance passes (timeout/stale transitions and
// expiration deletes) that mirror the Python APScheduler job.
package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/statshed/statshed-server/internal/store"
)

// TickInterval is the production scheduler period (the Python job runs every 60s).
const TickInterval = 60 * time.Second

// Worker drives the maintenance passes on a fixed interval.
type Worker struct {
	store    *store.Store
	interval time.Duration
}

// New builds a worker ticking every TickInterval.
func New(st *store.Store) *Worker {
	return &Worker{store: st, interval: TickInterval}
}

// Run ticks until ctx is cancelled. The first pass runs after the first interval elapses
// (the scheduler does not fire immediately at startup).
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	slog.Info("background worker started", "interval", w.interval)
	for {
		select {
		case <-ctx.Done():
			slog.Info("background worker stopped")
			return
		case <-ticker.C:
			w.Tick(ctx)
		}
	}
}

// Tick runs one timeout/stale pass then one expiration pass. Errors are logged, not fatal —
// a transient DB error should not kill the scheduler.
func (w *Worker) Tick(ctx context.Context) {
	now := time.Now().UTC()
	if _, err := w.store.RunTimeoutPass(ctx, now); err != nil {
		slog.Error("timeout pass", "err", err)
	}
	if _, err := w.store.RunExpirationPass(ctx, now); err != nil {
		slog.Error("expiration pass", "err", err)
	}
}
