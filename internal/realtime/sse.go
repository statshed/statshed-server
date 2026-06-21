package realtime

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// writeTimeout bounds a SINGLE SSE write. The server runs with no connection-level
// WriteTimeout so the long-lived stream survives across events (D15); this per-write
// deadline instead caps each individual write, so a stalled or dead client can never block
// the handler — and thus graceful shutdown — indefinitely (codex review). It is kept well
// under the server's shutdownGrace so a write in flight when shutdown begins still fails with
// headroom to spare.
const writeTimeout = 5 * time.Second

// ServeEvents streams Server-Sent Events to the client until it disconnects. It relies on
// the surrounding stack for the rest of the SSE policy: CORS headers (the /api subrouter's
// CORS middleware), gzip exclusion (text/event-stream is never compressed, M2/D15), and the
// absence of a server write timeout so the stream can run indefinitely (D15).
func (h *Hub) ServeEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // ask reverse proxies not to buffer the stream
	w.WriteHeader(http.StatusOK)

	// write sends one raw SSE chunk under a bounded per-write deadline, then flushes. The
	// deadline is best effort: SetWriteDeadline is a no-op on a ResponseWriter that cannot
	// expose its connection, in which case we are no worse off than before.
	write := func(chunk string) error {
		_ = rc.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := io.WriteString(w, chunk); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	// AIDEV-NOTE: Push an initial comment immediately. A proxy that buffers the response
	// until the first byte of BODY (the Vite dev proxy, and some production proxies) would
	// otherwise hold back the headers, so the client's EventSource onopen — which fires on
	// data, not just status — is delayed until the first event or the ~25s heartbeat. This
	// byte opens the stream promptly (and is ignored as an SSE comment).
	if err := write(": connected\n\n"); err != nil {
		return
	}

	ch := h.register()
	defer h.unregister(ch)

	ping := time.NewTicker(h.heartbeatInterval())
	defer ping.Stop()

	ctx := r.Context()
	for {
		// AIDEV-NOTE: Priority check — on hub shutdown (h.done closed) return at once,
		// BEFORE draining and writing any events still buffered on ch. A bare select could
		// otherwise pick the ready ch case, and writing to a slow/proxy-buffered client could
		// block with no recourse and stall graceful shutdown — exactly what Hub.Close (which
		// closes h.done) exists to prevent (codex review).
		select {
		case <-h.done:
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case <-h.done:
			return
		case e, open := <-ch:
			if !open {
				return // dropped by the hub on overflow -> the client reconnects and resyncs
			}
			if err := write(fmt.Sprintf("event: %s\ndata: %s\n\n", e.Name, e.Data)); err != nil {
				return
			}
		case <-ping.C:
			if err := write(": ping\n\n"); err != nil {
				return
			}
		}
	}
}
