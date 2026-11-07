package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/offpeak"
	"github.com/azrinaldi/zcode-bridge/internal/transport"
)

const asyncDisabledMsg = "idle channel is off; set ZCODE_ASYNC_ENABLED=1 and make sure a plan JWT is logged in"

// handleAsyncHealth probes the off-peak ticket queue.
func handleAsyncHealth(w http.ResponseWriter, r *http.Request) {
	if !offpeak.Enabled() {
		sendJSON(w, 400, map[string]any{"ok": false, "enabled": false, "error": asyncDisabledMsg})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	can, next, err := offpeak.Availability(ctx)
	if err != nil {
		sendJSON(w, 502, map[string]any{"ok": false, "enabled": true, "error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{
		"ok":              true,
		"enabled":         true,
		"can_take_number": can,
		"next_take_at":    next,
	})
}

// prepareAsync validates an async request and, for streaming clients, commits
// SSE headers so queue-wait keepalives reach the client. It returns the raw
// body to re-feed into the regular dialect handler.
func prepareAsync(w http.ResponseWriter, r *http.Request) ([]byte, bool, bool) {
	raw, err := readBody(r)
	if err != nil {
		anthropicError(w, 400, "Failed to read request body", "invalid_request_error")
		return nil, false, false
	}
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		anthropicError(w, 400, "Invalid JSON body", "invalid_request_error")
		return nil, false, false
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) == 0 {
		anthropicError(w, 400, "missing or invalid messages field", "invalid_request_error")
		return nil, false, false
	}
	streamWanted, _ := body["stream"].(bool)
	if streamWanted {
		body["stream"] = true
		if out, err := json.Marshal(body); err == nil {
			raw = out
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flushNow(w)
	}
	return raw, streamWanted, true
}

func flushNow(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeAsyncSSE(w http.ResponseWriter, event, payload string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flushNow(w)
}

func writeAsyncError(w http.ResponseWriter, committed bool, message, kind string) {
	payload, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]any{"type": kind, "message": message}})
	if committed {
		writeAsyncSSE(w, "error", string(payload))
		return
	}
	anthropicError(w, 502, message, kind)
}

// runAsync takes a ticket (emitting SSE keepalives while queued) and then
// replays the validated body through the regular handler with the ticket
// bound to the context, so every existing translation/capture path applies.
func runAsync(w http.ResponseWriter, r *http.Request, accountID string, raw []byte, streamWanted bool, handler func(http.ResponseWriter, *http.Request, map[string]any, string)) {
	keepalive := func() {
		if streamWanted {
			fmt.Fprint(w, ": keepalive\n\n")
			flushNow(w)
		}
	}
	ticket, err := offpeak.Acquire(r.Context(), keepalive)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		writeAsyncError(w, streamWanted, err.Error(), "api_error")
		return
	}
	defer offpeak.Settle(ticket)
	rm := r.Clone(r.Context())
	rm.Body = io.NopCloser(bytes.NewReader(raw))
	rm = rm.WithContext(transport.WithOffPeakTicket(rm.Context(), ticket))
	handlePost(w, rm, accountID, handler)
}

func handleAsyncMessages(w http.ResponseWriter, r *http.Request, accountID string) {
	if !offpeak.Enabled() {
		anthropicError(w, 400, asyncDisabledMsg, "async_disabled")
		return
	}
	raw, streamWanted, ok := prepareAsync(w, r)
	if !ok {
		return
	}
	runAsync(w, r, accountID, raw, streamWanted, handleMessages)
}

func handleAsyncChat(w http.ResponseWriter, r *http.Request, accountID string) {
	if !offpeak.Enabled() {
		openAIError(w, 400, asyncDisabledMsg, "async_disabled")
		return
	}
	raw, streamWanted, ok := prepareAsync(w, r)
	if !ok {
		return
	}
	runAsync(w, r, accountID, raw, streamWanted, handleChatCompletions)
}
