package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func integrationRequest(t *testing.T, method, path, bearer string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, bridgeURL()+path, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// TestAgentKeyLifecycle covers the downstream key contract: generate, require
// it on inference routes, then delete it and return to open loopback.
func TestAgentKeyLifecycle(t *testing.T) {
	setupBridge(t)
	path := filepath.Join(t.TempDir(), "agent-key.json")
	t.Setenv("ZCODE_AGENT_KEY_FILE", path)

	code, body := integrationRequest(t, http.MethodGet, "/v1/integration", "")
	if code != 200 || body["key_source"] != "none" || body["key_present"] != false {
		t.Fatalf("initial integration = %d %v", code, body)
	}

	code, body = integrationRequest(t, http.MethodPost, "/v1/integration/key", "")
	if code != 200 {
		t.Fatalf("generate key = %d %v", code, body)
	}
	key, _ := body["key"].(string)
	if key == "" {
		t.Fatal("generated key is empty")
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("key file = %v, %v", info, err)
	}

	code, _ = integrationRequest(t, http.MethodGet, "/v1/settings", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("settings without key = %d, want 401", code)
	}
	code, _ = integrationRequest(t, http.MethodGet, "/v1/settings", key)
	if code != 200 {
		t.Fatalf("settings with key = %d, want 200", code)
	}

	code, body = integrationRequest(t, http.MethodDelete, "/v1/integration/key", key)
	if code != 200 {
		t.Fatalf("delete key = %d %v", code, body)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("key file still present: %v", err)
	}
	code, _ = integrationRequest(t, http.MethodGet, "/v1/settings", "")
	if code != 200 {
		t.Fatalf("settings after delete = %d, want 200", code)
	}
}

// TestAgentKeyEnvWins keeps the environment token authoritative and read-only.
func TestAgentKeyEnvWins(t *testing.T) {
	setupBridge(t)
	path := filepath.Join(t.TempDir(), "agent-key.json")
	t.Setenv("ZCODE_AGENT_KEY_FILE", path)

	prev := cfg
	Configure(Config{Port: 0, Host: "127.0.0.1", BridgeToken: "env-token-test", ModelsCache: prev.ModelsCache})
	defer Configure(prev)

	code, body := integrationRequest(t, http.MethodGet, "/v1/integration", "env-token-test")
	if code != 200 || body["key_source"] != "env" || body["editable"] != false {
		t.Fatalf("env integration = %d %v", code, body)
	}
	code, _ = integrationRequest(t, http.MethodPost, "/v1/integration/key", "env-token-test")
	if code != http.StatusConflict {
		t.Fatalf("rotate under env = %d, want 409", code)
	}
	code, _ = integrationRequest(t, http.MethodDelete, "/v1/integration/key", "env-token-test")
	if code != http.StatusConflict {
		t.Fatalf("delete under env = %d, want 409", code)
	}
}
