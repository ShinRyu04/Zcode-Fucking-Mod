package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain doubles as a fake MCP server: when the test binary is re-executed
// with MCP_FAKE=1 it serves stdio JSON-RPC instead of running tests.
func TestMain(m *testing.M) {
	if os.Getenv("MCP_FAKE") == "1" {
		fakeServer()
		return
	}
	os.Exit(m.Run())
}

func fakeServer() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	enc := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var msg wireMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Method == "" || msg.isNotification() {
			continue
		}
		var res any
		switch msg.Method {
		case "initialize":
			res = map[string]any{
				"protocolVersion": ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fake", "version": "0.1"},
			}
		case "tools/list":
			var params struct{ Cursor string }
			_ = json.Unmarshal(msg.Params, &params)
			if params.Cursor == "page2" {
				res = map[string]any{"tools": []map[string]any{{"name": "second.tool", "description": "page two", "inputSchema": map[string]any{"type": "object"}}}}
			} else {
				res = map[string]any{
					"tools": []map[string]any{{
						"name":        "echo",
						"description": "echo back",
						"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
					}},
					"nextCursor": "page2",
				}
			}
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			res = map[string]any{
				"content": []map[string]any{{"type": "text", "text": "called " + params.Name + " with " + string(params.Arguments)}},
			}
		default:
			_ = enc.Encode(wireMessage{JSONRPC: "2.0", ID: msg.ID, Error: &Error{Code: -32601, Message: "unknown " + msg.Method}})
			continue
		}
		_ = enc.Encode(wireMessage{JSONRPC: "2.0", ID: msg.ID, Result: mustJSON(res)})
	}
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

func fakeConfig(t *testing.T, extra ...ServerConfig) *Config {
	t.Helper()
	cfg := &Config{Servers: []ServerConfig{{
		Name:    "fake",
		Command: os.Args[0],
		Env:     map[string]string{"MCP_FAKE": "1"},
	}}}
	cfg.Servers = append(cfg.Servers, extra...)
	return cfg
}

func TestRegistryStartsStdioServerAndListsTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reg := Start(ctx, fakeConfig(t))
	defer reg.Close()

	tools := reg.Tools()
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2 (pagination should be followed): %+v", len(tools), tools)
	}
	if tools[0].Name != "fake__echo" || tools[0].Server != "fake" || tools[0].Original != "echo" {
		t.Fatalf("unexpected first tool: %+v", tools[0])
	}
	if tools[1].Name != "fake__second_tool" {
		t.Fatalf("namespacing: %q", tools[1].Name)
	}

	raw, err := reg.Call(ctx, "fake__echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("result: %v (%s)", err, raw)
	}
	if len(res.Content) != 1 || res.Content[0].Text != `called echo with {"text":"hi"}` {
		t.Fatalf("unexpected call result: %s", raw)
	}

	status := reg.Status()
	if len(status) != 1 || status[0].Tools != 2 || status[0].Error != "" {
		t.Fatalf("status: %+v", status)
	}
}

func TestRegistryReportsFailingServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg := fakeConfig(t, ServerConfig{Name: "missing", Command: "definitely-not-a-real-binary-xyz"})
	reg := Start(ctx, cfg)
	defer reg.Close()

	if got := len(reg.Tools()); got != 2 {
		t.Fatalf("tools from healthy server = %d, want 2", got)
	}
	var failed *ServerStatus
	for _, st := range reg.Status() {
		if st.Name == "missing" {
			cp := st
			failed = &cp
		}
	}
	if failed == nil || failed.Error == "" {
		t.Fatalf("missing server not reported as failed: %+v", reg.Status())
	}
	if _, err := reg.Call(ctx, "nope__tool", nil); err == nil {
		t.Fatal("unknown tool should fail")
	}
}

func TestNamespacing(t *testing.T) {
	cases := []struct{ server, tool, want string }{
		{"zcode", "search", "zcode__search"},
		{"my server", "read.file", "my_server__read_file"},
		{"srv", "a/b:c", "srv__a_b_c"},
	}
	for _, c := range cases {
		if got := namespace(c.server, c.tool); got != c.want {
			t.Fatalf("namespace(%q,%q) = %q, want %q", c.server, c.tool, got, c.want)
		}
	}
	long := namespace(strings.Repeat("s", 40), strings.Repeat("t", 40))
	if len(long) != 64 {
		t.Fatalf("long name length = %d, want 64: %q", len(long), long)
	}
	if again := namespace(strings.Repeat("s", 40), strings.Repeat("t", 40)); again != long {
		t.Fatal("namespacing must be deterministic")
	}
}

func TestParseConfigShapes(t *testing.T) {
	claude := []byte(`{"mcpServers":{"b":{"command":"x"},"a":{"url":"http://h/mcp"}}}`)
	cfg, err := ParseConfig(claude)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 2 || cfg.Servers[0].Name != "a" || cfg.Servers[1].Name != "b" {
		t.Fatalf("map form: %+v", cfg.Servers)
	}
	list, err := ParseConfig([]byte(`[{"name":"one","command":"y"}]`))
	if err != nil || len(list.Servers) != 1 || list.Servers[0].Name != "one" {
		t.Fatalf("array form: %+v %v", list, err)
	}
	empty, err := LoadConfig("")
	if err != nil || len(empty.Servers) != 0 {
		t.Fatalf("empty config: %+v %v", empty, err)
	}
}

func TestHTTPTransportStreamableSSE(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		var msg wireMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if msg.isNotification() {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-1")
		w.Header().Set("Content-Type", "text/event-stream")
		calls++
		writeEvent := func(m wireMessage) {
			data, _ := json.Marshal(m)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
		}
		// an unrelated notification first, exercising the skip path
		writeEvent(wireMessage{JSONRPC: "2.0", Method: "notifications/progress", Params: mustJSON(map[string]any{"progress": 1})})
		var res any
		switch msg.Method {
		case "initialize":
			res = map[string]any{"protocolVersion": ProtocolVersion, "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "web"}}
		case "tools/list":
			res = map[string]any{"tools": []map[string]any{{"name": "web.tool", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			res = map[string]any{"content": []map[string]any{{"type": "text", "text": "web ok"}}}
		}
		writeEvent(wireMessage{JSONRPC: "2.0", ID: msg.ID, Result: mustJSON(res)})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := NewClient(ctx, "web", HTTPTransport(srv.URL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if len(client.Tools()) != 1 || client.Tools()[0].Name != "web.tool" {
		t.Fatalf("tools: %+v", client.Tools())
	}
	raw, err := client.Call(ctx, "web.tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "web ok") {
		t.Fatalf("call result: %s", raw)
	}
	if calls != 3 {
		t.Fatalf("requests = %d, want 3 (initialize, tools/list, tools/call)", calls)
	}
}
