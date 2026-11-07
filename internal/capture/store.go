package capture

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var store ring

const captureFileCompactBytes = 8 << 20

// CaptureFilePath is where the ring is appended when capture is enabled.
// ZCODE_CAPTURE_FILE wins, then ZCODE_CAPTURE_DIR, else data/capture.jsonl.
// Only redacted exchanges are written.
func CaptureFilePath() string {
	if v := strings.TrimSpace(os.Getenv("ZCODE_CAPTURE_FILE")); v != "" {
		return v
	}
	dir := strings.TrimSpace(os.Getenv("ZCODE_CAPTURE_DIR"))
	if dir == "" {
		dir = "data"
	}
	return filepath.Join(dir, "capture.jsonl")
}

// ring is a fixed-capacity circular buffer: oldest exchanges are overwritten
// once capacity is reached.
type ring struct {
	mu       sync.Mutex
	items    []*Exchange
	start    int
	n        int
	capacity int
	nextID   int64
}

func (r *ring) reset(capacity int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if capacity <= 0 {
		capacity = settings.capacity
	}
	r.items = nil
	r.start = 0
	r.n = 0
	r.capacity = capacity
	r.nextID = 0
}

func (r *ring) add(ex *Exchange, events []Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.insertLocked(ex, events)
	r.persistLocked()
}

func (r *ring) insertLocked(ex *Exchange, events []Event) {
	if events != nil {
		ex.Events = events
	}
	if r.capacity <= 0 {
		r.capacity = settings.capacity
	}
	if ex.ID == 0 {
		r.nextID++
		ex.ID = r.nextID
	} else if ex.ID > r.nextID {
		r.nextID = ex.ID
	}
	if len(r.items) < r.capacity {
		r.items = append(r.items, ex)
		r.n = len(r.items)
		return
	}
	if r.capacity <= 0 {
		return
	}
	r.items[r.start] = ex
	r.start = (r.start + 1) % r.capacity
	r.n = r.capacity
}

// restore reloads the newest exchanges from the persistence file, keeping
// their ids so a /inspect?id= link still resolves after a restart.
func (r *ring) restore(path string, capacity int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.capacity = capacity
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ex Exchange
		if json.Unmarshal(line, &ex) != nil {
			continue
		}
		r.insertLocked(&ex, nil)
	}
}

// persistLocked appends the current newest exchange; caller holds r.mu. IO
// failures never break a request — capture is diagnostic, not critical.
func (r *ring) persistLocked() {
	if !settings.persist {
		return
	}
	path := CaptureFilePath()
	if path == "" || r.n == 0 {
		return
	}
	ex := r.items[(r.start+r.n-1)%len(r.items)]
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	line, err := json.Marshal(ex)
	if err == nil {
		_, _ = f.Write(append(line, '\n'))
	}
	_ = f.Close()
	if info, err := os.Stat(path); err == nil && info.Size() > captureFileCompactBytes {
		r.compactLocked(path)
	}
}

// compactLocked rewrites the file from the bounded ring so it cannot grow
// without limit. Caller holds r.mu.
func (r *ring) compactLocked(path string) {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)
	for _, ex := range r.ordered() {
		if line, err := json.Marshal(ex); err == nil {
			_, _ = w.Write(append(line, '\n'))
		}
	}
	if w.Flush() == nil {
		_ = f.Close()
		_ = os.Rename(tmp, path)
		return
	}
	_ = f.Close()
	_ = os.Remove(tmp)
}

// ordered returns the ring contents oldest-first. Caller holds r.mu.
func (r *ring) ordered() []*Exchange {
	if r.n == 0 {
		return nil
	}
	out := make([]*Exchange, 0, r.n)
	for i := range r.n {
		out = append(out, r.items[(r.start+i)%len(r.items)])
	}
	return out
}

// List returns up to limit summaries, newest first.
func List(limit int) []Summary {
	st := Status()
	if !st.Enabled {
		return []Summary{}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	ordered := store.ordered()
	if limit <= 0 || limit > len(ordered) {
		limit = len(ordered)
	}
	out := make([]Summary, 0, limit)
	for i := len(ordered) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, summarize(ordered[i]))
	}
	return out
}

// Get returns one full exchange (bodies included) by id.
func Get(id int64) (Exchange, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, ex := range store.ordered() {
		if ex.ID == id {
			return clone(*ex), true
		}
	}
	return Exchange{}, false
}

// Clear drops every captured exchange and reports how many were dropped.
func Clear() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	dropped := store.n
	store.items = nil
	store.start = 0
	store.n = 0
	_ = os.Truncate(CaptureFilePath(), 0)
	return dropped
}

// Status reports whether capture is on and how full the ring is.
func Status() State {
	load()
	store.mu.Lock()
	defer store.mu.Unlock()
	capacity := store.capacity
	if capacity <= 0 {
		capacity = settings.capacity
	}
	return State{
		Enabled:  settings.enabled,
		Count:    store.n,
		Capacity: capacity,
		BodyMax:  settings.bodyMax,
	}
}

func summarize(ex *Exchange) Summary {
	return Summary{
		ID:            ex.ID,
		StartedAt:     ex.StartedAt,
		Method:        ex.Method,
		Path:          ex.Path,
		Route:         ex.Route,
		Model:         ex.Model,
		Account:       ex.Account,
		Stream:        ex.Stream,
		Status:        ex.Status,
		ErrorCode:     ex.ErrorCode,
		DurationMS:    ex.DurationMS,
		FirstTokenMS:  ex.FirstTokenMS,
		RequestBytes:  ex.RequestBytes,
		ResponseBytes: ex.ResponseBytes,
		Truncated:     ex.Truncated,
	}
}

func clone(ex Exchange) Exchange {
	ex.RequestHeaders = copyMap(ex.RequestHeaders)
	ex.ResponseHeaders = copyMap(ex.ResponseHeaders)
	if ex.Events != nil {
		ex.Events = append([]Event(nil), ex.Events...)
	}
	return ex
}

func copyMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
