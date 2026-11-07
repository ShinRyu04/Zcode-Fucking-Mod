// Package usage meters real throughput per completed request.
//
// The bridge sees the upstream SSE stream, so it does not need the io-byte
// estimation the ZCode speed panel uses: tokens come from the message_delta
// usage block, generation time from first-token -> completion.
//
// Persistence is one JSONL row per request in
// data/usage-YYYYMMDD.jsonl (dir 0700, file 0600). On first read the store
// loads the newest 10k rows across the newest files so today's totals and the
// sparkline survive a restart; no database, no dependency.
package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultDir matches the other local state (data/ is gitignored).
	DefaultDir = "data"
	// maxBootLines caps the startup scan.
	maxBootLines = 10000
	// recentLimit is how many rows the sparkline can draw.
	recentLimit = 200
	// tailBytes bounds one file read during the startup scan.
	tailBytes = 4 << 20
)

// Row is one completed request, as persisted.
type Row struct {
	At              time.Time `json:"at"`
	Model           string    `json:"model"`
	Account         string    `json:"account,omitempty"`
	Stream          bool      `json:"stream"`
	InputTokens     int64     `json:"input_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	CacheReadTokens int64     `json:"cache_read_tokens"`
	FirstTokenMS    int64     `json:"first_token_ms,omitempty"`
	GenMS           int64     `json:"gen_ms"`
	TPS             float64   `json:"tps"`
}

// ModelStats is a per-model aggregate.
type ModelStats struct {
	Requests        int64   `json:"requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	AvgTPS          float64 `json:"avg_tps"`
}

// Stats is one aggregate window (today or all time).
type Stats struct {
	Requests        int64                 `json:"requests"`
	InputTokens     int64                 `json:"input_tokens"`
	OutputTokens    int64                 `json:"output_tokens"`
	CacheReadTokens int64                 `json:"cache_read_tokens"`
	AvgTPS          float64               `json:"avg_tps"`
	PerModel        map[string]ModelStats `json:"per_model"`
}

// Snapshot is the GET /v1/usage payload.
type Snapshot struct {
	Today   Stats `json:"today"`
	AllTime Stats `json:"all_time"`
	Recent  []Row `json:"recent"`
}

type modelAcc struct {
	requests      int64
	input, output int64
	cache         int64
	genMS         int64
	genTokens     int64
}

type acc struct {
	requests      int64
	input, output int64
	cache         int64
	genMS         int64
	genTokens     int64
	models        map[string]*modelAcc
}

func newAcc() acc { return acc{models: map[string]*modelAcc{}} }

func (a *acc) add(row Row) {
	a.requests++
	a.input += row.InputTokens
	a.output += row.OutputTokens
	a.cache += row.CacheReadTokens
	if row.TPS > 0 && row.GenMS > 0 {
		a.genMS += row.GenMS
		a.genTokens += row.OutputTokens
	}
	m := a.models[row.Model]
	if m == nil {
		m = &modelAcc{}
		a.models[row.Model] = m
	}
	m.requests++
	m.input += row.InputTokens
	m.output += row.OutputTokens
	m.cache += row.CacheReadTokens
	if row.TPS > 0 && row.GenMS > 0 {
		m.genMS += row.GenMS
		m.genTokens += row.OutputTokens
	}
}

func (a *acc) stats() Stats {
	out := Stats{
		Requests: a.requests, InputTokens: a.input, OutputTokens: a.output,
		CacheReadTokens: a.cache, AvgTPS: tps(a.genTokens, a.genMS),
		PerModel: map[string]ModelStats{},
	}
	for name, m := range a.models {
		out.PerModel[name] = ModelStats{
			Requests: m.requests, InputTokens: m.input, OutputTokens: m.output,
			CacheReadTokens: m.cache, AvgTPS: tps(m.genTokens, m.genMS),
		}
	}
	return out
}

