package server

// End-to-end test suite against a stub gateway, mirroring the JS smoke
// checks 1:1. The stub answers the plan lane, the billing pair and the
// token endpoint so no real credential is touched. All JWTs here are
// synthetic fixtures (unsigned base64 payloads), never secrets.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/accounts"
	"github.com/azrinaldi/zcode-bridge/internal/auth"
	"github.com/azrinaldi/zcode-bridge/internal/capture"
	"github.com/azrinaldi/zcode-bridge/internal/systemprompt"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
	"github.com/azrinaldi/zcode-bridge/internal/usage"
)

func fakeJWT(user string) string {
	payload := base64URL([]byte(fmt.Sprintf(`{"user_id":%q,"iat":%d}`, user, time.Now().Unix())))
	return "eyJhbGciOiJIUzI1NiJ9." + payload + ".sig"
}

func base64URL(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, (len(b)+2)/3*4)
	for i := 0; i < len(b); i += 3 {
		var chunk [3]byte
		copy(chunk[:], b[i:])
		n := 0x10000*int(chunk[0]) + 0x100*int(chunk[1]) + int(chunk[2])
		out = append(out, alphabet[(n>>18)&63], alphabet[(n>>12)&63])
		if i+1 < len(b) {
			out = append(out, alphabet[(n>>6)&63])
		} else {
			out = append(out, '=')
		}
		if i+2 < len(b) {
			out = append(out, alphabet[n&63])
		} else {
			out = append(out, '=')
		}
	}
	return strings.TrimRight(string(out), "=")
}

var (
	JWT     = fakeJWT("u_smoke")
	FAILJWT = fakeJWT("u_spent")
	EXPIRED = "EXPIRED"
)

type seenState struct {
	mu           sync.Mutex
	anthropic    map[string]any
	anthropicHdr http.Header
	bearers      []string
	refreshCalls int
	currentHdr   http.Header
	balanceHdr   http.Header
}

var seen = &seenState{}

