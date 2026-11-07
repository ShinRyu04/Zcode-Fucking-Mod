package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type agentKeyRecord struct {
	Key       string `json:"key"`
	CreatedAt string `json:"createdAt"`
}

// agentKeyFile is the downstream key the bridge accepts when
// ZCODE_BRIDGE_TOKEN is not set. The env var always wins and is read-only.
func agentKeyFile() string {
	if f := strings.TrimSpace(os.Getenv("ZCODE_AGENT_KEY_FILE")); f != "" {
		return f
	}
	return filepath.Join("data", "agent-key.json")
}

func loadAgentKey() string {
	raw, err := os.ReadFile(agentKeyFile())
	if err != nil {
		return ""
	}
	var rec agentKeyRecord
	if json.Unmarshal(raw, &rec) != nil {
		return ""
	}
	return strings.TrimSpace(rec.Key)
}

func writeAgentKey(key string) error {
	path := agentKeyFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(agentKeyRecord{Key: key, CreatedAt: time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func newAgentKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(b), nil
}

func effectiveBridgeToken() string {
	if cfg.BridgeToken != "" {
		return cfg.BridgeToken
	}
	return loadAgentKey()
}

func keySource() string {
	if cfg.BridgeToken != "" {
		return "env"
	}
	if loadAgentKey() != "" {
		return "file"
	}
	return "none"
}

func maskKey(key string) string {
	if len(key) <= 10 {
		return key
	}
	return key[:5] + "..." + key[len(key)-4:]
}

// handleIntegration reports the downstream connection details. The full key is
// included only after requireObservabilityToken passed, which means the caller
// already presented it (or the bridge is open on loopback and has no key).
func handleIntegration(w http.ResponseWriter, r *http.Request) {
	key := effectiveBridgeToken()
	source := keySource()
	host := cfg.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	base := "http://" + host + ":" + strconv.Itoa(cfg.Port)
	defaultModel := strings.TrimSpace(os.Getenv("ZCODE_DEFAULT_MODEL"))
	if defaultModel == "" {
		defaultModel = "glm-5.3-flash"
	}
	out := map[string]any{
		"ok":            true,
		"base_url":      base,
		"key":           key,
		"masked_key":    maskKey(key),
		"key_present":   key != "",
		"key_source":    source,
		"editable":      source != "env",
		"default_model": defaultModel,
		"endpoints": map[string]string{
			"chat":      base + "/v1/chat/completions",
			"messages":  base + "/v1/messages",
			"responses": base + "/v1/responses",
			"models":    base + "/v1/models",
		},
	}
	sendJSON(w, 200, out)
}

// handleIntegrationKey generates or rotates the persisted downstream key.
func handleIntegrationKey(w http.ResponseWriter, r *http.Request) {
	if cfg.BridgeToken != "" {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "ZCODE_BRIDGE_TOKEN is set; rotate it in the environment"})
		return
	}
	key, err := newAgentKey()
	if err != nil {
		sendJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := writeAgentKey(key); err != nil {
		sendJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true, "key": key, "masked_key": maskKey(key), "key_source": "file"})
}

// handleIntegrationKeyDelete removes the persisted key, returning the bridge to
// open-on-loopback. The environment token cannot be removed from here.
func handleIntegrationKeyDelete(w http.ResponseWriter, r *http.Request) {
	if cfg.BridgeToken != "" {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "ZCODE_BRIDGE_TOKEN is set; unset it in the environment"})
		return
	}
	if err := os.Remove(agentKeyFile()); err != nil && !os.IsNotExist(err) {
		sendJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

func tokenEqual(presented, expected string) bool {
	if presented == "" || expected == "" {
		return presented == expected
	}
	return len(presented) == len(expected) && subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}
