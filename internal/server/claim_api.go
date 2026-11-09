package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/claim"
)

type claimRecord struct {
	At      time.Time     `json:"at"`
	Plan    string        `json:"plan"`
	Status  string        `json:"status"`
	Message string        `json:"message,omitempty"`
	Outcome claim.Outcome `json:"outcome"`
}

var claimHistory = struct {
	sync.Mutex
	entries []claimRecord
}{}

func recordClaim(out claim.Outcome, err error) {
	rec := claimRecord{At: time.Now().UTC(), Plan: out.PlanID, Status: "ok", Outcome: out}
	switch {
	case err == claim.ErrCaptchaRequired:
		rec.Status = "perlu-app"
		rec.Message = out.Message + " — klaim lewat app ZCode (captcha Aliyun), lalu Segarkan"
	case err != nil:
		rec.Status = "gagal"
		rec.Message = err.Error()
	case !out.OK:
		rec.Status = "ditolak"
		rec.Message = out.Message
	}
	claimHistory.Lock()
	claimHistory.entries = append([]claimRecord{rec}, claimHistory.entries...)
	if len(claimHistory.entries) > 50 {
		claimHistory.entries = claimHistory.entries[:50]
	}
	claimHistory.Unlock()
}

// handleClaimHistory returns the attempts this process made, newest first.
func handleClaimHistory(w http.ResponseWriter, r *http.Request) {
	claimHistory.Lock()
	entries := append([]claimRecord(nil), claimHistory.entries...)
	claimHistory.Unlock()
	sendJSON(w, 200, map[string]any{"ok": true, "count": len(entries), "entries": entries})
}

// handleClaimList enumerates claimable manual-claim plans (billing preview +
// marketing banner). Control-plane only: token-gated.
func handleClaimList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	plans, err := claim.ListPlans(ctx)
	cfg := claim.LoadConfig()
	out := map[string]any{
		"ok":      err == nil,
		"enabled": cfg.Enabled,
		"auto":    cfg.Auto,
		"count":   len(plans),
		"plans":   plans,
	}
	if err != nil {
		out["error"] = err.Error()
		sendJSON(w, 502, out)
		return
	}
	sendJSON(w, 200, out)
}

// handleClaimVerificationConfig exposes the Aliyun captcha config so the
// dashboard can run the same widget the desktop app uses. Token-gated.
func handleClaimVerificationConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cfg, err := claim.VerificationConfigFor(ctx)
	if err != nil {
		sendJSON(w, 502, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "config": cfg})
}

// handleClaimNow claims the configured plan or the highest-priority live one.
func handleClaimNow(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var payload struct {
		PlanID        string `json:"plan_id"`
		CaptchaParam  string `json:"captcha_param"`
		CaptchaRegion string `json:"captcha_region"`
	}
	_ = json.Unmarshal(raw, &payload)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	var out claim.Outcome
	param := strings.TrimSpace(payload.CaptchaParam)
	region := strings.TrimSpace(payload.CaptchaRegion)
	if strings.TrimSpace(payload.PlanID) != "" {
		out, err = claim.ClaimWithVerification(ctx, strings.TrimSpace(payload.PlanID), param, region)
	} else {
		out, err = claim.ClaimBestWithVerification(ctx, param, region)
	}
	if err != nil {
		recordClaim(out, err)
		sendJSON(w, 502, map[string]any{"ok": false, "error": err.Error(), "outcome": out})
		return
	}
	recordClaim(out, nil)
	sendJSON(w, 200, map[string]any{"ok": true, "outcome": out})
}
