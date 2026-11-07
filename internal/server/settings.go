package server

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/auth"
	"github.com/azrinaldi/zcode-bridge/internal/capture"
	"github.com/azrinaldi/zcode-bridge/internal/claim"
	"github.com/azrinaldi/zcode-bridge/internal/identity"
	"github.com/azrinaldi/zcode-bridge/internal/offpeak"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
	"github.com/azrinaldi/zcode-bridge/internal/update"
)

// settingsEnv lists the environment knobs the console reports. The bridge token
// and plan JWT are never included; only their presence is exposed.
var settingsEnv = []string{
	"ZCODE_CAPTURE", "ZCODE_CAPTURE_SIZE", "ZCODE_CAPTURE_BODY_MAX", "ZCODE_CAPTURE_DIR",
	"ZCODE_ASYNC_ENABLED", "ZCODE_ASYNC_ORIGIN", "ZCODE_ASYNC_MAX_WAIT_MS",
	"ZCODE_CLAIM_ENABLED", "ZCODE_CLAIM_AUTO", "ZCODE_CLAIM_PLAN_ID", "ZCODE_CLAIM_ORIGIN",
	"ZCODE_UPDATE_CHECK", "ZCODE_DESKTOP", "ZCODE_DB", "ZCODE_DEFAULT_MODEL",
	"ZCODE_CORS_ORIGIN", "ZCODE_APP_VERSION",
}

// handleSettings serves the effective configuration the console shows. Values
// come from the running process, so the page labels them restart-required.
func handleSettings(w http.ResponseWriter, r *http.Request) {
	st := capture.Status()
	claimCfg := claim.LoadConfig()
	env := map[string]string{}
	for _, key := range settingsEnv {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		env[key] = raw
	}
	deviceMid := identity.ResolveDeviceMid()
	device := "MISSING"
	if deviceMid != "" {
		device = "set (" + strconv.Itoa(len(deviceMid)) + " chars)"
	}
	planJWT := len(accounts.Summarize()) > 0 || auth.ResolvePlanJWT("") != ""
	sendJSON(w, 200, map[string]any{
		"ok":           true,
		"lane":         "zcode-plan/anthropic",
		"upstream":     transport.InferenceOrigin + transport.PlanAnthropicPath,
		"host":         cfg.Host,
		"port":         cfg.Port,
		"auth":         authLabel(),
		"version":      identity.ResolveAppVersion(),
		"plan_jwt":     planJWT,
		"device_mid":   device,
		"capture":      map[string]any{"enabled": st.Enabled, "capacity": st.Capacity, "body_max": st.BodyMax},
		"async":        offpeak.Enabled(),
		"claim":        claimCfg.Enabled,
		"claim_auto":   claimCfg.Auto,
		"update_check": update.Enabled(),
		"desktop":      desktopEnabled(),
		"env":          env,
	})
}

func authLabel() string {
	if cfg.BridgeToken != "" {
		return "bridge token required"
	}
	return "open (localhost)"
}
