// Package offpeak implements ZCode's off-peak ("idle plan") ticket queue: a
// free-compute lane where a request takes a queue number and is forwarded once
// the ticket turns ready. Behavior ported from TriDefender/zcode-api (source
// not copied); the upstream endpoints are documented in its config reference.
package offpeak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/auth"
)

const (
	availabilityPath = "/api/v1/off-peak/ticket/availability"
	ticketPath       = "/api/v1/off-peak/ticket"
	statusPath       = "/api/v1/off-peak/ticket/status"
	settlePath       = "/api/v1/off-peak/ticket/"
)

// Config is the async lane's tunables (env-overridable, defaults mirror the
// reference: 5 s poll, 3 s keepalive, unlimited wait, 3 retakes).
type Config struct {
	Origin         string
	PollInterval   time.Duration
	KeepAlive      time.Duration
	MaxWait        time.Duration
	MaxRetries     int
	ControlTimeout time.Duration
}

// Enabled reports whether the /async routes are switched on.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ZCODE_ASYNC_ENABLED"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// LoadConfig reads the async lane's environment overrides.
func LoadConfig() Config {
	c := Config{
		Origin:         envStr("ZCODE_ASYNC_ORIGIN", "https://zcode.z.ai"),
		PollInterval:   envMs("ZCODE_ASYNC_POLL_MS", 5000),
		KeepAlive:      envMs("ZCODE_ASYNC_KEEPALIVE_MS", 3000),
		MaxWait:        envMs("ZCODE_ASYNC_MAX_WAIT_MS", 0),
		MaxRetries:     3,
		ControlTimeout: envMs("ZCODE_ASYNC_CONTROL_TIMEOUT_MS", 15000),
	}
	if v := strings.TrimSpace(os.Getenv("ZCODE_ASYNC_MAX_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.MaxRetries = n
		}
	}
	c.Origin = strings.TrimRight(c.Origin, "/")
	return c
}

func envStr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envMs(key string, fallback int64) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return time.Duration(fallback) * time.Millisecond
}

// ErrUnavailable marks a control-plane refusal the client can retry later.
var ErrUnavailable = errors.New("off-peak queue unavailable")

// resolveJWT is a seam so tests can inject a token.
var resolveJWT = func() string { return auth.ResolvePlanJWT("") }

// SetJWTResolverForTest swaps credential resolution in tests.
func SetJWTResolverForTest(fn func() string) func() {
	prev := resolveJWT
	resolveJWT = fn
	return func() { resolveJWT = prev }
}

type client struct {
	cfg Config
	jwt string
	hc  *http.Client
}

func newClient(cfg Config, jwt string) *client {
	return &client{cfg: cfg, jwt: jwt, hc: &http.Client{Timeout: cfg.ControlTimeout}}
}

