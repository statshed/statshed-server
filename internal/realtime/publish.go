package realtime

import (
	"encoding/json"
	"log/slog"
)

// Publish marshals payload to JSON and broadcasts it as a named event. A marshal failure is
// logged and dropped (a single un-encodable event must never take down a handler).
func Publish(hub *Hub, name string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal SSE event", "event", name, "err", err)
		return
	}
	hub.Broadcast(Event{Name: name, Data: data})
}
