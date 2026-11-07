package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeMCPUpstream speaks just enough MCP over plain JSON HTTP for the bridge's
// HTTP transport: initialize, tools/list and tools/call.
func fakeMCPUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		if len(msg.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch msg.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]any{"name": "upstream", "version": "9.9"},
			}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{
				"name":        "weather",
				"description": "look up weather",
				"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			}}}
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			result = map[string]any{"content": []map[string]any{{
				"type": "text",
				"text": "sunny in " + string(params.Arguments),
			}}}
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{"code": -32601, "message": "no " + msg.Method}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func useMCPConfig(t *testing.T, config string) {
	t.Helper()
	t.Setenv("ZCODE_MCP_SERVERS", config)
	resetMCPForTest()
	t.Cleanup(resetMCPForTest)
}

func TestMCPEndpointAggregatesAndCalls(t *testing.T) {
	setupBridge(t)
	upstream := fakeMCPUpstream(t)
	useMCPConfig(t, fmt.Sprintf(`{"mcpServers":{"demo":{"url":%q}}}`, upstream.URL))

	status, text, body := doRequest(t, "POST", "/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"}},
	})
	if status != 200 {
		t.Fatalf("initialize status %d: %s", status, text)
	}
	result := body["result"].(map[string]any)
	if result["protocolVersion"] != "2025-03-26" {
		t.Fatalf("protocol version not echoed: %v", result)
	}
	info := result["serverInfo"].(map[string]any)
	if info["name"] != "zcode-bridge" {
		t.Fatalf("serverInfo: %v", info)
	}

	status, text, body = doRequest(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	if status != 200 {
		t.Fatalf("tools/list status %d: %s", status, text)
	}
	tools := body["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %d: %s", len(tools), text)
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "demo__weather" {
		t.Fatalf("tool name = %v", tool["name"])
	}
	if tool["description"] != "look up weather" {
		t.Fatalf("tool description = %v", tool["description"])
	}

	status, text, body = doRequest(t, "POST", "/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "demo__weather", "arguments": map[string]any{"city": "Jakarta"}},
	})
	if status != 200 {
		t.Fatalf("tools/call status %d: %s", status, text)
	}
	callResult := body["result"].(map[string]any)
	content := callResult["content"].([]any)
	if len(content) != 1 || !strings.Contains(content[0].(map[string]any)["text"].(string), "Jakarta") {
		t.Fatalf("call content: %v", callResult)
	}

	status, text, body = doRequest(t, "POST", "/mcp", map[string]any{
		"jsonrpc": "2.0", "id": 4, "method": "tools/call",
		"params": map[string]any{"name": "demo__missing", "arguments": map[string]any{}},
	})
	if status != 200 || body["result"].(map[string]any)["isError"] != true {
		t.Fatalf("unknown tool should be an isError result: %d %s", status, text)
	}

	// A notification gets no JSON-RPC body.
	status, _, _ = doRequest(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	if status != http.StatusAccepted {
		t.Fatalf("notification status = %d, want 202", status)
	}

	status, text, body = doRequest(t, "GET", "/mcp?format=json", nil)
	if status != 200 {
		t.Fatalf("catalog status %d: %s", status, text)
	}
	servers := body["servers"].([]any)
	if len(servers) != 1 || servers[0].(map[string]any)["name"] != "demo" {
		t.Fatalf("catalog servers: %v", servers)
	}
	if servers[0].(map[string]any)["tools"].(float64) != 1 {
		t.Fatalf("catalog tool count: %v", servers[0])
	}
	client := body["client"].(map[string]any)["mcpServers"].(map[string]any)["zcode-bridge"].(map[string]any)
	if !strings.HasSuffix(client["url"].(string), "/mcp") {
		t.Fatalf("client url: %v", client)
	}

	status, text, _ = doRequest(t, "GET", "/mcp", nil)
	if status != 200 || !strings.Contains(text, "demo__weather") {
		t.Fatalf("html catalog missing tool: %s", text)
	}
}

func TestMCPEndpointWithoutServers(t *testing.T) {
	setupBridge(t)
	useMCPConfig(t, "")

	status, text, body := doRequest(t, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	if status != 200 {
		t.Fatalf("tools/list status %d: %s", status, text)
	}
	if tools := body["result"].(map[string]any)["tools"].([]any); len(tools) != 0 {
		t.Fatalf("expected empty tool list: %s", text)
	}

	status, text, _ = doRequest(t, "GET", "/mcp", nil)
	if status != 200 || !strings.Contains(text, "No servers configured") {
		t.Fatalf("empty catalog: %s", text)
	}

	status, _, _ = doRequest(t, "POST", "/mcp", nil)
	if status == 200 {
		t.Fatal("empty body should not be treated as JSON-RPC")
	}
}
