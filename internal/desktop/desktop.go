// Package desktop reads the ZCode CLI usage database.
//
// The database is written by the desktop application, not by this bridge, so
// everything here is read-only and tolerant: a missing file, a schema change
// or a corrupt row degrades to "no data" instead of failing the request.
package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/sqlite"
)

// DefaultPath is where the ZCode CLI keeps its usage database.
func DefaultPath() string {
	if p := os.Getenv("ZCODE_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zcode", "cli", "db", "db.sqlite")
}

// Call is one model request recorded by the desktop application.
type Call struct {
	ID           string
	SessionID    string
	Model        string
	Status       string
	StartedAt    int64
	FirstTokenAt int64
	CompletedAt  int64
	Input        int64
	Output       int64
	Reasoning    int64
	CacheRead    int64
	CacheCreate  int64
}

// Generated is the token count used for speed: visible output plus thinking.
func (c Call) Generated() int64 { return c.Output + c.Reasoning }

// GenMillis is the pure generation duration, excluding the first-token wait.
func (c Call) GenMillis() int64 {
	if c.FirstTokenAt == 0 || c.CompletedAt <= c.FirstTokenAt {
		return 0
	}
	return c.CompletedAt - c.FirstTokenAt
}

// TPS is the measured speed of one finished call, or 0 when unmeasurable.
func (c Call) TPS() float64 {
	ms := c.GenMillis()
	if ms <= 0 {
		return 0
	}
	return float64(c.Generated()) * 1000 / float64(ms)
}

// Totals is one aggregation window.
type Totals struct {
	Requests    int    `json:"requests"`
	Input       int64  `json:"input_tokens"`
	Output      int64  `json:"output_tokens"`
	Reasoning   int64  `json:"reasoning_tokens"`
	CacheRead   int64  `json:"cache_read_tokens"`
	CacheCreate int64  `json:"cache_creation_tokens"`
	Tokens      int64  `json:"tokens"`
	AvgTPS      float64 `json:"avg_tps"`
}

