package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
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
	if rec.Status == "ok" {
		// A success supersedes failed attempts for the same plan: the red
		// "perlu-app" row from a captcha retry is noise once the claim lands.
		kept := claimHistory.entries[:0]
		for _, e := range claimHistory.entries {
			if e.Plan == rec.Plan && e.Status != "ok" {
				continue
			}
			kept = append(kept, e)
		}
		claimHistory.entries = kept
	}
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

// handleClaimAll sweeps every stored account: list its plans, claim the
// best one, and report per-account results. With ZCODE_CAPTCHA_CMD set the
// captcha is minted automatically; otherwise captcha-gated accounts report
// "perlu-app".
func handleClaimAll(w http.ResponseWriter, r *http.Request) {
	base, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	type result struct {
		Account string `json:"account"`
		Plan    string `json:"plan,omitempty"`
		OK      bool   `json:"ok"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}
	results := []result{}
	for _, a := range accounts.List() {
		actx, acancel := context.WithTimeout(base, 90*time.Second)
		plans, err := claim.ListPlansFor(actx, a.JWT, a.DeviceID)
		if err != nil {
			results = append(results, result{Account: a.ID, Status: "gagal", Message: err.Error()})
			acancel()
			continue
		}
		if len(plans) == 0 {
			results = append(results, result{Account: a.ID, Status: "kosong", Message: "tidak ada plan"})
			acancel()
			continue
		}
		out, err := claim.ClaimWithVerificationFor(actx, a.JWT, a.DeviceID, plans[0].PlanID, "", "")
		acancel()
		res := result{Account: a.ID, Plan: plans[0].PlanID}
		switch {
		case err == nil && out.OK:
			res.OK, res.Status = true, "ok"
		case err == claim.ErrCaptchaRequired:
			res.Status, res.Message = "perlu-app", out.Message
		case err != nil:
			res.Status, res.Message = "gagal", err.Error()
		default:
			res.Status, res.Message = "ditolak", out.Message
		}
		results = append(results, res)
	}
	sendJSON(w, 200, map[string]any{"ok": true, "results": results})
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