func stubGateway() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		seen.mu.Lock()
		seen.refreshCalls++
		seen.mu.Unlock()
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{
			"token": JWT, "zai": map[string]any{"access_token": "at-new", "refresh_token": "rt-new"},
		}})
	})
	mux.HandleFunc("/api/v1/zcode-plan/anthropic/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen.mu.Lock()
		seen.anthropic = body
		seen.anthropicHdr = r.Header.Clone()
		seen.bearers = append(seen.bearers, r.Header.Get("Authorization"))
		seen.mu.Unlock()
		bearer := r.Header.Get("Authorization")
		if r.Header.Get("X-Client-Sig") != "" || r.Header.Get("X-Client-Pow") != "" || r.Header.Get("X-App-Id") != "" {
			writeJSON(w, 429, map[string]any{"code": 1113, "msg": "insufficient balance (signed request hit the paid lane)"})
			return
		}
		if !strings.HasPrefix(bearer, "Bearer ") {
			writeJSON(w, 401, map[string]any{"code": 401, "msg": "unauthorized"})
			return
		}
		if bearer == "Bearer "+FAILJWT {
			writeJSON(w, 429, map[string]any{"code": 1113, "msg": "plan allowance spent for this account"})
			return
		}
		if bearer == "Bearer "+EXPIRED {
			writeJSON(w, 401, map[string]any{"code": 401, "msg": "token expired"})
			return
		}
		sys, _ := body["system"].([]any)
		ok := len(sys) == 3
		if ok {
			b1, _ := sys[0].(map[string]any)
			b2, _ := sys[1].(map[string]any)
			if b1["text"] != systemprompt.Captured().CliPrefix ||
				!strings.HasPrefix(str(b2["text"]), systemprompt.Captured().StablePrefix[:60]) {
				ok = false
			}
		}
		if !ok {
			writeJSON(w, 405, map[string]any{"code": 3012, "msg": "request has been blocked due to unusual activity."})
			return
		}
		tools, _ := body["tools"].([]any)
		stream := body["stream"] == true
		if !stream {
			if len(tools) > 0 {
				messages, _ := body["messages"].([]any)
				hasToolResult := false
				for _, rawMessage := range messages {
					message, _ := rawMessage.(map[string]any)
					content, _ := message["content"].([]any)
					for _, rawBlock := range content {
						block, _ := rawBlock.(map[string]any)
						if block["type"] == "tool_result" || block["type"] == "mcp_tool_result" {
							hasToolResult = true
						}
					}
				}
				if hasToolResult {
					writeJSON(w, 200, map[string]any{
						"id": "msg_stub_final", "type": "message", "role": "assistant", "model": "glm-5.3-flash",
						"content": []any{map[string]any{"type": "text", "text": "TOOL-FINAL"}}, "stop_reason": "end_turn",
						"usage": map[string]any{"input_tokens": 20, "output_tokens": 4},
					})
					return
				}
				toolName := "get_weather"
				if first, ok := tools[0].(map[string]any); ok && str(first["name"]) != "" {
					toolName = str(first["name"])
				}
				writeJSON(w, 200, map[string]any{
					"id": "msg_stub", "type": "message", "role": "assistant", "model": "glm-5.3-flash",
					"content":     []any{map[string]any{"type": "tool_use", "id": "toolu_stub", "name": toolName, "input": map[string]any{"city": "Jakarta"}}},
					"stop_reason": "tool_use", "usage": map[string]any{"input_tokens": 11, "output_tokens": 7},
				})
				return
			}
			writeJSON(w, 200, map[string]any{
				"id": "msg_stub", "type": "message", "role": "assistant", "model": "glm-5.3-flash",
				"content":     []any{map[string]any{"type": "text", "text": "STUB-OK"}},
				"stop_reason": "end_turn", "usage": map[string]any{"input_tokens": 11, "output_tokens": 7},
			})
			return
		}
		var events []map[string]any
		if len(tools) > 0 {
			events = []map[string]any{
				{"type": "message_start", "message": map[string]any{"id": "msg_stub", "model": "glm-5.3-flash", "usage": map[string]any{"input_tokens": 0, "output_tokens": 0}}},
				{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": "toolu_stub", "name": "get_weather"}},
				{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"city":"Jakarta"}`}},
				{"type": "content_block_stop", "index": 0},
				{"type": "message_delta", "delta": map[string]any{"stop_reason": "tool_use"}, "usage": map[string]any{"input_tokens": 12, "output_tokens": 9}},
				{"type": "message_stop"},
			}
		} else {
			events = []map[string]any{
				{"type": "message_start", "message": map[string]any{"id": "msg_stub", "model": "glm-5.3-flash", "usage": map[string]any{"input_tokens": 0, "output_tokens": 0}}},
				{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}},
				{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "STUB"}},
				{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "-OK"}},
				{"type": "content_block_stop", "index": 0},
				{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 5}},
				{"type": "message_stop"},
			}
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		for _, e := range events {
			encoded, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e["type"], encoded)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	})
	mux.HandleFunc("/api/v1/zcode-plan/billing/current", func(w http.ResponseWriter, r *http.Request) {
		seen.mu.Lock()
		seen.currentHdr = r.Header.Clone()
		seen.mu.Unlock()
		// Real behaviour 2026-10-30: with the control headers the payload is empty.
		plans := []any{}
		if r.Header.Get("X-Device-Mid") == "" {
			plans = []any{map[string]any{
				"name": "ZCODE WEEKEND BUILD (current-fallback)", "plan_id": "wk", "status": "active",
				"ends_at": 1793581200, "priority": 101,
				"entitlements": []any{map[string]any{
					"entitlement_id": "ent_wk", "show_name": "GLM-5.3-Flash", "meter": "model_usage",
					"unit_type": "token", "capabilities": []string{"model:glm-5.3-flash"},
					"grant_units": 500000000, "period": "one_time", "priority": 101,
				}},
			}}
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"server_time": time.Now().Unix(), "plans": plans}})
	})
	mux.HandleFunc("/api/v1/zcode-plan/billing/balance", func(w http.ResponseWriter, r *http.Request) {
		seen.mu.Lock()
		seen.balanceHdr = r.Header.Clone()
		seen.mu.Unlock()
		if r.Header.Get("X-Device-Mid") == "" {
			writeJSON(w, 400, map[string]any{"code": 3001, "msg": "parameter error"})
			return
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{
			"server_time": time.Now().Unix(),
			"plans": []any{
				map[string]any{"name": "ZCODE WEEKEND BUILD", "plan_id": "wk", "status": "active", "ends_at": 1793581200, "priority": 101,
					"entitlements": []any{map[string]any{"entitlement_id": "ent_wk", "show_name": "GLM-5.3-Flash", "meter": "model_usage", "unit_type": "token", "capabilities": []string{"model:glm-5.3-flash"}, "grant_units": 500000000, "period": "one_time", "priority": 101}}},
				map[string]any{"name": "ZCode Start Plan", "plan_id": "sp", "status": "active", "ends_at": 1730000000, "priority": 90,
					"entitlements": []any{
						map[string]any{"entitlement_id": "ent_53", "show_name": "GLM-5.3", "meter": "model_usage", "unit_type": "token", "capabilities": []string{"model:glm-5.3"}, "grant_units": 3000000, "period": "daily", "priority": 110},
						map[string]any{"entitlement_id": "ent_flash", "show_name": "GLM-5.3-Flash", "meter": "model_usage", "unit_type": "token", "capabilities": []string{"model:glm-5.3-flash"}, "grant_units": 5000000, "period": "daily", "priority": 80},
					}},
			},
			"balances": []any{
				map[string]any{"entitlement_id": "ent_53", "show_name": "GLM-5.3", "plan_id": "sp", "period": "daily", "period_start": 1793376000, "period_end": 1793462399, "total_units": 3000000, "used_units": 22, "remaining_units": 2999978, "available_units": 2999978},
				map[string]any{"entitlement_id": "ent_wk", "show_name": "GLM-5.3-Flash", "plan_id": "wk", "period": "one_time", "total_units": 500000000, "used_units": 0, "remaining_units": 500000000, "available_units": 500000000},
				map[string]any{"entitlement_id": "ent_flash", "show_name": "GLM-5.3-Flash", "plan_id": "sp", "period": "daily", "period_start": 1793376000, "period_end": 1793462399, "total_units": 5000000, "used_units": 27927, "remaining_units": 4972073, "available_units": 4972073},
			},
		}})
	})
	srv := httptest.NewServer(mux)
	return srv
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

var (
	bridge     *httptest.Server
	bridgeOnce sync.Once
)

func bridgeURL() string { return bridge.URL }

func setupBridge(t *testing.T) {
	t.Helper()
	bridgeOnce.Do(func() {
		gateway := stubGateway()
		// No per-test Cleanup: bridgeOnce keeps these alive for the whole
		// test binary; the process exit reclaims them.
		os.Setenv("ZCODE_INFERENCE_ORIGIN", gateway.URL)
		transport.InferenceOrigin = gateway.URL
		os.Setenv("ZCODE_OAUTH_ORIGIN", gateway.URL)
		auth.OAuthBase = gateway.URL
		os.Setenv("ZCODE_PLAN_JWT", JWT)
		os.Setenv("ZCODE_PLAN_JWT_FILE", filepath.Join(os.TempDir(), "zcode-smoke-no-such-jwt.json"))
		os.Setenv("ZCODE_IDENTITY_DEVICE_MID", "0a7c2d9d-ce62-4456-aa65-625511ff1ef0")
		os.Setenv("ZCODE_RETRY_BASE_MS", "10")
		accountsFile := filepath.Join(os.TempDir(), fmt.Sprintf("zcode-smoke-accounts-%d.json", os.Getpid()))
		os.Setenv("ZCODE_ACCOUNTS_FILE", accountsFile)
		if dir, dirErr := os.MkdirTemp("", "zcode-usage-"); dirErr == nil {
			os.Setenv("ZCODE_USAGE_DIR", dir)
		}
		accounts.ResetForTest()
		Configure(Config{Port: 0, Host: "127.0.0.1", BridgeToken: "", ModelsCache: 100 * time.Millisecond})
		capture.EnableForTest(50, 1<<14)
		bridge = httptest.NewServer(Route())
	})
}

func doRequest(t *testing.T, method, path string, body map[string]any) (int, string, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, bridgeURL()+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	text := buf.String()
	var parsed map[string]any
	if strings.HasPrefix(strings.TrimSpace(text), "{") || strings.HasPrefix(strings.TrimSpace(text), "[") {
		_ = json.Unmarshal([]byte(text), &parsed)
	}
	return res.StatusCode, text, parsed
}

func resetSeen() {
	seen.mu.Lock()
	defer seen.mu.Unlock()
	seen.anthropic = nil
	seen.bearers = nil
	seen.refreshCalls = 0
	seen.currentHdr = nil
	seen.balanceHdr = nil
}

func TestDashboardServesStatusPage(t *testing.T) {
	setupBridge(t)
	resetSeen()
	res, err := http.Get(bridgeURL() + "/classic")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	for _, want := range []string{"zcode-bridge", "/health", "/quota", "id=\"health\"", "id=\"quota\"",
		"Inspector", "id=\"panel-inspector\"", "/inspect?limit=200",
		"Penggunaan", "id=\"panel-usage\"", "renderUsageChart", "/v1/usage",
		"Playground", "id=\"panel-playground\"", "gaugeCard", "renderRangeChart", "renderModelTrend", "id=\"urange\"",
		"Modul tampilan", "Monitor jaringan", "renderNet", "id=\"statusbar\"", "/overlay",
		"Aplikasi ZCode", "id=\"udchart\"", "renderDesktop", "desktopChart", "id=\"udrange\"",
		"id=\"claimcard\"", "loadClaimPlans", "id=\"accsel\"", "switchAccount"} {
		if !strings.Contains(page, want) {
			t.Fatalf("dashboard page missing %q", want)
		}
	}
}

func TestDashboardServesShell(t *testing.T) {
	setupBridge(t)
	res, err := http.Get(bridgeURL() + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	raw, _ := io.ReadAll(res.Body)
	page := string(raw)
	for _, want := range []string{"/assets/app.css", "/assets/app.js", "/assets/chat.js",
		"id=\"view\"", "id=\"nav\"", "id=\"tokendlg\"", "Ringkasan", "Chat"} {
		if !strings.Contains(page, want) {
			t.Fatalf("dashboard shell missing %q", want)
		}
	}

	asset, err := http.Get(bridgeURL() + "/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer asset.Body.Close()
	if asset.StatusCode != 200 {
		t.Fatalf("asset status %d", asset.StatusCode)
	}
	if ct := asset.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("asset content-type = %q", ct)
	}
	js, _ := io.ReadAll(asset.Body)
	for _, want := range []string{"/v1/events", "/v1/settings", "/v1/quota", "/inspect"} {
		if !strings.Contains(string(js), want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestEventsStreamsSnapshot(t *testing.T) {
	setupBridge(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bridgeURL()+"/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	reader := bufio.NewReader(res.Body)
	var data string
	for i := 0; i < 8; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read frame: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			break
		}
	}
	if data == "" {
		t.Fatal("no data frame")
	}
	var snap map[string]any
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		t.Fatalf("frame json: %v", err)
	}
	for _, key := range []string{"usage", "net", "health"} {
		if _, ok := snap[key]; !ok {
			t.Fatalf("snapshot missing %q: %s", key, data)
		}
	}
	usageBody := snap["usage"].(map[string]any)
	if _, ok := usageBody["rings"]; !ok {
		t.Fatalf("usage missing rings: %v", usageBody)
	}
}

func TestSettingsReportsEffectiveConfig(t *testing.T) {
	setupBridge(t)
	status, _, body := doRequest(t, "GET", "/v1/settings", nil)
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	if body["lane"] != "zcode-plan/anthropic" {
		t.Fatalf("lane = %v", body["lane"])
	}
	if _, ok := body["capture"].(map[string]any); !ok {
		t.Fatalf("capture = %v", body["capture"])
	}
	if _, ok := body["env"].(map[string]any); !ok {
		t.Fatalf("env = %v", body["env"])
	}
	if body["port"] == nil {
		t.Fatal("port missing")
	}
}

func TestHealthReportsPlanLane(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, _, body := doRequest(t, "GET", "/health", nil)
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	if body["lane"] != "zcode-plan/anthropic" {
		t.Fatalf("lane = %v", body["lane"])
	}
	planJwt := body["planJwt"].(map[string]any)
	if planJwt["present"] != true {
		t.Fatalf("planJwt = %v", planJwt)
	}
}

func TestAnthropicPassthroughIdentityBlocks(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, text, body := doRequest(t, "POST", "/v1/messages", map[string]any{
		"model":    "GLM-5.3-Flash",
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
		"system":   "You are a terse assistant.",
	})
	if status != 200 {
		t.Fatalf("status %d: %s", status, text)
	}
	content := body["content"].([]any)
	if str(content[0].(map[string]any)["text"]) != "STUB-OK" {
		t.Fatalf("content = %v", content)
	}
	seen.mu.Lock()
	sys := seen.anthropic["system"].([]any)
	hdr := seen.anthropicHdr
	seen.mu.Unlock()
	if len(sys) != 3 {
		t.Fatalf("system blocks = %d", len(sys))
	}
	b1 := sys[0].(map[string]any)
	b2 := sys[1].(map[string]any)
	b3 := sys[2].(map[string]any)
	if str(b1["text"]) != systemprompt.Captured().CliPrefix {
		t.Fatal("block 1 must be cliPrefix verbatim")
	}
	if !strings.HasPrefix(str(b2["text"]), systemprompt.Captured().StablePrefix[:80]) {
		t.Fatal("block 2 must start with stablePrefix")
	}
	if !strings.Contains(str(b3["text"]), "terse assistant") {
		t.Fatal("client system rides block 3")
	}
	if hdr.Get("X-Client-Sig") != "" {
		t.Fatal("no signed headers on the plan lane")
	}
	if hdr.Get("Authorization") != "Bearer "+JWT {
		t.Fatalf("bearer = %q", hdr.Get("Authorization"))
	}
	if !regexp.MustCompile(`^session_[0-9a-f]{32}$`).MatchString(hdr.Get("X-Session-Id")) {
		t.Fatalf("x-session-id = %q", hdr.Get("X-Session-Id"))
	}
}

func TestOpenAIStreamChunks(t *testing.T) {
	setupBridge(t)
	resetSeen()
	req, _ := http.NewRequest("POST", bridgeURL()+"/v1/chat/completions", bytes.NewReader([]byte(
		`{"model":"glm-5.3-flash","stream":true,"messages":[{"role":"system","content":"be terse"},{"role":"user","content":"hi"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("stream http %d", res.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	text := buf.String()
	if !strings.Contains(text, "chat.completion.chunk") {
		t.Fatalf("missing chunks: %.200s", text)
	}
	if !strings.Contains(text, "STUB") {
		t.Fatal("text deltas forwarded")
	}
	if !strings.Contains(text, "[DONE]") {
		t.Fatal("stream terminated with [DONE]")
	}
}

func TestSessionAffinity(t *testing.T) {
	setupBridge(t)
	resetSeen()
	doRequest(t, "POST", "/v1/chat/completions", map[string]any{"model": "glm-5.3-flash", "user": "conv-42", "messages": []any{map[string]any{"role": "user", "content": "a"}}})
	seen.mu.Lock()
	first := seen.anthropicHdr.Get("X-Session-Id")
	seen.mu.Unlock()
	doRequest(t, "POST", "/v1/chat/completions", map[string]any{"model": "glm-5.3-flash", "user": "conv-42", "messages": []any{map[string]any{"role": "user", "content": "b"}}})
	seen.mu.Lock()
	second := seen.anthropicHdr.Get("X-Session-Id")
	seen.mu.Unlock()
	if first != second {
		t.Fatalf("session id must be stable per conversation: %q vs %q", first, second)
	}
	doRequest(t, "POST", "/v1/chat/completions", map[string]any{"model": "glm-5.3-flash", "user": "conv-43", "messages": []any{map[string]any{"role": "user", "content": "b"}}})
	seen.mu.Lock()
	third := seen.anthropicHdr.Get("X-Session-Id")
	seen.mu.Unlock()
	if third == first {
		t.Fatal("a different conversation gets a different session")
	}
}

func TestToolRoundTrip(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, text, body := doRequest(t, "POST", "/v1/chat/completions", map[string]any{
		"model":    "glm-5.3-flash",
		"messages": []any{map[string]any{"role": "user", "content": "weather?"}},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{
			"name": "get_weather", "description": "d",
			"parameters": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		}}},
	})
	if status != 200 {
		t.Fatalf("status %d: %s", status, text)
	}
	choices := body["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	toolCalls, hasCalls := msg["tool_calls"].([]any)
	if !hasCalls || len(toolCalls) == 0 {
		raw, _ := json.Marshal(body)
		t.Fatalf("no tool_calls: %.300s", raw)
	}
	call := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if str(call["name"]) != "get_weather" {
		t.Fatalf("tool name = %v", call["name"])
	}
	var args map[string]any
	if json.Unmarshal([]byte(str(call["arguments"])), &args) != nil || args["city"] != "Jakarta" {
		t.Fatalf("arguments = %v", call["arguments"])
	}
	if str(choices[0].(map[string]any)["finish_reason"]) != "tool_calls" {
		t.Fatalf("finish_reason = %v", choices[0].(map[string]any)["finish_reason"])
	}
	seen.mu.Lock()
	tools := seen.anthropic["tools"].([]any)
	seen.mu.Unlock()
	if str(tools[0].(map[string]any)["input_schema"].(map[string]any)["type"]) != "object" {
		t.Fatal("tools translated to input_schema")
	}
	// Follow-up: role:"tool" must become a tool_result block inside a user turn.
	status, text, _ = doRequest(t, "POST", "/v1/chat/completions", map[string]any{
		"model": "glm-5.3-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "weather?"},
			map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": "toolu_stub", "type": "function", "function": map[string]any{"name": "get_weather", "arguments": `{"city":"Jakarta"}`}}}},
			map[string]any{"role": "tool", "tool_call_id": "toolu_stub", "content": "31C"},
			map[string]any{"role": "user", "content": "thanks"},
		},
	})
	if status != 200 {
		t.Fatalf("follow-up status %d: %s", status, text)
	}
	seen.mu.Lock()
	msgs := seen.anthropic["messages"].([]any)
	seen.mu.Unlock()
	found := false
	for _, m := range msgs {
		mm, ok := m.(map[string]any)
		if !ok || mm["role"] != "user" {
			continue
		}
		content, ok := mm["content"].([]any)
		if !ok {
			continue
		}
		for _, b := range content {
			if bm, ok := b.(map[string]any); ok && bm["type"] == "tool_result" {
				found = true
			}
		}
	}
	if !found {
		raw, _ := json.Marshal(msgs)
		t.Fatalf("no tool_result block: %.400s", raw)
	}
}

func TestModelsMergesEntitlements(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, _, body := doRequest(t, "GET", "/v1/models", nil)
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	data := body["data"].([]any)
	var flash, glm map[string]any
	for _, m := range data {
		mm := m.(map[string]any)
		switch mm["id"] {
		case "glm-5.3-flash":
			flash = mm
		case "glm-5.3":
			glm = mm
		}
	}
	if flash == nil || glm == nil {
		t.Fatalf("missing models: %v", body["data"])
	}
	if flash["plan_remaining_tokens"].(float64) != 500000000+4972073 {
		t.Fatalf("flash remaining = %v", flash["plan_remaining_tokens"])
	}
	if glm["plan_remaining_tokens"].(float64) != 2999978 {
		t.Fatalf("glm remaining = %v", glm["plan_remaining_tokens"])
	}
	seen.mu.Lock()
	balanceHdr := seen.balanceHdr
	currentSeen := seen.currentHdr != nil
	seen.mu.Unlock()
	if balanceHdr.Get("X-Device-Mid") == "" {
		t.Fatal("balance calls must carry X-Device-Mid")
	}
	if currentSeen {
		t.Fatal("healthy balance response must not trigger the current fallback")
	}
}

func TestQuotaPerPlanRows(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, text, body := doRequest(t, "GET", "/v1/quota", nil)
	if status != 200 {
		t.Fatalf("status %d: %s", status, text)
	}
	if body["source"] != "balance" {
		t.Fatalf("source = %v", body["source"])
	}
	rows := body["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	for _, r := range rows {
		row := r.(map[string]any)
		if row["plan_id"] == "wk" {
			if row["plan_name"] != "ZCODE WEEKEND BUILD" {
				t.Fatalf("wk plan_name = %v", row["plan_name"])
			}
			if row["total_units"].(float64) != 500000000 {
				t.Fatalf("wk total = %v", row["total_units"])
			}
		}
		if row["entitlement_id"] == "ent_53" {
			if row["period_start"].(float64) != 1793376000 || row["period_end"].(float64) != 1793462399 {
				t.Fatalf("daily window = %v..%v", row["period_start"], row["period_end"])
			}
		}
	}
	if errs, ok := body["errors"].([]any); ok && len(errs) != 0 {
		t.Fatalf("errors = %v", errs)
	}
}

func TestSignedHeadersNeverSent(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, text, _ := doRequest(t, "POST", "/v1/chat/completions", map[string]any{
		"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "x"}},
	})
	if status != 200 {
		t.Fatalf("default path must never sign: %d %s", status, text)
	}
}

func writeAccounts(t *testing.T, entries []map[string]any, def string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"version": 1, "default": def, "accounts": entries})
	accountsFile := os.Getenv("ZCODE_ACCOUNTS_FILE")
	if err := os.WriteFile(accountsFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	accounts.ResetForTest()
}

func day() int64 { return 86400000 }

func TestMultiAccountRotation(t *testing.T) {
	setupBridge(t)
	writeAccounts(t, []map[string]any{
		{"id": "spent", "label": "spent", "jwt": FAILJWT, "userId": "u_spent", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-spent", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
		{"id": "good", "label": "good", "jwt": JWT, "userId": "u_smoke", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-good", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
	}, "spent")
	resetSeen()
	before := len(seenBearers())
	status, text, _ := doRequest(t, "POST", "/v1/chat/completions", map[string]any{"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "rotate me"}}})
	if status != 200 {
		t.Fatalf("status %d: %s", status, text)
	}
	used := seenBearers()[before:]
	if len(used) != 2 {
		t.Fatalf("bearers = %v", used)
	}
	if used[0] != "Bearer "+FAILJWT {
		t.Fatal("default account tried first")
	}
	if used[1] != "Bearer "+JWT {
		t.Fatal("rotated to the healthy account after 1113")
	}
	status, _, health := doRequest(t, "GET", "/health", nil)
	if status != 200 {
		t.Fatal("health failed")
	}
	ring := health["accounts"].([]any)
	if len(ring) != 2 {
		t.Fatalf("health accounts = %d", len(ring))
	}
	var spent map[string]any
	for _, a := range ring {
		if str(a.(map[string]any)["id"]) == "spent" {
			spent = a.(map[string]any)
		}
	}
	if spent == nil || str(spent["cooldownUntil"]) == "" {
		t.Fatal("spent account shows its cooldown")
	}
}

func seenBearers() []string {
	seen.mu.Lock()
	defer seen.mu.Unlock()
	return append([]string(nil), seen.bearers...)
}

func TestRefreshOn401(t *testing.T) {
	setupBridge(t)
	writeAccounts(t, []map[string]any{
		{"id": "exp", "label": "exp", "jwt": EXPIRED, "refreshToken": "rt-old", "userId": "u_smoke", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-exp", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
		{"id": "spent", "label": "spent", "jwt": FAILJWT, "userId": "u_spent", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-spent", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
		{"id": "good", "label": "good", "jwt": JWT, "userId": "u_smoke", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-good", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
	}, "exp")
	resetSeen()
	before := len(seenBearers())
	status, text, _ := doRequest(t, "POST", "/v1/chat/completions", map[string]any{"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "refresh me"}}})
	if status != 200 {
		t.Fatalf("status %d: %s", status, text)
	}
	seen.mu.Lock()
	refreshCalls := seen.refreshCalls
	seen.mu.Unlock()
	if refreshCalls != 1 {
		t.Fatalf("token endpoint calls = %d", refreshCalls)
	}
	used := seenBearers()[before:]
	if len(used) != 2 {
		t.Fatalf("bearers = %v", used)
	}
	if used[0] != "Bearer "+EXPIRED {
		t.Fatal("expired token tried first")
	}
	if used[1] != "Bearer "+JWT {
		t.Fatal("retried with the refreshed token")
	}
	raw, err := os.ReadFile(os.Getenv("ZCODE_ACCOUNTS_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		t.Fatal("corrupt accounts file")
	}
	for _, a := range stored.Accounts {
		if a["id"] == "exp" {
			if a["jwt"] != JWT {
				t.Fatal("refreshed token persisted")
			}
			if a["refreshToken"] != "rt-new" {
				t.Fatal("rotated refresh token persisted")
			}
		}
	}
}

func TestAccountPinnedSkipsRotation(t *testing.T) {
	setupBridge(t)
	writeAccounts(t, []map[string]any{
		{"id": "spent", "label": "spent", "jwt": FAILJWT, "userId": "u_spent", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-spent", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
		{"id": "good", "label": "good", "jwt": JWT, "userId": "u_smoke", "deviceId": "0a7c2d9d-ce62-4456-aa65-625511ff1ef0", "sessionSeed": "seed-good", "expiresAt": time.Now().UnixMilli() + day(), "needsReauth": false},
	}, "spent")
	resetSeen()
	status, _, _ := doRequest(t, "POST", "/v1/chat/completions?account=spent", map[string]any{"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "pinned"}}})
	if status != 429 {
		t.Fatalf("spent account answers its own error when pinned: %d", status)
	}
}

func TestInspectCapturesExchange(t *testing.T) {
	setupBridge(t)
	resetSeen()
	capture.Clear()
	status, text, _ := doRequest(t, "POST", "/v1/chat/completions", map[string]any{
		"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "capture me"}},
	})
	if status != 200 {
		t.Fatalf("chat status %d: %s", status, text)
	}
	code, raw, body := doRequest(t, "GET", "/inspect?limit=10", nil)
	if code != 200 {
		t.Fatalf("inspect status %d: %s", code, raw)
	}
	if body["enabled"] != true {
		t.Fatalf("inspect enabled = %v", body["enabled"])
	}
	entries := body["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	entry := entries[0].(map[string]any)
	if str(entry["route"]) != "chat.completions" || entry["status"].(float64) != 200 {
		t.Fatalf("summary = %v", entry)
	}
	if str(entry["model"]) != "glm-5.3-flash" {
		t.Fatalf("model = %v", entry["model"])
	}
	if str(entry["account"]) == "" {
		t.Fatalf("account attribution missing: %v", entry)
	}
	if _, hasBody := entry["request_body"]; hasBody {
		t.Fatal("the listing must not carry bodies")
	}
	id := int64(entry["id"].(float64))
	code, raw, detail := doRequest(t, "GET", "/inspect?id="+fmt.Sprint(id), nil)
	if code != 200 {
		t.Fatalf("detail status %d: %s", code, raw)
	}
	if !strings.Contains(str(detail["request_body"]), "capture me") {
		t.Fatalf("request body = %q", detail["request_body"])
	}
	if !strings.Contains(str(detail["response_body"]), "STUB-OK") {
		t.Fatalf("response body = %q", detail["response_body"])
	}
	events, _ := detail["events"].([]any)
	if len(events) == 0 {
		t.Fatal("upstream attempt not recorded")
	}
	if code, _, _ := doRequest(t, "GET", "/inspect?id=99999", nil); code != 404 {
		t.Fatalf("unknown id status = %d", code)
	}
}

func TestInspectHARAndReplay(t *testing.T) {
	setupBridge(t)
	resetSeen()
	capture.Clear()
	status, text, _ := doRequest(t, "POST", "/v1/chat/completions", map[string]any{
		"model": "glm-5.3-flash", "messages": []any{map[string]any{"role": "user", "content": "replay me"}},
	})
	if status != 200 {
		t.Fatalf("chat status %d: %s", status, text)
	}
	_, _, listing := doRequest(t, "GET", "/inspect?limit=10", nil)
	entries := listing["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	id := int64(entries[0].(map[string]any)["id"].(float64))

	code, raw, har := doRequest(t, "GET", "/inspect?format=har", nil)
	if code != 200 {
		t.Fatalf("har status %d: %s", code, raw)
	}
	log, _ := har["log"].(map[string]any)
	if log == nil || str(log["version"]) != "1.2" {
		t.Fatalf("har log = %v", har["log"])
	}
	harEntries, _ := log["entries"].([]any)
	if len(harEntries) != 1 {
		t.Fatalf("har entries = %d", len(harEntries))
	}
	first := harEntries[0].(map[string]any)
	req, _ := first["request"].(map[string]any)
	if req == nil || str(req["method"]) != "POST" || !strings.Contains(str(req["url"]), "/v1/chat/completions") {
		t.Fatalf("har request = %v", first["request"])
	}
	resp, _ := first["response"].(map[string]any)
	if resp == nil || resp["status"].(float64) != 200 {
		t.Fatalf("har response = %v", first["response"])
	}

	code, raw, replayed := doRequest(t, "POST", "/inspect/replay?id="+fmt.Sprint(id), nil)
	if code != 200 {
		t.Fatalf("replay status %d: %s", code, raw)
	}
	if replayed["ok"] != true || replayed["status"].(float64) != 200 {
		t.Fatalf("replay payload = %v", replayed)
	}
	if !strings.Contains(str(replayed["body"]), "STUB-OK") {
		t.Fatalf("replay body = %q", replayed["body"])
	}
	_, _, after := doRequest(t, "GET", "/inspect?limit=10", nil)
	if got := len(after["entries"].([]any)); got != 2 {
		t.Fatalf("replay was not captured: %d entries", got)
	}
}

func TestAccountsListAndSwitch(t *testing.T) {
	setupBridge(t)
	code, raw, body := doRequest(t, "GET", "/v1/accounts", nil)
	if code != 200 {
		t.Fatalf("accounts status %d: %s", code, raw)
	}
	if body["ok"] != true {
		t.Fatalf("accounts payload = %v", body)
	}
	if _, ok := body["accounts"].([]any); !ok && body["accounts"] != nil {
		t.Fatalf("accounts = %v", body["accounts"])
	}
	code, raw, body = doRequest(t, "POST", "/v1/accounts/switch", map[string]any{"id": "missing-account"})
	if code != 404 {
		t.Fatalf("switch missing status %d: %s", code, raw)
	}
	if body["ok"] != false {
		t.Fatalf("switch payload = %v", body)
	}
	code, _, _ = doRequest(t, "POST", "/v1/accounts/switch", map[string]any{})
	if code != 400 {
		t.Fatalf("switch without id status = %d", code)
	}
}

func TestAsyncAndClaimRoutes(t *testing.T) {
	setupBridge(t)
	t.Setenv("ZCODE_ASYNC_ENABLED", "")
	code, raw, body := doRequest(t, "GET", "/async/v1/health", nil)
	if code != 400 {
		t.Fatalf("async health disabled status %d: %s", code, raw)
	}
	if body["enabled"] != false {
		t.Fatalf("async health payload = %v", body)
	}
	t.Setenv("ZCODE_ASYNC_ENABLED", "1")
	code, raw, _ = doRequest(t, "GET", "/async/v1/health", nil)
	if code != 502 {
		t.Fatalf("async health without jwt status %d: %s", code, raw)
	}
	t.Setenv("ZCODE_CLAIM_ORIGIN", "http://127.0.0.1:1")
	code, raw, _ = doRequest(t, "GET", "/v1/claim", nil)
	if code != 502 {
		t.Fatalf("claim list unreachable origin status %d: %s", code, raw)
	}
}

func TestInspectCapturesStreamFlags(t *testing.T) {
	setupBridge(t)
	resetSeen()
	capture.Clear()
	req, _ := http.NewRequest("POST", bridgeURL()+"/v1/chat/completions", bytes.NewReader([]byte(
		`{"model":"glm-5.3-flash","stream":true,"messages":[{"role":"user","content":"stream"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()

	_, _, body := doRequest(t, "GET", "/inspect?limit=5", nil)
	entries := body["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	entry := entries[0].(map[string]any)
	if entry["stream"] != true {
		t.Fatalf("stream flag = %v", entry["stream"])
	}
	if str(entry["path"]) != "/v1/chat/completions" {
		t.Fatalf("path = %v", entry["path"])
	}
}

func TestInspectTokenGate(t *testing.T) {
	setupBridge(t)
	previous := cfg.BridgeToken
	cfg.BridgeToken = "inspect-token"
	defer func() { cfg.BridgeToken = previous }()

	if code, _, _ := doRequest(t, "GET", "/inspect", nil); code != 401 {
		t.Fatalf("unauthenticated inspect = %d", code)
	}
	req, _ := http.NewRequest("GET", bridgeURL()+"/inspect", nil)
	req.Header.Set("Authorization", "Bearer inspect-token")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("authenticated inspect = %d", res.StatusCode)
	}
}

func sseEvents(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, frame := range strings.Split(raw, "\n\n") {
		var payload string
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data:") {
				payload = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			t.Fatalf("bad SSE payload %q: %v", payload, err)
		}
		events = append(events, event)
	}
	return events
}

func TestResponsesNonStream(t *testing.T) {
	setupBridge(t)
	resetSeen()
	status, text, body := doRequest(t, "POST", "/v1/responses", map[string]any{
		"model": "glm-5.3-flash",
		"input": "halo",
	})
	if status != 200 {
		t.Fatalf("responses status %d: %s", status, text)
	}
	if body["object"] != "response" || body["status"] != "completed" {
		t.Fatalf("envelope = %s", text)
	}
	if body["output_text"] != "STUB-OK" {
		t.Fatalf("output_text = %v", body["output_text"])
	}
	output := body["output"].([]any)
	item := output[0].(map[string]any)
	if item["type"] != "message" || item["role"] != "assistant" {
		t.Fatalf("output item = %v", item)
	}
	content := item["content"].([]any)
	if content[0].(map[string]any)["text"] != "STUB-OK" {
		t.Fatalf("content = %v", content)
	}
	usage := body["usage"].(map[string]any)
	if usage["total_tokens"].(float64) != 18 {
		t.Fatalf("usage = %v", usage)
	}
	if _, ok := usage["input_tokens_details"].(map[string]any); !ok {
		t.Fatalf("usage details missing: %v", usage)
	}
}

func TestResponsesStreamEvents(t *testing.T) {
	setupBridge(t)
	resetSeen()
	req, _ := http.NewRequest("POST", bridgeURL()+"/v1/responses", bytes.NewReader([]byte(
		`{"model":"glm-5.3-flash","stream":true,"input":"halo"}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("stream status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	raw, _ := io.ReadAll(res.Body)
	events := sseEvents(t, string(raw))
	if len(events) < 6 {
		t.Fatalf("events = %d: %s", len(events), raw)
	}
	var types []string
	text := ""
	for _, event := range events {
		typ := str(event["type"])
		types = append(types, typ)
		switch typ {
		case "response.output_text.delta":
			text += str(event["delta"])
		case "response.completed":
			response := event["response"].(map[string]any)
			if response["status"] != "completed" {
				t.Fatalf("completed status = %v", response["status"])
			}
		}
	}
	want := []string{
		"response.created",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event types:\n got %v\nwant %v", types, want)
	}
	if text != "STUB-OK" {
		t.Fatalf("streamed text = %q", text)
	}
	if types[len(types)-1] != "response.completed" {
		t.Fatalf("last event = %s", types[len(types)-1])
	}
}

func TestResponsesCustomToolMapping(t *testing.T) {
	setupBridge(t)
	resetSeen()
	reqBody := map[string]any{
		"model":  "glm-5.3-flash",
		"stream": true,
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "cuaca?"},
			map[string]any{"type": "function_call", "call_id": "call_prev", "name": "get_weather", "arguments": `{"city":"Bandung"}`},
			map[string]any{"type": "function_call_output", "call_id": "call_prev", "output": "rainy"},
		},
		"tools": []any{
			map[string]any{"type": "custom", "name": "get_weather", "description": "weather probe"},
		},
	}
	raw, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", bridgeURL()+"/v1/responses", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	events := sseEvents(t, string(body))
	var sawAdded, sawInputDone, sawItemDone bool
	var finalItem map[string]any
	for _, event := range events {
		switch str(event["type"]) {
		case "response.output_item.added":
			item := event["item"].(map[string]any)
			if item["type"] == "custom_tool_call" {
				sawAdded = true
			}
		case "response.custom_tool_call_input.done":
			if str(event["input"]) != `{"city":"Jakarta"}` {
				t.Fatalf("custom input = %v", event["input"])
			}
			sawInputDone = true
		case "response.output_item.done":
			item := event["item"].(map[string]any)
			if item["type"] == "custom_tool_call" {
				sawItemDone = true
				finalItem = item
			}
		}
	}
	if !sawAdded || !sawInputDone || !sawItemDone {
		t.Fatalf("custom tool lifecycle incomplete: added=%v inputDone=%v itemDone=%v", sawAdded, sawInputDone, sawItemDone)
	}
	if finalItem["name"] != "get_weather" || finalItem["status"] != "completed" {
		t.Fatalf("final custom item = %v", finalItem)
	}
	// The history must have reached upstream as a tool_result block.
	seen.mu.Lock()
	defer seen.mu.Unlock()
	wire, _ := json.Marshal(seen.anthropic)
	if !strings.Contains(string(wire), "tool_result") {
		t.Fatalf("history not translated: %v", seen.anthropic["messages"])
	}
}

func TestUsageEndpointAggregatesStreamedRequest(t *testing.T) {
	setupBridge(t)
	usage.ResetForTest(t.TempDir())
	resetSeen()
	req, _ := http.NewRequest("POST", bridgeURL()+"/v1/chat/completions", bytes.NewReader([]byte(
		`{"model":"glm-5.3-flash","stream":true,"messages":[{"role":"user","content":"halo"}]}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("chat status = %d", res.StatusCode)
	}

	status, text, body := doRequest(t, "GET", "/v1/usage", nil)
	if status != 200 {
		t.Fatalf("usage status %d: %s", status, text)
	}
	today := body["today"].(map[string]any)
	if today["requests"].(float64) != 1 {
		t.Fatalf("requests = %v", today["requests"])
	}
	if today["input_tokens"].(float64) != 10 || today["output_tokens"].(float64) != 5 {
		t.Fatalf("tokens = %v / %v", today["input_tokens"], today["output_tokens"])
	}
	perModel := today["per_model"].(map[string]any)
	if _, ok := perModel["glm-5.3-flash"]; !ok {
		t.Fatalf("per_model = %v", perModel)
	}
	recent := body["recent"].([]any)
	if len(recent) != 1 {
		t.Fatalf("recent = %v", recent)
	}
	row := recent[0].(map[string]any)
	if row["model"] != "glm-5.3-flash" || row["stream"] != true {
		t.Fatalf("recent row = %v", row)
	}
	if allTime := body["all_time"].(map[string]any); allTime["requests"].(float64) != 1 {
		t.Fatalf("all_time = %v", allTime)
	}
}

func TestUsageEndpointTokenGate(t *testing.T) {
	setupBridge(t)
	previous := cfg.BridgeToken
	cfg.BridgeToken = "usage-token"
	defer func() { cfg.BridgeToken = previous }()

	if code, _, _ := doRequest(t, "GET", "/v1/usage", nil); code != 401 {
		t.Fatalf("unauthenticated usage = %d", code)
	}
	req, _ := http.NewRequest("GET", bridgeURL()+"/v1/usage", nil)
	req.Header.Set("Authorization", "Bearer usage-token")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("authenticated usage = %d", res.StatusCode)
	}
}

func TestOverlayServesSpeedReadout(t *testing.T) {
	setupBridge(t)
	usage.ResetForTest(t.TempDir())
	res, err := http.Get(bridgeURL() + "/overlay")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("overlay status = %d", res.StatusCode)
	}
	raw, _ := io.ReadAll(res.Body)
	page := string(raw)
	for _, want := range []string{"tok/dtk", "/v1/usage", "rata-rata hari ini", "data-mode=\"capsule\"", "zbOverlayMode", "id=\"arc\""} {
		if !strings.Contains(page, want) {
			t.Fatalf("overlay page missing %q", want)
		}
	}
	for _, path := range []string{"/overlay/gauge", "/overlay/mini", "/overlay/capsule"} {
		res, err := http.Get(bridgeURL() + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s status = %d", path, res.StatusCode)
		}
	}
}

func TestUsageEndpointIncludesDesktop(t *testing.T) {
	setupBridge(t)
	usage.ResetForTest(t.TempDir())
	t.Setenv("ZCODE_DB", filepath.Join("..", "sqlite", "testdata", "usage.db"))
	t.Setenv("ZCODE_DESKTOP", "")
	resetDesktopForTest()
	t.Cleanup(resetDesktopForTest)

	status, text, body := doRequest(t, "GET", "/v1/usage", nil)
	if status != 200 {
		t.Fatalf("usage status %d: %s", status, text)
	}
	block, ok := body["desktop"].(map[string]any)
	if !ok {
		t.Fatalf("desktop block missing: %v", body)
	}
	if block["available"] != true {
		t.Fatalf("desktop unavailable: %v", block)
	}
	snap := block["snapshot"].(map[string]any)
	all := snap["all_time"].(map[string]any)
	if all["requests"].(float64) != 2 {
		t.Fatalf("desktop all-time requests = %v", all["requests"])
	}
	models := snap["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("desktop models = %v", models)
	}
	ranges := snap["ranges"].([]any)
	if len(ranges) != 4 {
		t.Fatalf("desktop ranges = %v", ranges)
	}
	if got := len(ranges[0].(map[string]any)["buckets"].([]any)); got != 90 {
		t.Fatalf("15m buckets = %d", got)
	}
}

func TestUsageEndpointDesktopUnavailable(t *testing.T) {
	setupBridge(t)
	usage.ResetForTest(t.TempDir())
	t.Setenv("ZCODE_DB", filepath.Join(t.TempDir(), "missing.sqlite"))
	t.Setenv("ZCODE_DESKTOP", "")
	resetDesktopForTest()
	t.Cleanup(resetDesktopForTest)

	status, text, body := doRequest(t, "GET", "/v1/usage", nil)
	if status != 200 {
		t.Fatalf("usage status %d: %s", status, text)
	}
	block := body["desktop"].(map[string]any)
	if block["available"] != false {
		t.Fatalf("desktop should be unavailable: %v", block)
	}
}
