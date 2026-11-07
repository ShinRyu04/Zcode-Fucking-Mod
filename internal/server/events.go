package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/netmon"
	"github.com/azrinaldi/zcode-bridge/internal/usage"
)

// eventSnapshot is one SSE frame. Usage and net arrive every tick; health is
// throttled because resolving credentials is the expensive part.
type eventSnapshot struct {
	At     string          `json:"at"`
	Usage  usageResponse   `json:"usage"`
	Net    netmon.Snapshot `json:"net"`
	Health map[string]any  `json:"health,omitempty"`
}

// eventUsage trims the recent list: the console only paints the last few
// requests, and the full history is available from /v1/usage on demand.
func eventUsage() usageResponse {
	snap := usage.Current()
	if len(snap.Recent) > 60 {
		snap.Recent = snap.Recent[:60]
	}
	return usageResponse{Snapshot: snap, Desktop: currentDesktop()}
}

// handleEvents streams the console snapshot over Server-Sent Events. The page
// reads it with fetch so the bridge token can travel in the Authorization
// header; EventSource cannot set headers.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		sendJSON(w, 500, map[string]any{"ok": false, "error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	health := healthPayload()
	tick := 0
	for {
		snap := eventSnapshot{
			At:    time.Now().UTC().Format(time.RFC3339),
			Usage: eventUsage(),
			Net:   netmon.Current(),
		}
		if tick%5 == 0 {
			health = healthPayload()
		}
		snap.Health = health
		body, err := json.Marshal(snap)
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", body); err != nil {
			return
		}
		flusher.Flush()
		tick++
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
