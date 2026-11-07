// Package capture keeps a bounded in-memory ring of finished HTTP exchanges
// for the built-in inspector (GET /inspect, dashboard "Inspector" tab).
//
// The bridge terminates both legs itself, so no MITM is involved: a recorder
// is opened before the inference handler runs, wraps the ResponseWriter, and
// is finalised when the handler returns. Capture is opt-in
// (ZCODE_CAPTURE=1); when it is off, Begin returns nil and every call site
// short-circuits without touching the ring.
//
// Ring size: ZCODE_CAPTURE_SIZE (default 200). Stored bodies are cut at
// ZCODE_CAPTURE_BODY_MAX bytes (default 64KB) and marked truncated.
package capture

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Defaults documented in the README.
const (
	DefaultCapacity = 200
	DefaultBodyMax  = 64 << 10
)

// Event is one upstream attempt or rotation noted while the exchange ran.
type Event struct {
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"` // attempt | rotation
	Account    string    `json:"account,omitempty"`
	Attempt    int       `json:"attempt,omitempty"`
	Status     int       `json:"status,omitempty"`
	Code       any       `json:"code,omitempty"`
	Switchable bool      `json:"switchable,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
}

// Exchange is one captured request/response pair, bodies included.
type Exchange struct {
	ID              int64             `json:"id"`
	StartedAt       time.Time         `json:"started_at"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Route           string            `json:"route"`
	Model           string            `json:"model,omitempty"`
	Account         string            `json:"account,omitempty"`
	Stream          bool              `json:"stream"`
	Status          int               `json:"status"`
	ErrorCode       any               `json:"error_code,omitempty"`
	DurationMS      int64             `json:"duration_ms"`
	FirstTokenMS    int64             `json:"first_token_ms,omitempty"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseHeaders map[string]string `json:"response_headers"`
	RequestBody     string            `json:"request_body"`
	ResponseBody    string            `json:"response_body"`
	RequestBytes    int               `json:"request_bytes"`
	ResponseBytes   int               `json:"response_bytes"`
	Truncated       bool              `json:"truncated"`
	Events          []Event           `json:"events,omitempty"`
}

// Summary is the list view: everything except the bodies, which stay out of
// the /inspect?limit=N listing so a 200-entry ring costs kilobytes, not
// megabytes.
type Summary struct {
	ID            int64     `json:"id"`
	StartedAt     time.Time `json:"started_at"`
	Method        string    `json:"method"`
	Path          string    `json:"path"`
	Route         string    `json:"route"`
	Model         string    `json:"model,omitempty"`
	Account       string    `json:"account,omitempty"`
	Stream        bool      `json:"stream"`
	Status        int       `json:"status"`
	ErrorCode     any       `json:"error_code,omitempty"`
	DurationMS    int64     `json:"duration_ms"`
	FirstTokenMS  int64     `json:"first_token_ms,omitempty"`
	RequestBytes  int       `json:"request_bytes"`
	ResponseBytes int       `json:"response_bytes"`
	Truncated     bool      `json:"truncated"`
}

// State describes the ring for the inspector UI and /inspect envelope.
type State struct {
	Enabled  bool `json:"enabled"`
	Count    int  `json:"count"`
	Capacity int  `json:"capacity"`
	BodyMax  int  `json:"body_max"`
}

var (
	loadOnce sync.Once
	settings settingsState
)

type settingsState struct {
	enabled  bool
	persist  bool
	capacity int
	bodyMax  int
}

// Enabled reports whether capture was requested (ZCODE_CAPTURE=1).
func Enabled() bool {
	load()
	return settings.enabled
}

func load() {
	loadOnce.Do(func() {
		settings.enabled = truthy(os.Getenv("ZCODE_CAPTURE"))
		settings.persist = settings.enabled
		settings.capacity = envInt("ZCODE_CAPTURE_SIZE", DefaultCapacity)
		settings.bodyMax = envInt("ZCODE_CAPTURE_BODY_MAX", DefaultBodyMax)
		if settings.enabled {
			store.reset(settings.capacity)
			store.restore(CaptureFilePath(), settings.capacity)
		}
	})
}

// EnableForTest turns capture on for this process and resets the ring.
// Production code only ever calls Enabled.
func EnableForTest(capacity, bodyMax int) {
	load()
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	if bodyMax <= 0 {
		bodyMax = DefaultBodyMax
	}
	settings.enabled = true
	settings.persist = false
	settings.capacity = capacity
	settings.bodyMax = bodyMax
	store.reset(capacity)
}

// ResetForTest restores the default (disabled) state.
func ResetForTest() {
	loadOnce = sync.Once{}
	settings = settingsState{}
	store.reset(0)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// --- recorder ---

type ctxKey struct{}

// WithRecorder stores the recorder on a request context so inner handlers can
// annotate the exchange (account, model, error code) without new parameters.
func WithRecorder(ctx context.Context, rec *Recorder) context.Context {
	if rec == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, rec)
}

// FromContext returns the recorder of the current request, or nil.
func FromContext(ctx context.Context) *Recorder {
	rec, _ := ctx.Value(ctxKey{}).(*Recorder)
	return rec
}

// Recorder accumulates one exchange. A nil *Recorder is valid and does
// nothing, so call sites never need an Enabled() branch.
type Recorder struct {
	mu       sync.Mutex
	ex       Exchange
	started  time.Time
	bodyMax  int
	reqBody  []byte
	respBuf  []byte
	status   int
	respHdr  http.Header
	firstTok time.Time
	finished bool
}

// Begin opens a recorder for one request. Returns nil when capture is off.
// raw is the request body as read by the handler (truncated on store).
func Begin(r *http.Request, raw []byte) *Recorder {
	if !Enabled() || r == nil {
		return nil
	}
	now := time.Now()
	rec := &Recorder{started: now, bodyMax: settings.bodyMax}
	rec.ex = Exchange{
		StartedAt:      now,
		Method:         r.Method,
		Path:           r.URL.RequestURI(),
		Route:          routeName(r.URL.Path),
		RequestHeaders: redactHeader(r.Header),
		RequestBytes:   len(raw),
	}
	rec.reqBody, rec.ex.Truncated = cut(raw, rec.bodyMax, false)
	return rec
}

// routeName maps a path onto the short label the inspector shows.
func routeName(p string) string {
	switch strings.TrimRight(p, "/") {
	case "/v1/messages":
		return "messages"
	case "/v1/chat/completions":
		return "chat.completions"
	case "/v1/responses":
		return "responses"
	}
	return strings.TrimPrefix(strings.TrimRight(p, "/"), "/")
}

// Wrap returns a ResponseWriter that tees status, headers and body into the
// recorder. It preserves http.Flusher so SSE still streams.
func (rec *Recorder) Wrap(w http.ResponseWriter) http.ResponseWriter {
	if rec == nil {
		return w
	}
	return newWriter(w, rec)
}

func (rec *Recorder) setStatus(code int) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	rec.status = code
	rec.mu.Unlock()
}

func (rec *Recorder) appendBody(p []byte) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.ex.ResponseBytes += len(p)
	if len(rec.respBuf) >= rec.bodyMax {
		rec.ex.Truncated = true
		return
	}
	room := rec.bodyMax - len(rec.respBuf)
	chunk := p
	if len(chunk) > room {
		chunk = chunk[:room]
		rec.ex.Truncated = true
	}
	rec.respBuf = append(rec.respBuf, chunk...)
}

// NoteAccount records the account that served the request.
func (rec *Recorder) NoteAccount(id string) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	rec.ex.Account = id
	rec.mu.Unlock()
}

// NoteModel records the normalized model id.
func (rec *Recorder) NoteModel(model string) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	rec.ex.Model = model
	rec.mu.Unlock()
}

// NoteStream marks the exchange as SSE.
func (rec *Recorder) NoteStream(stream bool) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	rec.ex.Stream = stream
	rec.mu.Unlock()
}

// NoteFirstToken timestamps the first streamed delta (TTFT).
func (rec *Recorder) NoteFirstToken() {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	if rec.firstTok.IsZero() {
		rec.firstTok = time.Now()
	}
	rec.mu.Unlock()
}

// NoteError records the upstream envelope code that failed the exchange so the
// inspector can filter on 3012/3008/3009/1113/429.
func (rec *Recorder) NoteError(code any) {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	if rec.ex.ErrorCode == nil {
		rec.ex.ErrorCode = code
	}
	rec.mu.Unlock()
}

// NoteAttempt appends one upstream attempt or rotation event.
func (rec *Recorder) NoteAttempt(ev Event) {
	if rec == nil {
		return
	}
	ev.At = time.Now()
	rec.mu.Lock()
	if len(rec.ex.Events) < 32 {
		rec.ex.Events = append(rec.ex.Events, ev)
	}
	rec.mu.Unlock()
}

// Finish stores the exchange in the ring. Safe to call once; later calls are
// no-ops.
func (rec *Recorder) Finish() {
	if rec == nil {
		return
	}
	rec.mu.Lock()
	if rec.finished {
		rec.mu.Unlock()
		return
	}
	rec.finished = true
	ex := rec.ex
	ex.DurationMS = time.Since(rec.started).Milliseconds()
	if ex.Status == 0 {
		ex.Status = rec.status
	}
	if ex.Status == 0 {
		ex.Status = http.StatusOK
	}
	if !rec.firstTok.IsZero() {
		ex.FirstTokenMS = rec.firstTok.Sub(rec.started).Milliseconds()
	}
	if rec.respHdr != nil {
		ex.ResponseHeaders = redactHeader(rec.respHdr)
	}
	ex.RequestBody = redactBody(string(rec.reqBody))
	ex.ResponseBody = redactBody(string(rec.respBuf))
	events := ex.Events
	rec.mu.Unlock()

	if !Enabled() {
		return
	}
	store.add(&ex, events)
}

// writer tees the response into the recorder.
type writer struct {
	http.ResponseWriter
	rec     *Recorder
	needHdr bool
}

func (w *writer) WriteHeader(code int) {
	if w.needHdr {
		w.rec.mu.Lock()
		w.rec.respHdr = w.Header().Clone()
		w.rec.mu.Unlock()
		w.needHdr = false
	}
	w.rec.setStatus(code)
	w.ResponseWriter.WriteHeader(code)
}

func (w *writer) Write(p []byte) (int, error) {
	if w.needHdr {
		w.WriteHeader(http.StatusOK)
	}
	w.rec.appendBody(p)
	return w.ResponseWriter.Write(p)
}

// Flush keeps SSE clients streaming through the wrapper.
func (w *writer) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func newWriter(w http.ResponseWriter, rec *Recorder) *writer {
	return &writer{ResponseWriter: w, rec: rec, needHdr: true}
}

// --- redaction ---

var sensitiveHeaders = map[string]bool{
	"authorization":         true,
	"proxy-authorization":   true,
	"x-api-key":             true,
	"x-zcode-authorization": true,
	"cookie":                true,
	"set-cookie":            true,
}

var (
	jwtRe    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]*`)
	bearerRe = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]{12,}`)
	keyRe    = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}`)
)

func redactHeader(h http.Header) map[string]string {
	if h == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if sensitiveHeaders[strings.ToLower(k)] {
			out[k] = "[redacted]"
			continue
		}
		out[k] = redactBody(strings.Join(v, ", "))
	}
	return out
}

func redactBody(s string) string {
	if s == "" {
		return ""
	}
	s = bearerRe.ReplaceAllString(s, "Bearer [redacted]")
	s = jwtRe.ReplaceAllString(s, "[redacted-jwt]")
	s = keyRe.ReplaceAllString(s, "[redacted-key]")
	return s
}

func cut(raw []byte, max int, already bool) ([]byte, bool) {
	if max <= 0 || len(raw) <= max {
		out := make([]byte, len(raw))
		copy(out, raw)
		return out, already
	}
	out := make([]byte, max)
	copy(out, raw[:max])
	return out, true
}
