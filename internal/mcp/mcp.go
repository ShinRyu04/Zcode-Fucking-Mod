// Package mcp is a minimal Model Context Protocol client: it starts (or
// connects to) external tool servers, lists their tools and forwards calls.
//
// Only the parts a bridge needs are implemented — initialize, tools/list,
// tools/call — over the two transports that matter in practice: a child
// process speaking newline-delimited JSON-RPC on stdio, and the streamable
// HTTP transport (POST + JSON or SSE response, Mcp-Session-Id).
//
// Tool names are namespaced as {server}__{tool} so one flat tool list can be
// handed to a model that has no notion of servers.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is what the client announces; servers answer with the
// version they support and the client accepts it.
const ProtocolVersion = "2025-06-18"

// ClientName identifies the bridge to upstream servers.
const ClientName = "zcode-bridge"

// Error is a JSON-RPC error returned by a server.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("mcp error %d: %s (%s)", e.Code, e.Message, string(e.Data))
	}
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

type wireMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

func (m *wireMessage) isNotification() bool { return len(m.ID) == 0 }

// Tool is one tool as the server describes it.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// InitializeResult is the server's answer to initialize.
type InitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Instructions string `json:"instructions"`
}

// Transport moves JSON-RPC messages to one server.
type Transport interface {
	Request(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}

// Client is one initialized connection to an MCP server.
type Client struct {
	Name    string
	Version string // server-reported version
	tr      Transport
	tools   []Tool
}

// NewClient initializes tr and lists the server's tools.
func NewClient(ctx context.Context, name string, tr Transport) (*Client, error) {
	c := &Client{Name: name, tr: tr}
	raw, err := c.tr.Request(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": ClientName, "version": Version()},
	})
	if err != nil {
		tr.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}
	var res InitializeResult
	if err := json.Unmarshal(raw, &res); err != nil {
		tr.Close()
		return nil, fmt.Errorf("initialize result: %w", err)
	}
	c.Version = res.ServerInfo.Version
	if err := c.tr.Notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		// A server that refuses the notification is still usable.
		_ = err
	}
	tools, err := c.listTools(ctx)
	if err != nil {
		tr.Close()
		return nil, err
	}
	c.tools = tools
	return c, nil
}

// Version is the client version, reusing the app version the identity layer
// reports so one number identifies the build everywhere.
func Version() string {
	if v := os.Getenv("ZCODE_IDENTITY_APP_VERSION"); v != "" {
		return v
	}
	return "3.16.3"
}

func (c *Client) listTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for page := 0; page < 50; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.tr.Request(ctx, "tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("tools/list: %w", err)
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("tools/list result: %w", err)
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" || res.NextCursor == cursor {
			return all, nil
		}
		cursor = res.NextCursor
	}
	return all, nil
}

// Tools returns the cached tool list.
func (c *Client) Tools() []Tool { return c.tools }

// Call invokes one tool and returns its raw result.
func (c *Client) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	return c.tr.Request(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}

// Close releases the connection.
func (c *Client) Close() error { return c.tr.Close() }

// ---- stdio transport ----

type stdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *ringBuffer

	mu     sync.Mutex
	nextID int64
	wait   map[int64]chan wireMessage
	done   chan struct{}
	err    error
}

// StdioTransport starts command and speaks newline-delimited JSON-RPC over
// its stdin/stdout. On Windows a .cmd/.bat shim is launched through cmd.exe,
// which CreateProcess cannot do directly.
func StdioTransport(command string, args []string, env map[string]string, dir string) (Transport, error) {
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("mcp command %q: %w", command, err)
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".cmd" || ext == ".bat" {
		args = append([]string{"/c", path}, args...)
		path = os.Getenv("COMSPEC")
		if path == "" {
			path = "cmd.exe"
		}
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	t := &stdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stderr: newRingBuffer(8 << 10),
		wait:   map[int64]chan wireMessage{},
		done:   make(chan struct{}),
	}
	cmd.Stderr = t.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go t.readLoop(stdout)
	go func() {
		err := cmd.Wait()
		t.mu.Lock()
		if t.err == nil {
			t.err = err
		}
		t.mu.Unlock()
		close(t.done)
	}()
	return t, nil
}

func (t *stdioTransport) readLoop(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg wireMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // servers occasionally log to stdout; ignore non-JSON
		}
		if msg.Method != "" && !msg.isNotification() {
			// Server→client request (sampling, roots, …): decline politely.
			t.reply(msg.ID, wireMessage{JSONRPC: "2.0", Error: &Error{Code: -32601, Message: "method not supported by bridge client: " + msg.Method}})
			continue
		}
		if msg.isNotification() {
			continue
		}
		var id int64
		if err := json.Unmarshal(msg.ID, &id); err != nil {
			continue
		}
		t.mu.Lock()
		ch := t.wait[id]
		delete(t.wait, id)
		t.mu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
	t.mu.Lock()
	if t.err == nil {
		if err := sc.Err(); err != nil {
			t.err = err
		} else {
			t.err = errors.New("mcp server closed its stdout")
		}
	}
	t.mu.Unlock()
	select {
	case <-t.done:
	default:
	}
}