// tps is tokens over generation seconds. Rows without generation time (for
// example non-streaming calls, where no first token exists) fall back to 0
// rather than pretending the whole latency was generation.
func tps(tokens, genMS int64) float64 {
	if tokens <= 0 || genMS <= 0 {
		return 0
	}
	value := float64(tokens) / (float64(genMS) / 1000)
	return float64(int64(value*10+0.5)) / 10
}

type store struct {
	mu       sync.Mutex
	dir      string
	loaded   bool
	all      acc
	today    acc
	todayKey string
	recent   []Row
}

var current = &store{}

// ResetForTest wipes in-memory state and points the store at dir.
func ResetForTest(dir string) {
	current.mu.Lock()
	defer current.mu.Unlock()
	current.dir = dir
	current.loaded = false
	current.all = acc{}
	current.today = acc{}
	current.todayKey = ""
	current.recent = nil
}

func (s *store) storageDir() string {
	if s.dir != "" {
		return s.dir
	}
	if env := strings.TrimSpace(os.Getenv("ZCODE_USAGE_DIR")); env != "" {
		return env
	}
	return DefaultDir
}

// Record appends one row and updates the aggregates. Persistence failures are
// silent: metering must never break a proxied request.
func Record(row Row) {
	if row.At.IsZero() {
		row.At = time.Now()
	}
	if row.Model == "" {
		row.Model = "unknown"
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	current.ensureLoaded()
	current.apply(row)
	current.persist(row)
}

func (s *store) ensureLoaded() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.all = newAcc()
	s.today = newAcc()
	s.todayKey = dayKey(time.Now())
	rows := loadRows(s.storageDir())
	for _, row := range rows {
		s.apply(row)
	}
}

func (s *store) apply(row Row) {
	s.all.add(row)
	key := dayKey(row.At)
	if key == s.todayKey {
		s.today.add(row)
	} else if key > s.todayKey {
		// Clock moved past midnight while running: roll today's window.
		s.todayKey = key
		s.today = newAcc()
		s.today.add(row)
	}
	s.recent = append(s.recent, row)
	if len(s.recent) > recentLimit {
		s.recent = s.recent[len(s.recent)-recentLimit:]
	}
}

func (s *store) persist(row Row) {
	dir := s.storageDir()
	if dir == "" {
		return
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	path := filepath.Join(dir, "usage-"+dayKey(row.At)+".jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = file.Write(append(encoded, '\n'))
	_ = file.Close()
}

// Current returns aggregates for today, all time and the recent rows.
func Current() Snapshot {
	current.mu.Lock()
	defer current.mu.Unlock()
	current.ensureLoaded()
	recent := append([]Row(nil), current.recent...)
	return Snapshot{
		Today:   current.today.stats(),
		AllTime: current.all.stats(),
		Recent:  recent,
	}
}

func dayKey(t time.Time) string {
	return t.Local().Format("20060102")
}

// loadRows reads the newest files and keeps at most maxBootLines of the
// newest rows.
func loadRows(dir string) []Row {
	files, _ := filepath.Glob(filepath.Join(dir, "usage-*.jsonl"))
	if len(files) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	var lines []string
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		chunk := tail(file)
		_ = file.Close()
		for _, line := range strings.Split(chunk, "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) >= maxBootLines {
			break
		}
	}
	if len(lines) > maxBootLines {
		lines = lines[len(lines)-maxBootLines:]
	}
	rows := make([]Row, 0, len(lines))
	for _, line := range lines {
		var row Row
		if json.Unmarshal([]byte(line), &row) == nil {
			rows = append(rows, row)
		}
	}
	return rows
}

// tail reads the last tailBytes of a file (a partial first line is dropped by
// the caller's JSON decode).
func tail(file *os.File) string {
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return ""
	}
	offset := info.Size() - tailBytes
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return ""
	}
	buf := make([]byte, info.Size()-offset)
	n, _ := file.Read(buf)
	text := string(buf[:n])
	if offset > 0 {
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		}
	}
	return text
}
