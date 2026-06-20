package realtime

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHubBroadcastDelivers(t *testing.T) {
	h := NewHub()
	ch := h.register()
	h.Broadcast(Event{Name: "status_update", Data: []byte(`{"schema_version":1}`)})
	select {
	case e := <-ch:
		if e.Name != "status_update" || string(e.Data) != `{"schema_version":1}` {
			t.Errorf("got %s / %s", e.Name, e.Data)
		}
	default:
		t.Fatal("no event delivered")
	}
}

func TestHubDropsSlowClientOnOverflow(t *testing.T) {
	h := NewHub()
	ch := h.register()

	// Fill the buffer exactly; the client survives.
	for i := 0; i < clientBufferDepth; i++ {
		h.Broadcast(Event{Name: "x", Data: []byte("y")})
	}
	if h.ClientCount() != 1 {
		t.Fatalf("client dropped before overflow, count = %d", h.ClientCount())
	}

	// One more overflows the buffer -> the whole client is dropped (and its channel closed).
	h.Broadcast(Event{Name: "x", Data: []byte("y")})
	if h.ClientCount() != 0 {
		t.Errorf("slow client not dropped on overflow, count = %d", h.ClientCount())
	}
	drained := 0
	for range ch { // closed channel -> drains the buffered events then ends
		drained++
	}
	if drained != clientBufferDepth {
		t.Errorf("drained %d buffered events, want %d", drained, clientBufferDepth)
	}
}

func TestServeEventsStreamsFramesAndHeartbeat(t *testing.T) {
	h := NewHub()
	h.heartbeat = 40 * time.Millisecond // white-box: short heartbeat for the test
	srv := httptest.NewServer(http.HandlerFunc(h.ServeEvents))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	frames := make(chan string, 128)
	go scanFrames(resp.Body, frames)

	if !eventually(func() bool { return h.ClientCount() == 1 }) {
		t.Fatal("client did not register")
	}
	h.Broadcast(Event{Name: "status_update", Data: []byte(`{"schema_version":1}`)})

	gotEvent, gotPing := false, false
	deadline := time.After(2 * time.Second)
	for !gotEvent || !gotPing {
		select {
		case f := <-frames:
			if strings.Contains(f, "event: status_update") && strings.Contains(f, `data: {"schema_version":1}`) {
				gotEvent = true
			}
			if strings.Contains(f, ": ping") {
				gotPing = true
			}
		case <-deadline:
			t.Fatalf("timeout waiting for frames; event=%v ping=%v", gotEvent, gotPing)
		}
	}
}

// scanFrames splits an SSE stream into frames (blocks separated by a blank line).
func scanFrames(r io.Reader, out chan<- string) {
	sc := bufio.NewScanner(r)
	var buf []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(buf) > 0 {
				out <- strings.Join(buf, "\n")
				buf = nil
			}
			continue
		}
		buf = append(buf, line)
	}
}

func eventually(cond func() bool) bool {
	for i := 0; i < 200; i++ {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
