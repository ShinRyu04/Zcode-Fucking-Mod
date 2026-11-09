// Package claim ports the manual "weekend plan" claim behavior of
// TriDefender/zcode-api: enumerate claimable campaigns from the billing
// preview plus the marketing banner face, then POST the highest-priority
// plan id to the billing claim endpoint. Source not copied; endpoints and
// header shape are protocol facts.
package claim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/auth"
	"github.com/azrinaldi/zcode-bridge/internal/identity"
)

// Plan is one claimable manual-claim campaign.
type Plan struct {
	PlanID   string `json:"plan_id"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	StartsAt int64  `json:"starts_at,omitempty"`
	EndsAt   int64  `json:"ends_at,omitempty"`
	Source   string `json:"source"`
}

// Outcome is one claim attempt.
type Outcome struct {
	OK        bool   `json:"ok"`
	PlanID    string `json:"plan_id"`
	Code      any    `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	StartsAt  int64  `json:"starts_at,omitempty"`
	EndsAt    int64  `json:"ends_at,omitempty"`
	FailureAt int64  `json:"failure_ends_at,omitempty"`
}

// Config is the claim scheduler's tunables.
type Config struct {
	Enabled      bool
	Auto         bool
	Origin       string
	PollInterval time.Duration
	Cooldown     time.Duration
	PlanID       string
	Timeout      time.Duration
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envMs(key string, fallback int64) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Duration(fallback) * time.Millisecond
}

// LoadConfig reads claim settings (enabled+auto by default, like the
// reference; the scheduler is inert without a plan JWT).
func LoadConfig() Config {
	origin := strings.TrimSpace(os.Getenv("ZCODE_CLAIM_ORIGIN"))
	if origin == "" {
		origin = "https://zcode.z.ai"
	}
	return Config{
		Enabled:      envBool("ZCODE_CLAIM_ENABLED", true),
		Auto:         envBool("ZCODE_CLAIM_AUTO", true),
		Origin:       strings.TrimRight(origin, "/"),
		PollInterval: envMs("ZCODE_CLAIM_POLL_INTERVAL_MS", 300000),
		Cooldown:     envMs("ZCODE_CLAIM_COOLDOWN_MS", 600000),
		PlanID:       strings.TrimSpace(os.Getenv("ZCODE_CLAIM_PLAN_ID")),
		Timeout:      envMs("ZCODE_CLAIM_TIMEOUT_MS", 15000),
	}
}

var resolveJWT = func() string {
	if a := accounts.Usable(nil); len(a) > 0 && a[0].JWT != "" {
		return a[0].JWT // default account first (Usable pins it at the head)
	}
	return auth.ResolvePlanJWT("")
}

// SetJWTResolverForTest swaps credential resolution in tests.
func SetJWTResolverForTest(fn func() string) func() {
	prev := resolveJWT
	resolveJWT = fn
	return func() { resolveJWT = prev }
}

func appVersion() string { return identity.ResolveAppVersion() }

type apiError struct {
	Status int
	Code   any
	Msg    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("claim api HTTP %d code %v: %s", e.Status, e.Code, e.Msg)
}

func doJSON(ctx context.Context, hc *http.Client, method, rawurl string, headers map[string]string, payload any) (map[string]any, *apiError) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, &apiError{Msg: err.Error()}
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawurl, body)
	if err != nil {
		return nil, &apiError{Msg: err.Error()}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, &apiError{Msg: err.Error()}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	var code any
	if parsed != nil {
		code = parsed["code"]
	}
	okStatus := res.StatusCode >= 200 && res.StatusCode < 300
	okCode := code == nil || code == float64(0)
	if parsed != nil && code == nil && res.StatusCode >= 400 {
		okCode = false
	}
	if !okStatus || !okCode {
		msg := ""
		if parsed != nil {
			if s, ok := parsed["msg"].(string); ok {
				msg = s
			} else if s, ok := parsed["message"].(string); ok {
				msg = s
			}
		}
		if msg == "" && len(raw) < 200 {
			msg = string(raw)
		}
		return nil, &apiError{Status: res.StatusCode, Code: code, Msg: msg}
	}
	return parsed, nil
}

