package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/auth"
)

type accountLogin struct {
	Flow      *auth.DeviceFlow
	Label     string
	Status    string // pending | ok | error | cancelled
	Error     string
	AccountID string
	Started   time.Time
	cancel    context.CancelFunc
}

var accountLogins = struct {
	sync.Mutex
	m map[string]*accountLogin
}{m: map[string]*accountLogin{}}

// handleAccountLoginStart begins a device flow and polls it in the background
// so the dashboard can show the approval URL without blocking a request.
func handleAccountLoginStart(w http.ResponseWriter, r *http.Request) {
	raw, _ := readBody(r)
	var payload struct {
		Label string `json:"label"`
	}
	_ = json.Unmarshal(raw, &payload)
	label := strings.TrimSpace(payload.Label)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	flow, err := auth.StartDeviceFlow(ctx)
	if err != nil {
		cancel()
		sendJSON(w, 502, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	sess := &accountLogin{Flow: flow, Label: label, Status: "pending", Started: time.Now(), cancel: cancel}
	accountLogins.Lock()
	pruneLoginsLocked()
	accountLogins.m[flow.FlowID] = sess
	accountLogins.Unlock()

	go func() {
		acct, err := accounts.CompleteDeviceFlow(ctx, flow, label)
		accountLogins.Lock()
		defer accountLogins.Unlock()
		if sess.Status == "cancelled" {
			return
		}
		if err != nil {
			sess.Status = "error"
			sess.Error = err.Error()
			return
		}
		sess.Status = "ok"
		sess.AccountID = acct.ID
	}()

	sendJSON(w, 200, map[string]any{
		"ok":               true,
		"flow_id":          flow.FlowID,
		"authorize_url":    flow.AuthorizeURL,
		"expires_at":       flow.ExpiresAt,
		"poll_interval_ms": flow.IntervalSec * 1000,
	})
}

// handleAccountLoginStatus reports one started flow. The account summary is
// added once the flow completes so the dashboard can refresh immediately.
func handleAccountLoginStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("flow_id"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	accountLogins.Lock()
	sess := accountLogins.m[id]
	if sess == nil {
		accountLogins.Unlock()
		sendJSON(w, 404, map[string]any{"ok": false, "error": "unknown login flow"})
		return
	}
	out := map[string]any{
		"ok":            true,
		"flow_id":       sess.Flow.FlowID,
		"status":        sess.Status,
		"authorize_url": sess.Flow.AuthorizeURL,
		"expires_at":    sess.Flow.ExpiresAt,
		"error":         sess.Error,
	}
	accountID := sess.AccountID
	accountLogins.Unlock()

	if accountID != "" {
		for _, a := range accounts.Summarize() {
			if a.ID == accountID {
				out["account"] = a
				break
			}
		}
	}
	sendJSON(w, 200, out)
}

// handleAccountLoginCancel stops waiting; it does not revoke anything upstream.
func handleAccountLoginCancel(w http.ResponseWriter, r *http.Request) {
	raw, _ := readBody(r)
	var payload struct {
		FlowID string `json:"flow_id"`
	}
	_ = json.Unmarshal(raw, &payload)
	id := strings.TrimSpace(payload.FlowID)
	accountLogins.Lock()
	sess := accountLogins.m[id]
	if sess != nil {
		sess.Status = "cancelled"
		sess.cancel()
		delete(accountLogins.m, id)
	}
	accountLogins.Unlock()
	sendJSON(w, 200, map[string]any{"ok": true})
}

// handleAccountRemove forgets one stored account; the ring moves its default
// to the first survivor.
func handleAccountRemove(w http.ResponseWriter, r *http.Request) {
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
	if !accounts.Remove(id) {
		sendJSON(w, 404, map[string]any{"ok": false, "error": "no stored account with that id"})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "accounts": accounts.Summarize()})
}

// pruneLoginsLocked drops finished sessions older than ten minutes.
func pruneLoginsLocked() {
	cutoff := time.Now().Add(-10 * time.Minute)
	for id, sess := range accountLogins.m {
		if sess.Started.Before(cutoff) {
			sess.cancel()
			delete(accountLogins.m, id)
		}
	}
}
