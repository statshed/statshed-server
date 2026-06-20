package realtime

import (
	"fmt"
	"net/http"
	"time"
)

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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // ask reverse proxies not to buffer the stream
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.register()
	defer h.unregister(ch)

	ping := time.NewTicker(h.heartbeatInterval())
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, open := <-ch:
			if !open {
				return // dropped by the hub on overflow -> the client reconnects and resyncs
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, e.Data); err != nil {
				return
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
