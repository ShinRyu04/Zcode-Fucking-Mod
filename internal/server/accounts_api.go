package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
)

// handleAccountsList exposes the non-secret account ring for the dashboard's
// account/plan switch. Tokens never leave the store.
func handleAccountsList(w http.ResponseWriter) {
	list := accounts.Summarize()
	sendJSON(w, 200, map[string]any{"ok": true, "count": len(list), "accounts": list})
}

// handleAccountSwitch pins one stored account as the rotation head, which is
// the credential/plan the next inference call uses.
func handleAccountSwitch(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var payload struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &payload) != nil || strings.TrimSpace(payload.ID) == "" {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "body must be {\"id\":\"<account>\"}"})
		return
	}
	id := strings.TrimSpace(payload.ID)
	if !accounts.SetDefault(id) {
		sendJSON(w, 404, map[string]any{
			"ok":    false,
			"error": "no stored account with that id; desktop-store accounts cannot be switched (run zcode-bridge login)",
		})
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok":       true,
		"default":  id,
		"accounts": accounts.Summarize(),
	})
}