func (t *stdioTransport) reply(id json.RawMessage, msg wireMessage) {
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	msg.ID = id
	_ = t.write(msg)
}

func (t *stdioTransport) write(msg wireMessage) error {
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = t.stdin.Write(data)
	return err
}

func (t *stdioTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	ch := make(chan wireMessage, 1)
	t.wait[id] = ch
	t.mu.Unlock()

	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	idRaw, _ := json.Marshal(id)
	if err := t.write(wireMessage{ID: idRaw, Method: method, Params: raw}); err != nil {
		t.mu.Lock()
		delete(t.wait, id)
		t.mu.Unlock()
		return nil, err
	}
	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.wait, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case <-t.done:
		select {
		case msg := <-ch:
			return messageResult(msg)
		default:
		}
		t.mu.Lock()
		err := t.err
		t.mu.Unlock()
		detail := strings.TrimSpace(t.stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("mcp server exited: %v: %s", err, tail(detail, 400))
		}
		return nil, fmt.Errorf("mcp server exited: %v", err)
	case msg := <-ch:
		return messageResult(msg)
	}
}

func messageResult(msg wireMessage) (json.RawMessage, error) {
	if msg.Error != nil {
		return nil, msg.Error
	}
	return msg.Result, nil
}

func (t *stdioTransport) Notify(ctx context.Context, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return t.write(wireMessage{Method: method, Params: raw})
}

func (t *stdioTransport) Close() error {
	_ = t.stdin.Close()
	select {
	case <-t.done:
		return nil
	case <-time.After(2 * time.Second):
	}
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	return nil
}

// ---- streamable HTTP transport ----

type httpTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	session string
	mu      sync.Mutex
	nextID  int64
}

// HTTPTransport connects to a streamable-HTTP MCP endpoint.
func HTTPTransport(url string, headers map[string]string) Transport {
	return &httpTransport{
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (t *httpTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	t.nextID++
	id := t.nextID
	t.mu.Unlock()

	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	idRaw, _ := json.Marshal(id)
	req := wireMessage{ID: idRaw, Method: method, Params: raw}
	return t.post(ctx, req, idRaw)
}

func (t *httpTransport) Notify(ctx context.Context, method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req := wireMessage{Method: method, Params: raw}
	_, err = t.post(ctx, req, nil)
	return err
}

func (t *httpTransport) post(ctx context.Context, req wireMessage, expectID json.RawMessage) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}
	t.mu.Lock()
	if t.session != "" {
		httpReq.Header.Set("Mcp-Session-Id", t.session)
	}
	t.mu.Unlock()

	res, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if sid := res.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.session = sid
		t.mu.Unlock()
	}
	if res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("mcp http %d: %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	if strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		return readSSE(res.Body, expectID)
	}
	var msg wireMessage
	if err := json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(&msg); err != nil {
		return nil, err
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	return msg.Result, nil
}

// readSSE returns the first event whose id matches. Streamable HTTP servers
// may interleave notifications; those are skipped.
func readSSE(body io.Reader, expectID json.RawMessage) (json.RawMessage, error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	var data bytes.Buffer
	flush := func() (json.RawMessage, bool, error) {
		if data.Len() == 0 {
			return nil, false, nil
		}
		payload := bytes.TrimSpace(data.Bytes())
		data.Reset()
		var msg wireMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, false, nil
		}
		if msg.Error != nil && (len(expectID) == 0 || bytes.Equal(msg.ID, expectID)) {
			return nil, false, msg.Error
		}
		if len(expectID) > 0 && bytes.Equal(msg.ID, expectID) {
			return msg.Result, true, nil
		}
		return nil, false, nil
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			res, done, err := flush()
			if err != nil {
				return nil, err
			}
			if done {
				return res, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	res, done, err := flush()
	if err != nil {
		return nil, err
	}
	if done {
		return res, nil
	}
	return nil, errors.New("mcp sse stream ended without a response")
}

func (t *httpTransport) Close() error {
	t.mu.Lock()
	session := t.session
	t.mu.Unlock()
	if session == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodDelete, t.url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Mcp-Session-Id", session)
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	res, err := t.client.Do(req)
	if err == nil {
		_ = res.Body.Close()
	}
	return err
}

// ---- helpers ----

type ringBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newRingBuffer(max int) *ringBuffer { return &ringBuffer{max: max} }

func (r *ringBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
	return len(p), nil
}

func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
