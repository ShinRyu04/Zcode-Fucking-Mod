package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ServerConfig is one upstream server. Either Command (stdio) or URL (http)
// is required; Transport is inferred when empty.
type ServerConfig struct {
	Name           string            `json:"name"`
	Transport      string            `json:"transport"`
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	Dir            string            `json:"dir"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers"`
	Disabled       bool              `json:"disabled"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// Config is the file shape. Both {"servers":[…]} (bridge-style) and
// {"mcpServers":{name:{…}}} (Claude/Codex-style) are accepted.
type Config struct {
	Servers []ServerConfig `json:"servers"`

	// MDPServers is the map form; consumed by LoadConfig and left nil after.
	MDPServers map[string]ServerConfig `json:"mcpServers,omitempty"`
}

// LoadConfig reads a servers file, an inline JSON object, or returns an empty
// config when path is empty.
func LoadConfig(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &Config{}, nil
	}
	var raw []byte
	var err error
	if strings.HasPrefix(path, "{") || strings.HasPrefix(path, "[") {
		raw = []byte(path)
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	return ParseConfig(raw)
}

// ParseConfig decodes either accepted shape.
func ParseConfig(raw []byte) (*Config, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var list []ServerConfig
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		return &Config{Servers: list}, nil
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.MDPServers) > 0 {
		names := make([]string, 0, len(cfg.MDPServers))
		for name := range cfg.MDPServers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			sc := cfg.MDPServers[name]
			if sc.Name == "" {
				sc.Name = name
			}
			cfg.Servers = append(cfg.Servers, sc)
		}
		cfg.MDPServers = nil
	}
	return &cfg, nil
}

// ExposedTool is one tool seen through the namespace.
type ExposedTool struct {
	Name        string          `json:"name"`
	Original    string          `json:"original"`
	Server      string          `json:"server"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ServerStatus is what the catalog page reports per server.
type ServerStatus struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	Tools     int    `json:"tools"`
	Error     string `json:"error,omitempty"`
}

type serverEntry struct {
	cfg    ServerConfig
	client *Client
	err    error
}

// Registry owns every configured server and the flattened tool list.
type Registry struct {
	mu      sync.RWMutex
	servers []*serverEntry
	tools   []ExposedTool
	index   map[string]*ExposedTool
	closed  bool
}

// Start connects to every enabled server in parallel and lists tools.
// Failing servers do not abort the rest; their error is reported per server.
func Start(ctx context.Context, cfg *Config) *Registry {
	r := &Registry{index: map[string]*ExposedTool{}}
	if cfg == nil {
		return r
	}
	entries := make([]*serverEntry, 0, len(cfg.Servers))
	for _, sc := range cfg.Servers {
		if sc.Disabled || strings.TrimSpace(sc.Name) == "" {
			continue
		}
		entries = append(entries, &serverEntry{cfg: sc})
	}
	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func(e *serverEntry) {
			defer wg.Done()
			timeout := time.Duration(e.cfg.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			e.client, e.err = connect(sctx, e.cfg)
		}(e)
	}
	wg.Wait()

	r.servers = entries
	for _, e := range entries {
		if e.err != nil || e.client == nil {
			continue
		}
		for _, t := range e.client.Tools() {
			exposed := ExposedTool{
				Name:        namespace(e.cfg.Name, t.Name),
				Original:    t.Name,
				Server:      e.cfg.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			}
			if _, clash := r.index[exposed.Name]; clash {
				// Two servers map onto one name; keeping both would make the
				// flat list ambiguous, so the later one loses.
				e.err = fmt.Errorf("tool name collision on %q", exposed.Name)
				continue
			}
			r.index[exposed.Name] = &exposed
			r.tools = append(r.tools, exposed)
		}
	}
	sort.Slice(r.tools, func(i, j int) bool {
		if r.tools[i].Server == r.tools[j].Server {
			return r.tools[i].Original < r.tools[j].Original
		}
		return r.tools[i].Server < r.tools[j].Server
	})
	return r
}

func connect(ctx context.Context, sc ServerConfig) (*Client, error) {
	transport := strings.ToLower(strings.TrimSpace(sc.Transport))
	switch {
	case transport == "http" || transport == "streamable-http" || (transport == "" && sc.URL != ""):
		if sc.URL == "" {
			return nil, fmt.Errorf("http transport without url")
		}
		return NewClient(ctx, sc.Name, HTTPTransport(sc.URL, sc.Headers))
	case transport == "stdio" || transport == "":
		if sc.Command == "" {
			return nil, fmt.Errorf("stdio transport without command")
		}
		tr, err := StdioTransport(sc.Command, sc.Args, sc.Env, sc.Dir)
		if err != nil {
			return nil, err
		}
		return NewClient(ctx, sc.Name, tr)
	default:
		return nil, fmt.Errorf("unknown transport %q", sc.Transport)
	}
}

// namespace flattens {server}__{tool} into the name a model and an MCP client
// see. Characters outside [A-Za-z0-9_-] become '_' and the whole name is kept
// within the 64-character limit tool APIs accept.
func namespace(server, tool string) string {
	name := sanitize(server) + "__" + sanitize(tool)
	if len(name) <= 64 {
		return name
	}
	sum := fnv32(name)
	suffix := fmt.Sprintf("_%08x", sum)
	cut := 64 - len(suffix)
	if cut < 1 {
		cut = 1
	}
	return name[:cut] + suffix
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "server"
	}
	return out
}

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// Tools returns the flattened list.
func (r *Registry) Tools() []ExposedTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ExposedTool, len(r.tools))
	copy(out, r.tools)
	return out
}

// Has reports whether name is a known flattened tool.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.index[name]
	return ok
}

// Call forwards one tool call to its server.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	r.mu.RLock()
	tool := r.index[name]
	var entry *serverEntry
	if tool != nil {
		for _, e := range r.servers {
			if e.cfg.Name == tool.Server {
				entry = e
				break
			}
		}
	}
	r.mu.RUnlock()
	if tool == nil {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	if entry == nil || entry.client == nil {
		return nil, fmt.Errorf("server %q is not connected", tool.Server)
	}
	return entry.client.Call(ctx, tool.Original, args)
}

// Status reports each configured server.
func (r *Registry) Status() []ServerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ServerStatus, 0, len(r.servers))
	for _, e := range r.servers {
		st := ServerStatus{Name: e.cfg.Name, Transport: transportName(e.cfg)}
		if e.client != nil {
			st.Tools = len(e.client.Tools())
		}
		if e.err != nil {
			st.Error = e.err.Error()
		}
		out = append(out, st)
	}
	return out
}

func transportName(sc ServerConfig) string {
	switch strings.ToLower(strings.TrimSpace(sc.Transport)) {
	case "http", "streamable-http":
		return "http"
	case "stdio":
		return "stdio"
	}
	if sc.URL != "" {
		return "http"
	}
	return "stdio"
}

// Close disconnects every server. Safe to call twice.
func (r *Registry) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	servers := r.servers
	r.mu.Unlock()
	for _, e := range servers {
		if e.client != nil {
			_ = e.client.Close()
		}
	}
}
