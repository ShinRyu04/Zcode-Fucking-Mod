package server

import (
	"net/http"

	"github.com/azrinaldi/zcode-bridge/internal/usage"
)

// handleUsage serves GET /v1/usage: today and all-time token totals plus the
// recent rows the dashboard sparkline draws. Numbers come from upstream usage
// blocks, never from estimating bytes.
func handleUsage(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, usage.Current())
}
