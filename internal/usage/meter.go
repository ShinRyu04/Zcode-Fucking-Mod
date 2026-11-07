package usage

import (
	"sync"
	"time"
)

// Meter measures one request. Create it before the upstream call, feed it the
// first streamed token and the usage blocks, then Finish once.
//
// A Meter that never saw a usage block records nothing: failures and aborted
// streams have no token truth to report.
type Meter struct {
	mu         sync.Mutex
	started    time.Time
	firstToken time.Time
	model      string
	account    string
	stream     bool
	input      int64
	output     int64
	cache      int64
	seen       bool
	finished   bool
}

// Start opens a meter for one request.
func Start(model, account string, stream bool) *Meter {
	return &Meter{started: time.Now(), model: model, account: account, stream: stream}
}

// SetAccount records the account that served the request (the transport hook
// reports it after the successful attempt).
func (m *Meter) SetAccount(id string) {
	if m == nil || id == "" {
		return
	}
	m.mu.Lock()
	if m.account == "" {
		m.account = id
	}
	m.mu.Unlock()
}

// FirstToken timestamps the first streamed delta.
func (m *Meter) FirstToken() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.firstToken.IsZero() {
		m.firstToken = time.Now()
	}
	m.mu.Unlock()
}

// NoteUsage merges an Anthropic usage object (message_start, message_delta or
// a non-streaming response). Values only grow, so a later block that repeats
// or omits a bucket cannot shrink the total.
func (m *Meter) NoteUsage(usage map[string]any) {
	if m == nil || usage == nil {
		return
	}
	num := func(key string) int64 {
		v, _ := usage[key].(float64)
		if v != v {
			return 0
		}
		return int64(v)
	}
	input, output, cache := num("input_tokens"), num("output_tokens"), num("cache_read_input_tokens")
	m.mu.Lock()
	if input > m.input {
		m.input = input
	}
	if output > m.output {
		m.output = output
	}
	if cache > m.cache {
		m.cache = cache
	}
	m.seen = true
	m.mu.Unlock()
}

// Finish writes the row. Safe to call more than once.
func (m *Meter) Finish() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.finished {
		m.mu.Unlock()
		return
	}
	m.finished = true
	seen, row := m.seen, Row{
		At: m.started, Model: m.model, Account: m.account, Stream: m.stream,
		InputTokens: m.input, OutputTokens: m.output, CacheReadTokens: m.cache,
	}
	first := m.firstToken
	m.mu.Unlock()
	if !seen {
		return
	}

	completed := time.Now()
	row.GenMS = completed.Sub(row.At).Milliseconds()
	if !first.IsZero() {
		row.FirstTokenMS = first.Sub(row.At).Milliseconds()
		row.GenMS = completed.Sub(first).Milliseconds()
		if row.GenMS > 0 {
			row.TPS = tps(row.OutputTokens, row.GenMS)
		}
	}
	Record(row)
}
