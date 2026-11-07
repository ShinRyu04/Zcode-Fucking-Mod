package offpeak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAcquireWaitsWithKeepaliveAndSettles(t *testing.T) {
	var mu sync.Mutex
	polls := 0
	settled := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"ticket_id": "t1", "state": "queued", "next_poll_after": 0.001,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket/status":
			mu.Lock()
			polls++
			n := polls
			mu.Unlock()
			state := "queued"
			if n >= 2 {
				state = "ready"
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"next_poll_after": 0.001,
				"tickets":         []any{map[string]any{"ticket_id": "t1", "state": state}},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket/t1/settle":
			mu.Lock()
			settled = append(settled, "t1")
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("ZCODE_ASYNC_ORIGIN", srv.URL)
	t.Setenv("ZCODE_ASYNC_POLL_MS", "1")
	t.Setenv("ZCODE_ASYNC_KEEPALIVE_MS", "1")
	t.Setenv("ZCODE_ASYNC_MAX_RETRIES", "1")
	t.Setenv("ZCODE_ASYNC_CONTROL_TIMEOUT_MS", "2000")
	restore := SetJWTResolverForTest(func() string { return "jwt" })
	defer restore()

	keepalives := 0
	id, err := Acquire(context.Background(), func() { keepalives++ })
	if err != nil {
		t.Fatal(err)
	}
	if id != "t1" {
		t.Fatalf("ticket = %q", id)
	}
	if keepalives == 0 {
		t.Fatal("expected at least one keepalive during the queue wait")
	}
	Settle(id)
	mu.Lock()
	defer mu.Unlock()
	if len(settled) != 1 || settled[0] != "t1" {
		t.Fatalf("settled = %v", settled)
	}
}

func TestAcquireRetakesExpiredTicket(t *testing.T) {
	var mu sync.Mutex
	takes := 0
	polls := 0
	settled := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket":
			mu.Lock()
			takes++
			n := takes
			mu.Unlock()
			id := "t1"
			if n > 1 {
				id = "t2"
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"ticket_id": id, "state": "queued", "next_poll_after": 0.001,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket/status":
			mu.Lock()
			polls++
			n := polls
			mu.Unlock()
			state := "expired"
			id := "t1"
			if n >= 2 {
				state = "ready"
				id = "t2"
			}
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"tickets": []any{map[string]any{"ticket_id": id, "state": state}},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket/t1/settle":
			mu.Lock()
			settled = append(settled, "t1")
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/off-peak/ticket/t2/settle":
			mu.Lock()
			settled = append(settled, "t2")
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("ZCODE_ASYNC_ORIGIN", srv.URL)
	t.Setenv("ZCODE_ASYNC_POLL_MS", "1")
	t.Setenv("ZCODE_ASYNC_KEEPALIVE_MS", "1")
	t.Setenv("ZCODE_ASYNC_MAX_RETRIES", "2")
	t.Setenv("ZCODE_ASYNC_CONTROL_TIMEOUT_MS", "2000")
	restore := SetJWTResolverForTest(func() string { return "jwt" })
	defer restore()

	id, err := Acquire(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if id != "t2" {
		t.Fatalf("retake ticket = %q", id)
	}
	Settle(id)
	mu.Lock()
	defer mu.Unlock()
	if len(settled) != 2 || settled[0] != "t1" || settled[1] != "t2" {
		t.Fatalf("settled = %v", settled)
	}
}

func TestEnabledFlag(t *testing.T) {
	t.Setenv("ZCODE_ASYNC_ENABLED", "1")
	if !Enabled() {
		t.Fatal("ZCODE_ASYNC_ENABLED=1 must enable the idle lane")
	}
	t.Setenv("ZCODE_ASYNC_ENABLED", "off")
	if Enabled() {
		t.Fatal("off must disable the idle lane")
	}
}

var _ = time.Second