// call performs one control-plane call and unwraps the {code,data} envelope.
// A non-zero business code on HTTP 200 is an error, matching the reference.
func (c *client) call(ctx context.Context, method, path string, payload any) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Origin+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+c.jwt)
	req.Header.Set("x-coding-plan-api-key", c.jwt)
	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env struct {
		Code *float64        `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("off-peak %s: HTTP %d", path, res.StatusCode)
		}
		return nil, fmt.Errorf("off-peak %s: invalid response", path)
	}
	if res.StatusCode >= 400 {
		msg := env.Msg
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", res.StatusCode)
		}
		return nil, fmt.Errorf("off-peak %s: %s", path, msg)
	}
	if env.Code != nil && *env.Code != 0 {
		return nil, fmt.Errorf("off-peak %s: biz code %v %s", path, *env.Code, env.Msg)
	}
	out := map[string]any{}
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &out)
	}
	return out, nil
}

// Availability probes the queue and returns can_take_number plus the next
// allowed attempt time (unix seconds; 0 when unknown).
func Availability(ctx context.Context) (bool, int64, error) {
	jwt := resolveJWT()
	if jwt == "" {
		return false, 0, errors.New("no plan JWT")
	}
	data, err := newClient(LoadConfig(), jwt).call(ctx, http.MethodGet, availabilityPath, nil)
	if err != nil {
		return false, 0, err
	}
	can, _ := data["can_take_number"].(bool)
	var next int64
	if v, ok := data["next_take_at"].(float64); ok {
		next = int64(v)
	}
	return can, next, nil
}

type ticketState struct {
	ID          string
	State       string
	NextPollMs  int64
	ActiveUntil int64
}

func (c *client) take(ctx context.Context, taskID string) (ticketState, error) {
	data, err := c.call(ctx, http.MethodPost, ticketPath, map[string]any{"task_id": taskID})
	if err != nil {
		return ticketState{}, err
	}
	id, _ := data["ticket_id"].(string)
	state, _ := data["state"].(string)
	if id == "" || state == "" {
		return ticketState{}, fmt.Errorf("off-peak takeTicket: malformed response")
	}
	t := ticketState{ID: id, State: state}
	if v, ok := data["next_poll_after"].(float64); ok {
		t.NextPollMs = int64(v) * 1000
	}
	return t, nil
}

func (c *client) status(ctx context.Context, id string) (ticketState, error) {
	data, err := c.call(ctx, http.MethodPost, statusPath, map[string]any{"ticket_ids": []string{id}})
	if err != nil {
		return ticketState{}, err
	}
	items, _ := data["tickets"].([]any)
	if len(items) == 0 {
		return ticketState{ID: id, State: "not_found"}, nil
	}
	first, _ := items[0].(map[string]any)
	state, _ := first["state"].(string)
	t := ticketState{ID: id, State: state}
	if v, ok := data["next_poll_after"].(float64); ok {
		t.NextPollMs = int64(v) * 1000
	}
	if v, ok := first["active_deadline"].(float64); ok {
		t.ActiveUntil = int64(v)
	}
	return t, nil
}

func (c *client) settle(ctx context.Context, id string) {
	// 4xx here means the server already cleaned up; settle is best-effort.
	_, _ = c.call(ctx, http.MethodPost, settlePath+id+"/settle", nil)
}

func readyState(s string) bool {
	return s == "ready" || s == "active"
}

// Acquire takes a queue number and blocks until the ticket is ready, calling
// keepalive between polls so the client connection sees SSE comments. It
// retakes an expired ticket up to MaxRetries times. The returned ticket must
// be settled with Settle once the LLM call finishes.
func Acquire(ctx context.Context, keepalive func()) (string, error) {
	cfg := LoadConfig()
	jwt := resolveJWT()
	if jwt == "" {
		return "", errors.New("async endpoints require a plan JWT (run zcode-bridge login)")
	}
	c := newClient(cfg, jwt)
	deadline := time.Time{}
	if cfg.MaxWait > 0 {
		deadline = time.Now().Add(cfg.MaxWait)
	}
	taskID := fmt.Sprintf("bridge-%d", time.Now().UnixNano())
	for attempt := 0; ; attempt++ {
		t, err := c.take(ctx, taskID)
		if err != nil {
			return "", err
		}
		for {
			if readyState(t.State) {
				return t.ID, nil
			}
			if t.State != "queued" {
				break // expired / not_found / settled
			}
			wait := time.Duration(t.NextPollMs) * time.Millisecond
			if wait <= 0 {
				wait = cfg.PollInterval
			}
			if !deadline.IsZero() && time.Now().Add(wait).After(deadline) {
				c.settle(context.Background(), t.ID)
				return "", errors.New("async max wait timeout exceeded")
			}
			if keepalive != nil {
				keepalive()
			}
			if err := sleepKeepalive(ctx, wait, cfg.KeepAlive, keepalive); err != nil {
				c.settle(context.Background(), t.ID)
				return "", err
			}
			next, err := c.status(ctx, t.ID)
			if err != nil {
				if ctx.Err() != nil {
					c.settle(context.Background(), t.ID)
					return "", ctx.Err()
				}
				continue
			}
			t = next
		}
		c.settle(context.Background(), t.ID)
		if attempt >= cfg.MaxRetries {
			return "", fmt.Errorf("async ticket expired %d time(s); upstream retries exhausted", attempt+1)
		}
		if err := sleepKeepalive(ctx, 0, cfg.KeepAlive, keepalive); err != nil {
			return "", err
		}
	}
}

func sleepKeepalive(ctx context.Context, total, every time.Duration, keepalive func()) error {
	if total <= 0 {
		if keepalive != nil {
			keepalive()
		}
		return ctx.Err()
	}
	end := time.Now().Add(total)
	for {
		remaining := time.Until(end)
		if remaining <= 0 {
			return nil
		}
		step := remaining
		if every > 0 && step > every {
			step = every
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(step):
		}
		if keepalive != nil && time.Until(end) > 0 {
			keepalive()
		}
	}
}

// Settle closes out a ticket (success, abort, or client disconnect).
func Settle(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	jwt := resolveJWT()
	if jwt == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), LoadConfig().ControlTimeout)
	defer cancel()
	newClient(LoadConfig(), jwt).settle(ctx, id)
}