func headerAuth(jwt string) map[string]string {
	h := map[string]string{}
	if jwt != "" {
		h["Authorization"] = "Bearer " + jwt
	}
	if mid := identity.ResolveDeviceMid(); mid != "" {
		h["X-Device-Mid"] = mid
	}
	return h
}

// ListPlans merges the billing preview and the marketing banner face,
// deduped by plan id (preview wins), ordered by descending priority.
func ListPlans(ctx context.Context) ([]Plan, error) {
	cfg := LoadConfig()
	hc := &http.Client{Timeout: cfg.Timeout}
	jwt := resolveJWT()
	previewURL := cfg.Origin + "/api/v1/zcode-plan/billing/preview?app_version=" +
		url.QueryEscape(appVersion()) + "&platform=" + url.QueryEscape(runtime.GOOS+"-"+runtime.GOARCH)
	plans, perr := previewPlans(ctx, hc, previewURL, jwt)
	if perr != nil && perr.Status != 404 {
		return nil, perr
	}
	plans = append(plans, marketingPlans(ctx, hc, cfg, jwt)...)
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].Priority > plans[j].Priority })
	return plans, nil
}

func previewPlans(ctx context.Context, hc *http.Client, endpoint, jwt string) ([]Plan, *apiError) {
	env, err := doJSON(ctx, hc, http.MethodGet, endpoint, headerAuth(jwt), nil)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, nil
	}
	data, _ := env["data"].(map[string]any)
	rawPlans, _ := data["plans"].([]any)
	out := make([]Plan, 0, len(rawPlans))
	for _, rp := range rawPlans {
		m, _ := rp.(map[string]any)
		if p, ok := parsePlan(m, "preview"); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func parsePlan(m map[string]any, source string) (Plan, bool) {
	if m == nil {
		return Plan{}, false
	}
	id, _ := m["plan_id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return Plan{}, false
	}
	p := Plan{PlanID: id, Name: id, Source: source}
	if s, ok := m["name"].(string); ok && strings.TrimSpace(s) != "" {
		p.Name = strings.TrimSpace(s)
	}
	if v, ok := m["priority"].(float64); ok {
		p.Priority = int(v)
	}
	if v, ok := m["starts_at"].(float64); ok {
		p.StartsAt = int64(v)
	}
	if v, ok := m["ends_at"].(float64); ok {
		p.EndsAt = int64(v)
	}
	return p, true
}

// marketingPlans scans marketing/touch deliveries for buttons whose action
// is claim_zcode_plan (depth-limited, defensive parse like the reference).
func marketingPlans(ctx context.Context, hc *http.Client, cfg Config, jwt string) []Plan {
	mid := identity.ResolveDeviceMid()
	if mid == "" {
		return nil
	}
	headers := headerAuth(jwt)
	headers["X-Client-Language"] = "zh-CN"
	headers["X-ZCode-App-Version"] = appVersion()
	env, err := doJSON(ctx, hc, http.MethodGet, cfg.Origin+"/api/v1/marketing/touch?seq=0", headers, nil)
	if err != nil || env == nil {
		return nil
	}
	data, _ := env["data"].(map[string]any)
	deliveries, _ := data["deliveries"].([]any)
	seen := map[string]bool{}
	var out []Plan
	var walk func(node any, depth int)
	var collect func(node any, depth int, plans *[]map[string]any)
	collect = func(node any, depth int, plans *[]map[string]any) {
		if depth > 4 || node == nil {
			return
		}
		switch v := node.(type) {
		case []any:
			for _, item := range v {
				collect(item, depth+1, plans)
			}
		case map[string]any:
			if zp, ok := v["zcode_plan"].(map[string]any); ok {
				*plans = append(*plans, zp)
			}
			for _, child := range v {
				collect(child, depth+1, plans)
			}
		}
	}
	walk = func(delivery any, depth int) {
		d, _ := delivery.(map[string]any)
		if d == nil {
			return
		}
		rich := []map[string]any{}
		collect(d, 0, &rich)
		richByID := map[string]map[string]any{}
		for _, rp := range rich {
			id, _ := rp["plan_id"].(string)
			richByID[strings.TrimSpace(id)] = rp
		}
		for _, face := range []string{"banner", "popup"} {
			host, _ := d[face].(map[string]any)
			if host == nil {
				continue
			}
			buttons, _ := host["buttons"].([]any)
			for _, b := range buttons {
				bm, _ := b.(map[string]any)
				action, _ := bm["action"].(map[string]any)
				if action == nil {
					continue
				}
				if t, _ := action["type"].(string); t != "claim_zcode_plan" {
					continue
				}
				args, _ := action["args"].(map[string]any)
				id, _ := args["plan_id"].(string)
				id = strings.TrimSpace(id)
				if id == "" || seen[id] {
					continue
				}
				raw := richByID[id]
				if raw == nil {
					raw = map[string]any{"plan_id": id}
				}
				if p, ok := parsePlan(raw, "banner"); ok {
					seen[id] = true
					out = append(out, p)
				}
			}
		}
	}
	for _, d := range deliveries {
		walk(d, 0)
	}
	return out
}

// Claim posts one plan id to the billing claim endpoint. Captcha-gated
// campaigns cannot be solved here; the error names that case.
func Claim(ctx context.Context, planID string) (Outcome, error) {
	cfg := LoadConfig()
	hc := &http.Client{Timeout: cfg.Timeout}
	jwt := resolveJWT()
	if jwt == "" {
		return Outcome{PlanID: planID, Message: "login_required"}, fmt.Errorf("claim requires a plan JWT (run zcode-bridge login)")
	}
	headers := map[string]string{
		"Authorization":       "Bearer " + jwt,
		"Content-Type":        "application/json",
		"X-ZCode-App-Version": appVersion(),
		"X-Platform":          runtime.GOOS + "-" + runtime.GOARCH,
	}
	if mid := identity.ResolveDeviceMid(); mid != "" {
		headers["X-Device-Mid"] = mid
	}
	env, apiErr := doJSON(ctx, hc, http.MethodPost, cfg.Origin+"/api/v1/zcode-plan/billing/claim", headers, map[string]any{"plan_id": planID})
	if apiErr != nil {
		msg := apiErr.Msg
		switch fmt.Sprint(apiErr.Code) {
		case "4001", "4002", "4003":
			msg += " (captcha/reCAPTCHA campaign: claim from the ZCode desktop app)"
		}
		return Outcome{PlanID: planID, Code: apiErr.Code, Message: msg}, apiErr
	}
	out := Outcome{OK: true, PlanID: planID}
	if env != nil {
		if data, ok := env["data"].(map[string]any); ok {
			if plan, ok := data["plan"].(map[string]any); ok {
				if v, ok := plan["starts_at"].(float64); ok {
					out.StartsAt = int64(v)
				}
				if v, ok := plan["ends_at"].(float64); ok {
					out.EndsAt = int64(v)
				}
			}
		}
	}
	return out, nil
}

// ClaimBest lists plans and claims the configured plan or the highest
// priority one. It returns the outcome and any error.
func ClaimBest(ctx context.Context) (Outcome, error) {
	cfg := LoadConfig()
	target := cfg.PlanID
	if target == "" {
		plans, err := ListPlans(ctx)
		if err != nil {
			return Outcome{}, err
		}
		if len(plans) == 0 {
			return Outcome{}, ErrNothingToClaim
		}
		target = plans[0].PlanID
	}
	return Claim(ctx, target)
}

// ErrNothingToClaim means no campaign is live for this account.
var ErrNothingToClaim = fmt.Errorf("no claimable plan right now")

// Start launches the background scheduler when enabled+auto and a JWT is
// present. It is a no-op otherwise and never blocks.
func Start(ctx context.Context) {
	cfg := LoadConfig()
	if !cfg.Enabled || !cfg.Auto {
		return
	}
	if resolveJWT() == "" {
		return
	}
	go func() {
		nextAllowed := time.Now()
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		attempt := func() {
			if time.Now().Before(nextAllowed) {
				return
			}
			out, err := ClaimBest(ctx)
			switch {
			case err == nil && out.OK:
				log.Printf("claim: plan %s claimed (starts %s)", out.PlanID, time.UnixMilli(out.StartsAt).Format(time.RFC3339))
				nextAllowed = time.Now().Add(cfg.PollInterval)
			case err == ErrNothingToClaim:
				nextAllowed = time.Now().Add(cfg.PollInterval)
			default:
				if err != nil {
					log.Printf("claim: %v", err)
				}
				nextAllowed = time.Now().Add(cfg.Cooldown)
			}
		}
		attempt()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				attempt()
			}
		}
	}()
}
