package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/mcp"
)

// The bridge speaks MCP as a server: clients point at POST /mcp and see every
// tool from every configured upstream server under one flat, namespaced list.
// Upstreams come from ZCODE_MCP_SERVERS (path or inline JSON). Nothing runs
// until the first MCP request arrives, so an unconfigured bridge costs
// nothing.

var (
	mcpOnce     sync.Once
	mcpRegistry *mcp.Registry
	mcpInitErr  error
)

func mcpServers() *mcp.Registry {
	mcpOnce.Do(func() {
		cfg, err := mcp.LoadConfig(os.Getenv("ZCODE_MCP_SERVERS"))
		if err != nil {
			mcpInitErr = err
			mcpRegistry = mcp.Start(context.Background(), &mcp.Config{})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		mcpRegistry = mcp.Start(ctx, cfg)
	})
	return mcpRegistry
}

func resetMCPForTest() {
	mcpOnce = sync.Once{}
	if mcpRegistry != nil {
		mcpRegistry.Close()
	}
	mcpRegistry = nil
	mcpInitErr = nil
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// handleMCP serves the Model Context Protocol endpoint.
//
//	POST /mcp  JSON-RPC 2.0 (initialize, tools/list, tools/call, ping)
//	GET  /mcp  catalog: upstream servers, flattened tools, client config
func handleMCP(w http.ResponseWriter, r *http.Request) {
	if !requireObservabilityToken(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		mcpCatalog(w, r)
	case http.MethodPost:
		mcpDispatch(w, r)
	case http.MethodDelete:
		// Streamable HTTP session teardown; the registry is process-wide.
		w.WriteHeader(http.StatusOK)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func mcpDispatch(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		mcpWrite(w, http.StatusBadRequest, map[string]any{"jsonrpc": "2.0", "id": nil, "error": jsonrpcError{Code: -32700, Message: "failed to read body"}})
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		mcpWrite(w, http.StatusBadRequest, map[string]any{"jsonrpc": "2.0", "id": nil, "error": jsonrpcError{Code: -32700, Message: "empty body"}})
		return
	}
	if strings.HasPrefix(trimmed, "[") {
		var batch []jsonrpcRequest
		if err := json.Unmarshal(raw, &batch); err != nil {
			mcpWrite(w, http.StatusBadRequest, map[string]any{"jsonrpc": "2.0", "id": nil, "error": jsonrpcError{Code: -32700, Message: "invalid JSON"}})
			return
		}
		out := make([]map[string]any, 0, len(batch))
		for _, req := range batch {
			if res, ok := mcpHandle(r, req); ok {
				out = append(out, res)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		mcpWrite(w, http.StatusOK, out)
		return
	}
	var req jsonrpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		mcpWrite(w, http.StatusBadRequest, map[string]any{"jsonrpc": "2.0", "id": nil, "error": jsonrpcError{Code: -32700, Message: "invalid JSON"}})
		return
	}
	res, ok := mcpHandle(r, req)
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	mcpWrite(w, http.StatusOK, res)
}

func mcpWrite(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// mcpHandle answers one JSON-RPC message. The bool is false for
// notifications, which get no body.
func mcpHandle(httpReq *http.Request, req jsonrpcRequest) (map[string]any, bool) {
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"
	if isNotification {
		return nil, false
	}
	base := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	fail := func(code int, msg string) (map[string]any, bool) {
		out := map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": jsonrpcError{Code: code, Message: msg}}
		return out, true
	}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := params.ProtocolVersion
		if version == "" {
			version = mcp.ProtocolVersion
		}
		base["result"] = map[string]any{
			"protocolVersion": version,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{"name": "zcode-bridge", "version": mcp.Version()},
			"instructions": "Tools are served by zcode-bridge and forwarded to the upstream MCP servers listed by GET /mcp. " +
				"Names are {server}__{tool}.",
		}
		return base, true
	case "ping":
		base["result"] = map[string]any{}
		return base, true
	case "tools/list":
		reg := mcpServers()
		tools := make([]map[string]any, 0, len(reg.Tools()))
		for _, t := range reg.Tools() {
			schema := t.InputSchema
			if len(schema) == 0 || string(schema) == "null" {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			tools = append(tools, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": schema,
			})
		}
		base["result"] = map[string]any{"tools": tools}
		return base, true
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return fail(-32602, "invalid params: "+err.Error())
		}
		if params.Name == "" {
			return fail(-32602, "tools/call requires a name")
		}
		reg := mcpServers()
		ctx, cancel := context.WithTimeout(httpReq.Context(), 120*time.Second)
		defer cancel()
		raw, err := reg.Call(ctx, params.Name, params.Arguments)
		if err != nil {
			base["result"] = map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}
			return base, true
		}
		if len(raw) == 0 || string(raw) == "null" {
			base["result"] = map[string]any{"content": []map[string]any{}}
			return base, true
		}
		var result any
		if err := json.Unmarshal(raw, &result); err != nil {
			base["result"] = map[string]any{
				"content": []map[string]any{{"type": "text", "text": string(raw)}},
			}
			return base, true
		}
		base["result"] = result
		return base, true
	case "resources/list":
		base["result"] = map[string]any{"resources": []any{}}
		return base, true
	case "prompts/list":
		base["result"] = map[string]any{"prompts": []any{}}
		return base, true
	case "logging/setLevel":
		base["result"] = map[string]any{}
		return base, true
	default:
		return fail(-32601, "method not found: "+req.Method)
	}
}

// mcpCatalog is the human page (and a JSON catalog for `Accept:
// application/json` or ?format=json).
func mcpCatalog(w http.ResponseWriter, r *http.Request) {
	reg := mcpServers()
	status := reg.Status()
	tools := reg.Tools()
	endpoint := "http://" + r.Host + "/mcp"

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
		sendJSON(w, http.StatusOK, map[string]any{
			"endpoint": endpoint,
			"servers":  status,
			"tools":    tools,
			"error":    errString(mcpInitErr),
			"client":   map[string]any{"mcpServers": map[string]any{"zcode-bridge": map[string]any{"type": "http", "url": endpoint}}},
		})
		return
	}

	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset="utf-8"><title>zcode-bridge · MCP</title>`)
	b.WriteString(`<style>body{background:#0b0e14;color:#cbd5e1;font:14px/1.55 ui-monospace,Consolas,monospace;margin:0;padding:32px}h1{font-size:18px;color:#e2e8f0;margin:0 0 4px}p.dim{color:#64748b;margin:0 0 24px}a{color:#38bdf8}table{border-collapse:collapse;width:100%;margin:0 0 28px}th,td{text-align:left;padding:7px 10px;border-bottom:1px solid #1c222e;vertical-align:top}th{color:#94a3b8;font-weight:600}code,pre{background:#121722;border:1px solid #1c222e;border-radius:6px}code{padding:1px 5px}pre{padding:14px;overflow:auto}.bad{color:#f87171}.ok{color:#34d399}</style>`)
	b.WriteString(`<h1>MCP endpoint</h1><p class="dim">` + html.EscapeString(endpoint) + ` · JSON-RPC 2.0 · tools are namespaced <code>{server}__{tool}</code></p>`)
	if mcpInitErr != nil {
		b.WriteString(`<p class="bad">Config error: ` + html.EscapeString(mcpInitErr.Error()) + `</p>`)
	}
	b.WriteString(`<h3>Upstream servers</h3>`)
	if len(status) == 0 {
		b.WriteString(`<p class="dim">No servers configured. Set <code>ZCODE_MCP_SERVERS</code> to a JSON file (or inline JSON) such as:</p>`)
		b.WriteString(`<pre>{"mcpServers":{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","D:\\work"]},"remote":{"url":"http://127.0.0.1:9000/mcp"}}}</pre>`)
	} else {
		b.WriteString(`<table><tr><th>Server</th><th>Transport</th><th>Tools</th><th>Status</th></tr>`)
		for _, st := range status {
			state := `<span class="ok">ok</span>`
			if st.Error != "" {
				state = `<span class="bad">` + html.EscapeString(st.Error) + `</span>`
			}
			b.WriteString(fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>`,
				html.EscapeString(st.Name), html.EscapeString(st.Transport), st.Tools, state))
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`<h3>Tools (` + fmt.Sprint(len(tools)) + `)</h3>`)
	if len(tools) == 0 {
		b.WriteString(`<p class="dim">None yet. A failing server above explains why.</p>`)
	} else {
		b.WriteString(`<table><tr><th>Name</th><th>Server</th><th>Description</th></tr>`)
		for _, t := range tools {
			b.WriteString(fmt.Sprintf(`<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>`,
				html.EscapeString(t.Name), html.EscapeString(t.Server), html.EscapeString(t.Description)))
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`<h3>Client config</h3><pre>` + html.EscapeString(fmt.Sprintf(`{"mcpServers":{"zcode-bridge":{"type":"http","url":"%s"}}}`, endpoint)) + `</pre>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
