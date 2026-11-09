package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/auth"
	"github.com/azrinaldi/zcode-bridge/internal/capture"
	"github.com/azrinaldi/zcode-bridge/internal/claim"
	"github.com/azrinaldi/zcode-bridge/internal/identity"
	"github.com/azrinaldi/zcode-bridge/internal/netmon"
	"github.com/azrinaldi/zcode-bridge/internal/quota"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
	"github.com/azrinaldi/zcode-bridge/internal/webui"
)

// Route dispatches one request; handler errors that happen after the
// headers are sent are impossible here because every handler writes its
// own status first.
func Route() http.Handler {
	return http.HandlerFunc(route)
}

func route(w http.ResponseWriter, r *http.Request) {
	if !cors(w, r) {
		return // preflight handled
	}
	pathName := strings.TrimRight(r.URL.Path, "/")
	if pathName == "" {
		pathName = "/"
	}
	accountID := r.URL.Query().Get("account")
	if accountID == "" {
		accountID = r.Header.Get("X-Zcode-Account")
	}

	switch {
	case r.Method == http.MethodGet && (pathName == "/health" || pathName == "/healthz"):
		handleHealth(w)
	case r.Method == http.MethodGet && (pathName == "/v1/models" || pathName == "/models"):
		sendJSON(w, 200, map[string]any{"object": "list", "data": liveModels(r.Context())})
	case r.Method == http.MethodGet && (pathName == "/v1/quota" || pathName == "/quota"):
		handleQuota(w, r, accountID)
	case r.Method == http.MethodGet && pathName == "/dashboard":
		webui.Page(w, r)
	case r.Method == http.MethodGet && pathName == "/chat":
		webui.Page(w, r)
	case r.Method == http.MethodGet && pathName == "/classic":
		handleDashboard(w)
	case r.Method == http.MethodGet && strings.HasPrefix(pathName, "/assets/"):
		webui.Asset(w, r)
	case r.Method == http.MethodGet && pathName == "/v1/events":
		if requireObservabilityToken(w, r) {
			handleEvents(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/settings":
		if requireObservabilityToken(w, r) {
			handleSettings(w, r)
		}
	case r.Method == http.MethodGet && (pathName == "/overlay" || strings.HasPrefix(pathName, "/overlay/")):
		handleOverlay(w, r)
	case r.Method == http.MethodGet && (pathName == "/inspect" || pathName == "/inspector"):
		if requireObservabilityToken(w, r) {
			handleInspect(w, r)
		}
	case r.Method == http.MethodDelete && pathName == "/inspect":
		if requireObservabilityToken(w, r) {
			handleInspectClear(w)
		}
	case r.Method == http.MethodPost && pathName == "/inspect/replay":
		if requireObservabilityToken(w, r) {
			handleInspectReplay(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/claim/history":
		if requireObservabilityToken(w, r) {
			handleClaimHistory(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/claim/verification-config":
		if requireObservabilityToken(w, r) {
			handleClaimVerificationConfig(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/claim":
		if requireObservabilityToken(w, r) {
			handleClaimList(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/claim":
		if requireObservabilityToken(w, r) {
			handleClaimNow(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/accounts":
		if requireObservabilityToken(w, r) {
			handleAccountsList(w)
		}
	case r.Method == http.MethodGet && pathName == "/v1/accounts/pool":
		if requireObservabilityToken(w, r) {
			handleAccountsPool(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/accounts/switch":
		if requireObservabilityToken(w, r) {
			handleAccountSwitch(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/accounts/remove":
		if requireObservabilityToken(w, r) {
			handleAccountRemove(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/accounts/login":
		if requireObservabilityToken(w, r) {
			handleAccountLoginStart(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/accounts/login/status":
		if requireObservabilityToken(w, r) {
			handleAccountLoginStatus(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/accounts/login/cancel":
		if requireObservabilityToken(w, r) {
			handleAccountLoginCancel(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/integration":
		if requireObservabilityToken(w, r) {
			handleIntegration(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/integration/key":
		if requireObservabilityToken(w, r) {
			handleIntegrationKey(w, r)
		}
	case r.Method == http.MethodDelete && pathName == "/v1/integration/key":
		if requireObservabilityToken(w, r) {
			handleIntegrationKeyDelete(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/sharing":
		if requireObservabilityToken(w, r) {
			handleSharingList(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/sharing":
		if requireObservabilityToken(w, r) {
			handleSharingCreate(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/sharing/like":
		if requireObservabilityToken(w, r) {
			handleSharingLike(w, r)
		}
	case r.Method == http.MethodPost && pathName == "/v1/sharing/comment":
		if requireObservabilityToken(w, r) {
			handleSharingComment(w, r)
		}
	case r.Method == http.MethodDelete && pathName == "/v1/sharing":
		if requireObservabilityToken(w, r) {
			handleSharingDelete(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/usage":
		if requireObservabilityToken(w, r) {
			handleUsage(w, r)
		}
	case r.Method == http.MethodGet && pathName == "/v1/net":
		if requireObservabilityToken(w, r) {
			sendJSON(w, 200, netmon.Current())
		}
	case (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodDelete) && pathName == "/mcp":
		handleMCP(w, r)
	case r.Method == http.MethodGet && pathName == "/async/v1/health":
		handleAsyncHealth(w, r)
	case r.Method == http.MethodPost && pathName == "/async/v1/messages":
		requireAuth(w, r, "anthropic", func() {
			handleAsyncMessages(w, r, accountID)
		})
	case r.Method == http.MethodPost && pathName == "/async/v1/chat/completions":
		requireAuth(w, r, "openai", func() {
			handleAsyncChat(w, r, accountID)
		})
	case r.Method == http.MethodPost && pathName == "/v1/messages":
		requireAuth(w, r, "anthropic", func() {
			handlePost(w, r, accountID, handleMessages)
		})
	case r.Method == http.MethodPost && pathName == "/v1/chat/completions":
		requireAuth(w, r, "openai", func() {
			handlePost(w, r, accountID, handleChatCompletions)
		})
	case r.Method == http.MethodPost && pathName == "/v1/responses":
		requireAuth(w, r, "openai", func() {
			handlePost(w, r, accountID, handleResponses)
		})
	default:
		anthropicError(w, 404, fmt.Sprintf("No route for %s %s", r.Method, r.URL.Path), "not_found_error")
	}
}

// requireObservabilityToken gates routes that expose captured traffic or usage
// aggregates: ZCODE_BRIDGE_TOKEN when set, otherwise open on loopback (same
// rule the /dashboard page itself follows).
func requireObservabilityToken(w http.ResponseWriter, r *http.Request) bool {
	if authorized(r) {
		return true
	}
	sendJSON(w, 401, map[string]any{"ok": false, "error": "invalid bridge token"})
	return false
}

func requireAuth(w http.ResponseWriter, r *http.Request, dialect string, next func()) {
	if !authorized(r) {
		if dialect == "openai" {
			openAIError(w, 401, "Invalid bridge token", "invalid_api_key")
		} else {
			anthropicError(w, 401, "Invalid bridge token", "authentication_error")
		}
		return
	}
	next()
}

func handlePost(w http.ResponseWriter, r *http.Request, accountID string, handler func(http.ResponseWriter, *http.Request, map[string]any, string)) {
	raw, err := readBody(r)
	if err != nil {
		anthropicError(w, 400, "Failed to read request body", "invalid_request_error")
		return
	}
	if len(raw) > maxBodyBytes {
		anthropicError(w, 413, "Request body too large", "invalid_request_error")
		return
	}
	body, ok := parseJSON(raw)
	if !ok {
		anthropicError(w, 400, "Invalid JSON body", "invalid_request_error")
		return
	}
	// Capture wraps the writer before the handler runs, so the inspector sees
	// the real response (status, headers, truncated body) without buffering:
	// streaming writers still forward every byte as it arrives.
	if rec := capture.Begin(r, raw); rec != nil {
		w = rec.Wrap(w)
		r = r.WithContext(capture.WithRecorder(r.Context(), rec))
		defer rec.Finish()
	}
	handler(w, r, body, accountID)
}

// handleHealth reports lane, credential and rotation state (no secrets).
func handleHealth(w http.ResponseWriter) {
	sendJSON(w, 200, healthPayload())
}

// healthPayload is shared by /health and the SSE snapshot so both report the
// same credential state.
func healthPayload() map[string]any {
	ring := accounts.Summarize()
	deviceMid := identity.ResolveDeviceMid()
	planJwt := map[string]any{"present": false}
	if len(ring) > 0 {
		planJwt = map[string]any{"present": true, "count": len(ring)}
	} else if jwt := auth.ResolvePlanJWT(""); jwt != "" {
		claims, _ := auth.DecodeJWT(jwt)
		entry := map[string]any{"present": true}
		if iat, ok := claims["iat"].(float64); ok {
			entry["issuedAt"] = time.Unix(int64(iat), 0).UTC().Format(time.RFC3339)
		}
		if uid, ok := claims["user_id"].(string); ok {
			entry["userId"] = uid
		}
		planJwt = entry
	}
	authMode := "open (localhost)"
	if effectiveBridgeToken() != "" {
		authMode = "bridge token required"
	}
	return map[string]any{
		"ok":            true,
		"lane":          "zcode-plan/anthropic",
		"upstream":      transport.InferenceOrigin + transport.PlanAnthropicPath,
		"clientVersion": identity.ResolveAppVersion(),
		"deviceMid": func() string {
			if deviceMid != "" {
				return "set"
			}
			return "MISSING (control calls may fail 3001)"
		}(),
		"accounts": ring,
		"planJwt":  planJwt,
		"auth":     authMode,
		"version":  identity.ResolveAppVersion(),
	}
}

// handleQuota serves the billing readout for the first usable account (or
// the ?account= pin).
func handleQuota(w http.ResponseWriter, r *http.Request, accountID string) {
	cands, err := transport.Candidates(accountID, "")
	if err != nil || len(cands) == 0 {
		sendJSON(w, 502, map[string]any{"ok": false, "error": "no usable plan credential: run login"})
		return
	}
	acct := cands[0]
	q, err := quota.ReadQuota(r.Context(), acct.JWT, acct.DeviceID)
	if err != nil {
		sendJSON(w, 502, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	q.Account = firstNonEmpty(acct.Label, acct.ID, accountID, "default")
	sendJSON(w, 200, q)
}

// cors implements opt-in browser support: ZCODE_CORS_ORIGIN="*" or a comma
// list of allowed origins. Returns false when the request was handled as a
// preflight.
func cors(w http.ResponseWriter, r *http.Request) bool {
	corsOrigins := strings.TrimSpace(firstNonEmpty(os.Getenv("ZCODE_CORS_ORIGIN"), ""))
	if corsOrigins == "" {
		return true
	}
	origin := r.Header.Get("Origin")
	allowed := corsOrigins == "*"
	if !allowed && origin != "" {
		for _, o := range strings.Split(corsOrigins, ",") {
			if strings.TrimSpace(o) == origin {
				allowed = true
				break
			}
		}
	}
	if allowed && origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Api-Key, X-Session-Id, X-Zcode-Account")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	return true
}

// ListenAndServe starts the HTTP server. keepAlive timeouts mirror the JS
// server's 65s values so long SSE streams survive intermediary idle cuts.
func ListenAndServe() error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	netmon.Start(cfg.Port)
	claim.Start(context.Background())
	srv := &http.Server{
		Addr:              addr,
		Handler:           Route(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       66 * time.Second,
	}
	return srv.ListenAndServe()
}
