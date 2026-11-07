package server

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/capture"
)

// maxInspectFull caps how many bodies one export may carry; the ring already
// limits this, but a hand-set ?limit= would otherwise copy megabytes.
const maxInspectFull = 50

// handleInspect serves the captured traffic ring:
//
//	GET /inspect?limit=N          newest-first summaries (no bodies)
//	GET /inspect?id=N             one exchange, bodies included
//	GET /inspect?limit=N&full=1   newest-first with bodies (JSON export)
//
// Bodies stay out of the listing because a full 200-entry ring with 64KB
// bodies would be 12MB per dashboard refresh.
func handleInspect(w http.ResponseWriter, r *http.Request) {
	state := capture.Status()
	if !state.Enabled {
		sendJSON(w, 200, map[string]any{
			"enabled":  false,
			"count":    0,
			"capacity": state.Capacity,
			"body_max": state.BodyMax,
			"entries":  []capture.Summary{},
			"hint":     "capture is off: restart the bridge with ZCODE_CAPTURE=1 to record exchanges",
		})
		return
	}
	if idText := r.URL.Query().Get("id"); idText != "" {
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			sendJSON(w, 400, map[string]any{"ok": false, "error": "id must be a positive integer"})
			return
		}
		ex, ok := capture.Get(id)
		if !ok {
			sendJSON(w, 404, map[string]any{"ok": false, "error": "no captured exchange with that id"})
			return
		}
		sendJSON(w, 200, ex)
		return
	}

	if format := r.URL.Query().Get("format"); format == "har" {
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		if limit > maxInspectFull {
			limit = maxInspectFull
		}
		entries := make([]any, 0, limit)
		for _, sum := range capture.List(limit) {
			if ex, ok := capture.Get(sum.ID); ok {
				entries = append(entries, harEntry(ex))
			}
		}
		sendJSON(w, 200, map[string]any{"log": map[string]any{
			"version": "1.2",
			"creator": map[string]any{"name": "zcode-bridge", "version": "0.1.0"},
			"entries": entries,
		}})
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	full := r.URL.Query().Get("full") == "1"
	if full {
		if limit > maxInspectFull {
			limit = maxInspectFull
		}
		entries := make([]capture.Exchange, 0, limit)
		for _, sum := range capture.List(limit) {
			if ex, ok := capture.Get(sum.ID); ok {
				entries = append(entries, ex)
			}
		}
		sendJSON(w, 200, map[string]any{
			"enabled": true, "count": state.Count, "capacity": state.Capacity,
			"body_max": state.BodyMax, "entries": entries,
		})
		return
	}
	sendJSON(w, 200, map[string]any{
		"enabled": true, "count": state.Count, "capacity": state.Capacity,
		"body_max": state.BodyMax, "entries": capture.List(limit),
	})
}

// handleInspectClear drops the ring (the inspector's "Bersihkan" button).
func handleInspectClear(w http.ResponseWriter) {
	sendJSON(w, 200, map[string]any{"ok": true, "cleared": capture.Clear()})
}

// handleInspectHAR exports the newest exchanges as HAR 1.2, see
// GET /inspect?format=har.

func harHeaders(h map[string]string) []map[string]string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]string{"name": k, "value": h[k]})
	}
	return out
}

func bridgeSelfURL() string {
	host := cfg.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port <= 0 {
		port = 20131
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func harEntry(ex capture.Exchange) map[string]any {
	contentType := "application/json"
	for k, v := range ex.ResponseHeaders {
		if strings.EqualFold(k, "Content-Type") {
			contentType = v
		}
	}
	requestMime := "application/json"
	for k, v := range ex.RequestHeaders {
		if strings.EqualFold(k, "Content-Type") {
			requestMime = v
		}
	}
	postData := map[string]any{"mimeType": requestMime, "text": ex.RequestBody}
	return map[string]any{
		"startedDateTime": ex.StartedAt.UTC().Format(time.RFC3339Nano),
		"time":            ex.DurationMS,
		"request": map[string]any{
			"method":      ex.Method,
			"url":         bridgeSelfURL() + ex.Path,
			"httpVersion": "HTTP/1.1",
			"headers":     harHeaders(ex.RequestHeaders),
			"queryString": []any{},
			"headersSize": -1,
			"bodySize":    ex.RequestBytes,
			"postData":    postData,
		},
		"response": map[string]any{
			"status":      ex.Status,
			"statusText":  http.StatusText(ex.Status),
			"httpVersion": "HTTP/1.1",
			"headers":     harHeaders(ex.ResponseHeaders),
			"content": map[string]any{
				"size":     ex.ResponseBytes,
				"mimeType": contentType,
				"text":     ex.ResponseBody,
			},
			"redirectURL": "",
			"headersSize": -1,
			"bodySize":    ex.ResponseBytes,
		},
		"cache":           map[string]any{},
		"timings":         map[string]any{"send": 0, "wait": ex.DurationMS, "receive": 0},
		"serverIPAddress": "127.0.0.1",
		"connection":      "",
	}
}

// handleInspectReplay re-issues one captured request through the bridge's own
// handler chain (in-process, same lane). It is a real inference call and
// produces a new capture entry, so it requires the observability token like
// every other inspector route.
func handleInspectReplay(w http.ResponseWriter, r *http.Request) {
	if !capture.Status().Enabled {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "capture is off (ZCODE_CAPTURE=1)"})
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		sendJSON(w, 400, map[string]any{"ok": false, "error": "id must be a positive integer"})
		return
	}
	ex, ok := capture.Get(id)
	if !ok {
		sendJSON(w, 404, map[string]any{"ok": false, "error": "no captured exchange with that id"})
		return
	}
	if ex.Truncated {
		sendJSON(w, 409, map[string]any{"ok": false, "error": "request body was truncated; replay would change the prompt"})
		return
	}
	method := ex.Method
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, "http://review.invalid"+ex.Path, strings.NewReader(ex.RequestBody))
	if err != nil {
		sendJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	for k, v := range ex.RequestHeaders {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") ||
			strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "X-Api-Key") {
			continue
		}
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.BridgeToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.BridgeToken)
	}
	rec := &replayRecorder{headers: http.Header{}}
	route(rec, req)
	body := rec.buf.String()
	truncated := false
	if len(body) > 4<<20 {
		body = body[:4<<20]
		truncated = true
	}
	sendJSON(w, 200, map[string]any{
		"ok":        true,
		"source_id": ex.ID,
		"status":    rec.statusCode(),
		"body":      body,
		"truncated": truncated,
	})
}

// replayRecorder captures an in-process response without a live socket.
type replayRecorder struct {
	headers http.Header
	code    int
	buf     bytes.Buffer
}

func (rr *replayRecorder) Header() http.Header { return rr.headers }

func (rr *replayRecorder) WriteHeader(code int) {
	if rr.code == 0 {
		rr.code = code
	}
}

func (rr *replayRecorder) Write(p []byte) (int, error) {
	if rr.code == 0 {
		rr.code = http.StatusOK
	}
	return rr.buf.Write(p)
}

func (rr *replayRecorder) statusCode() int {
	if rr.code == 0 {
		return http.StatusOK
	}
	return rr.code
}