// ModelStat is per-model speed and volume.
type ModelStat struct {
	Model    string  `json:"model"`
	Requests int     `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Share    float64 `json:"share"`
	AvgTPS   float64 `json:"avg_tps"`
	PeakTPS  float64 `json:"peak_tps"`
}

// Bucket is one chart column.
type Bucket struct {
	At     int64   `json:"at"`
	TPS    float64 `json:"tps"`
	Tokens int64   `json:"tokens"`
	Calls  int     `json:"calls"`
}

// Range is one chart range (15m, 1h, 6h, 24h).
type Range struct {
	Key      string   `json:"key"`
	BucketMS int64    `json:"bucket_ms"`
	Buckets  []Bucket `json:"buckets"`
}

// Snapshot is everything the panel needs from the desktop database.
type Snapshot struct {
	Path         string      `json:"path"`
	ReadAt       int64       `json:"read_at"`
	Today        Totals      `json:"today"`
	AllTime      Totals      `json:"all_time"`
	High7d       float64     `json:"high_7d"`
	Avg7d        float64     `json:"avg_7d"`
	Models       []ModelStat `json:"models"`
	Ranges       []Range     `json:"ranges"`
	InFlight     int         `json:"in_flight"`
	Sessions     int         `json:"sessions"`
	LastActivity int64       `json:"last_activity"`
}

var (
	// ErrUnavailable means the database cannot be read at all.
	ErrUnavailable = errors.New("desktop: usage database unavailable")
	rangeWidths    = []struct {
		key  string
		dur  time.Duration
		step time.Duration
	}{
		{"15m", 15 * time.Minute, 10 * time.Second},
		{"1h", time.Hour, 40 * time.Second},
		{"6h", 6 * time.Hour, 4 * time.Minute},
		{"24h", 24 * time.Hour, 16 * time.Minute},
	}
)

// Read snapshots the database. now is passed in so callers can test.
func Read(path string, now time.Time) (*Snapshot, error) {
	if path == "" {
		return nil, ErrUnavailable
	}
	if _, err := os.Stat(path); err != nil {
		return nil, ErrUnavailable
	}
	db, err := sqlite.Open(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer db.Close()

	calls, err := readCalls(db)
	if err != nil {
		return nil, err
	}
	inFlight, sessions, last, err := readActivity(db, now)
	if err != nil {
		// Activity is advisory; usage still stands on its own.
		inFlight, sessions, last = 0, 0, 0
	}
	snap := aggregate(calls, now)
	snap.Path = path
	snap.ReadAt = now.UnixMilli()
	snap.InFlight = inFlight
	if sessions > snap.Sessions {
		snap.Sessions = sessions
	}
	if last > snap.LastActivity {
		snap.LastActivity = last
	}
	return snap, nil
}

func readCalls(db *sqlite.DB) ([]Call, error) {
	table := db.Table("model_usage")
	if table == nil {
		return nil, ErrUnavailable
	}
	idx := map[string]int{}
	for i, c := range table.Columns {
		idx[strings.ToLower(c.Name)] = i
	}
	need := []string{"status", "started_at", "model_id", "output_tokens", "completed_at"}
	for _, n := range need {
		if _, ok := idx[n]; !ok {
			return nil, ErrUnavailable
		}
	}
	getText := func(v []sqlite.Value, name string) string {
		if i, ok := idx[name]; ok && i < len(v) {
			return v[i].String()
		}
		return ""
	}
	getInt := func(v []sqlite.Value, name string) int64 {
		if i, ok := idx[name]; ok && i < len(v) {
			n, _ := v[i].Int64()
			return n
		}
		return 0
	}
	var calls []Call
	err := table.Scan(func(_ int64, v []sqlite.Value) error {
		c := Call{
			ID:           getText(v, "id"),
			SessionID:    getText(v, "session_id"),
			Model:        strings.ToLower(getText(v, "model_id")),
			Status:       getText(v, "status"),
			StartedAt:    getInt(v, "started_at"),
			FirstTokenAt: getInt(v, "first_token_at"),
			CompletedAt:  getInt(v, "completed_at"),
			Input:        getInt(v, "input_tokens"),
			Output:       getInt(v, "output_tokens"),
			Reasoning:    getInt(v, "reasoning_tokens"),
			CacheRead:    getInt(v, "cache_read_input_tokens"),
			CacheCreate:  getInt(v, "cache_creation_input_tokens"),
		}
		calls = append(calls, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return calls, nil
}

type messageData struct {
	Role string `json:"role"`
	Time struct {
		Created   *int64 `json:"created"`
		Completed *int64 `json:"completed"`
	} `json:"time"`
}

// readActivity counts sessions whose newest assistant message has no
// completion stamp - the desktop application's "a call is running" signal.
func readActivity(db *sqlite.DB, now time.Time) (inFlight, sessions int, last int64, err error) {
	table := db.Table("message")
	if table == nil {
		return 0, 0, 0, ErrUnavailable
	}
	idx := map[string]int{}
	for i, c := range table.Columns {
		idx[strings.ToLower(c.Name)] = i
	}
	sessionIdx, ok1 := idx["session_id"]
	dataIdx, ok2 := idx["data"]
	createdIdx, ok3 := idx["time_created"]
	if !ok1 || !ok2 || !ok3 {
		return 0, 0, 0, ErrUnavailable
	}
	type latest struct {
		created int64
		done    bool
	}
	newest := map[string]latest{}
	seen := map[string]bool{}
	cutoff := now.Add(-30 * time.Minute).UnixMilli()
	err = table.Scan(func(_ int64, v []sqlite.Value) error {
		if sessionIdx >= len(v) || dataIdx >= len(v) || createdIdx >= len(v) {
			return nil
		}
		sess := v[sessionIdx].String()
		created, _ := v[createdIdx].Int64()
		seen[sess] = true
		if created > last {
			last = created
		}
		data := v[dataIdx].String()
		if !strings.Contains(data, `"assistant"`) {
			return nil
		}
		var md messageData
		if json.Unmarshal([]byte(data), &md) != nil || md.Role != "assistant" {
			return nil
		}
		cur, ok := newest[sess]
		if !ok || created > cur.created {
			newest[sess] = latest{created: created, done: md.Time.Completed != nil}
		}
		return nil
	})
	if err != nil {
		return 0, 0, 0, err
	}
	sessions = len(seen)
	for _, l := range newest {
		if !l.done && l.created >= cutoff {
			inFlight++
		}
	}
	return inFlight, sessions, last, nil
}

func aggregate(calls []Call, now time.Time) *Snapshot {
	snap := &Snapshot{}
	loc := now.Location()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	weekStart := todayStart.AddDate(0, 0, -6)
	todayMS := todayStart.UnixMilli()
	weekMS := weekStart.UnixMilli()

	modelTokens := map[string]int64{}
	modelReq := map[string]int{}
	modelGen := map[string]int64{}
	modelMS := map[string]int64{}
	modelPeak := map[string]float64{}
	modelAvgGen := map[string]int64{}
	modelAvgMS := map[string]int64{}

	var todayGen, todayGenMS, weekGen, weekGenMS, allGen, allGenMS int64
	sessionIDs := map[string]bool{}

	for _, c := range calls {
		if c.Status != "completed" {
			continue
		}
		total := c.Input + c.Output + c.Reasoning + c.CacheCreate
		addTotals(&snap.AllTime, c, total)
		allGen += c.Generated()
		allGenMS += c.GenMillis()
		if c.CompletedAt >= todayMS {
			addTotals(&snap.Today, c, total)
			todayGen += c.Generated()
			todayGenMS += c.GenMillis()
		}
		if c.CompletedAt >= weekMS {
			weekGen += c.Generated()
			weekGenMS += c.GenMillis()
			if c.Generated() >= 300 && c.GenMillis() >= 1000 {
				if tps := c.TPS(); tps > snap.High7d {
					snap.High7d = tps
				}
			}
		}
		sessionIDs[c.SessionID] = true
		m := strings.ToLower(c.Model)
		modelTokens[m] += total
		modelReq[m]++
		modelGen[m] += c.Generated()
		modelMS[m] += c.GenMillis()
		modelAvgGen[m] += c.Generated()
		modelAvgMS[m] += c.GenMillis()
		if tps := c.TPS(); tps > modelPeak[m] {
			modelPeak[m] = tps
		}
		if c.CompletedAt > snap.LastActivity {
			snap.LastActivity = c.CompletedAt
		}
	}
	if todayGenMS > 0 {
		snap.Today.AvgTPS = float64(todayGen) * 1000 / float64(todayGenMS)
	}
	if weekGenMS > 0 {
		snap.Avg7d = float64(weekGen) * 1000 / float64(weekGenMS)
	}
	if allGenMS > 0 {
		snap.AllTime.AvgTPS = float64(allGen) * 1000 / float64(allGenMS)
	}
	var grand int64
	for _, t := range modelTokens {
		grand += t
	}
	for m, req := range modelReq {
		st := ModelStat{Model: m, Requests: req, Tokens: modelTokens[m], PeakTPS: modelPeak[m]}
		if modelAvgMS[m] > 0 {
			st.AvgTPS = float64(modelAvgGen[m]) * 1000 / float64(modelAvgMS[m])
		}
		if grand > 0 {
			st.Share = float64(modelTokens[m]) / float64(grand)
		}
		snap.Models = append(snap.Models, st)
	}
	sort.Slice(snap.Models, func(i, j int) bool {
		if snap.Models[i].Tokens != snap.Models[j].Tokens {
			return snap.Models[i].Tokens > snap.Models[j].Tokens
		}
		return snap.Models[i].Model < snap.Models[j].Model
	})
	snap.Sessions = len(sessionIDs)
	for _, r := range rangeWidths {
		snap.Ranges = append(snap.Ranges, buildRange(r.key, r.dur, r.step, calls, now))
	}
	return snap
}

func addTotals(t *Totals, c Call, total int64) {
	t.Requests++
	t.Input += c.Input
	t.Output += c.Output
	t.Reasoning += c.Reasoning
	t.CacheRead += c.CacheRead
	t.CacheCreate += c.CacheCreate
	t.Tokens += total
}

// buildRange buckets finished calls by completion time and reports the
// weighted speed of each bucket, mirroring the panel's per-range queries.
func buildRange(key string, span, step time.Duration, calls []Call, now time.Time) Range {
	n := int(span / step)
	r := Range{Key: key, BucketMS: step.Milliseconds(), Buckets: make([]Bucket, n)}
	start := now.Add(-span)
	startMS := start.UnixMilli()
	stepMS := step.Milliseconds()
	type acc struct {
		gen, ms int64
		calls   int
	}
	accs := make([]acc, n)
	for _, c := range calls {
		if c.Status != "completed" || c.CompletedAt < startMS {
			continue
		}
		i := int((c.CompletedAt - startMS) / stepMS)
		if i < 0 || i >= n {
			continue
		}
		accs[i].gen += c.Generated()
		accs[i].ms += c.GenMillis()
		accs[i].calls++
	}
	for i := 0; i < n; i++ {
		b := Bucket{At: startMS + int64(i)*stepMS}
		if accs[i].ms > 0 {
			b.TPS = float64(accs[i].gen) * 1000 / float64(accs[i].ms)
		}
		b.Tokens = accs[i].gen
		b.Calls = accs[i].calls
		r.Buckets[i] = b
	}
	return r
}
