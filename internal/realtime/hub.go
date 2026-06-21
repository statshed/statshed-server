// Package realtime provides the Server-Sent Events hub that replaces the Python server's
// Socket.IO broadcasts (spec.md §7). One hub fans every event out to all connected
// EventSource clients.
package realtime

import (
	"sync"
	"time"
)

// clientBufferDepth is how many events a slow client may fall behind before the hub drops
// the whole client (S6). A dropped client's EventSource reconnects and refetches, so it
// resyncs — far better than silently losing one event on a live client.
const clientBufferDepth = 64

// defaultHeartbeat is the SSE comment-ping interval that keeps proxies from idling the
// connection out (~25s, comfortably under typical 30–60s proxy idle timeouts).
const defaultHeartbeat = 25 * time.Second

// Event is one SSE frame: Name is the event: type, Data is the already-marshaled JSON
// payload (including schema_version).
type Event struct {
	Name string
	Data []byte
}

// Hub fans events out to all connected SSE clients.
type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
	closed  bool

	// heartbeat is the ping interval; 0 means defaultHeartbeat. Settable in tests.
	heartbeat time.Duration
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
}

// Broadcast delivers e to every connected client. A client whose buffer is full is dropped
// (unregistered + channel closed) so its EventSource reconnects and resyncs.
func (h *Hub) Broadcast(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- e:
		default:
			delete(h.clients, ch)
			close(ch)
		}
	}
}

// register adds a client and returns its event channel. After the hub is closed it returns
// an already-closed channel, so a late-arriving client's handler exits immediately.
func (h *Hub) register() chan Event {
	ch := make(chan Event, clientBufferDepth)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch
	}
	h.clients[ch] = struct{}{}
	return ch
}

// Close disconnects every client (closing their channels so the SSE handlers return) and
// marks the hub closed. Called on server shutdown so graceful shutdown does not block on the
// long-lived SSE handlers, and so each connection closes cleanly — letting any proxy
// propagate the close and the clients reconnect. Idempotent.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.clients {
		delete(h.clients, ch)
		close(ch)
	}
}

// unregister removes a client and closes its channel. Idempotent: a client already dropped
// by Broadcast (overflow) is a no-op, so the channel is never double-closed.
func (h *Hub) unregister(ch chan Event) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// ClientCount returns the number of connected clients (used by tests).
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) heartbeatInterval() time.Duration {
	if h.heartbeat > 0 {
		return h.heartbeat
	}
	return defaultHeartbeat
}
