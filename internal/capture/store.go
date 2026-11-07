package capture

import "sync"

var store ring

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
	if r.capacity <= 0 {
		r.capacity = settings.capacity
	}
	ex.Events = events
	r.nextID++
	ex.ID = r.nextID
	if len(r.items) < r.capacity {
		r.items = append(r.items, ex)
		r.n = len(r.items)
		return
	}
	r.items[r.start] = ex
	r.start = (r.start + 1) % r.capacity
	r.n = r.capacity
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
