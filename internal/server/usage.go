package server

import (
	"net/http"

	"github.com/azrinaldi/zcode-bridge/internal/usage"
)

// usageResponse is the /v1/usage body: bridge-measured rows plus the desktop
// application's own database snapshot, kept in separate fields on purpose so
// the two sources are never silently mixed.
type usageResponse struct {
	usage.Snapshot
	Desktop desktopPayload `json:"desktop"`
}

// handleUsage serves GET /v1/usage: today and all-time token totals plus the
// recent rows the dashboard sparkline draws. Bridge numbers come from upstream
// usage blocks, never from estimating bytes; the desktop block is read from
// the ZCode CLI database and labelled as such.
func handleUsage(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, usageResponse{Snapshot: usage.Current(), Desktop: currentDesktop()})
}
