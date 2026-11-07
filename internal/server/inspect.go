package server

import (
	"net/http"
	"strconv"

	"github.com/azrinaldi/zcode-bridge/internal/capture"
)

// maxInspectFull caps how many bodies one export may carry; the ring already
// limits this, but a hand-set ?limit= would otherwise copy megabytes.
const maxInspectFull = 50

// handleInspect serves the captured traffic ring:
//
//	GET /inspect?limit=N          newest-first summaries (no bodies)
//	GET /inspect?id=N             one exchange, bodies included
//	GET /inspect?limit=N&full=1   newest-first with bodies (JSON export)
//
// Bodies stay out of the listing because a full 200-entry ring with 64KB
// bodies would be 12MB per dashboard refresh.
func handleInspect(w http.ResponseWriter, r *http.Request) {
	state := capture.Status()
	if !state.Enabled {
		sendJSON(w, 200, map[string]any{
			"enabled":  false,
			"count":    0,
			"capacity": state.Capacity,
			"body_max": state.BodyMax,
			"entries":  []capture.Summary{},
			"hint":     "capture is off: restart the bridge with ZCODE_CAPTURE=1 to record exchanges",
		})
		return
	}
	if idText := r.URL.Query().Get("id"); idText != "" {
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			sendJSON(w, 400, map[string]any{"ok": false, "error": "id must be a positive integer"})
			return
		}
		ex, ok := capture.Get(id)
		if !ok {
			sendJSON(w, 404, map[string]any{"ok": false, "error": "no captured exchange with that id"})
			return
		}
		sendJSON(w, 200, ex)
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	full := r.URL.Query().Get("full") == "1"
	if full {
		if limit > maxInspectFull {
			limit = maxInspectFull
		}
		entries := make([]capture.Exchange, 0, limit)
		for _, sum := range capture.List(limit) {
			if ex, ok := capture.Get(sum.ID); ok {
				entries = append(entries, ex)
			}
		}
		sendJSON(w, 200, map[string]any{
			"enabled": true, "count": state.Count, "capacity": state.Capacity,
			"body_max": state.BodyMax, "entries": entries,
		})
		return
	}
	sendJSON(w, 200, map[string]any{
		"enabled": true, "count": state.Count, "capacity": state.Capacity,
		"body_max": state.BodyMax, "entries": capture.List(limit),
	})
}

// handleInspectClear drops the ring (the inspector's "Bersihkan" button).
func handleInspectClear(w http.ResponseWriter) {
	sendJSON(w, 200, map[string]any{"ok": true, "cleared": capture.Clear()})
}
