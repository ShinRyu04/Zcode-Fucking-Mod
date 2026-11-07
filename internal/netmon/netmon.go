// Package netmon samples local network counters and TCP connection state
// without external dependencies. On Windows the numbers come straight from
// iphlpapi (GetIfTable / GetExtendedTcpTable); elsewhere the package reports
// Supported=false so the dashboard can hide the card instead of guessing.
package netmon

import (
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Interface is one adapter's rate and lifetime counters.
type Interface struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Index       uint32  `json:"index"`
	Up          bool    `json:"up"`
	InBytes     uint64  `json:"in_bytes"`
	OutBytes    uint64  `json:"out_bytes"`
	InBps       float64 `json:"in_bps"`
	OutBps      float64 `json:"out_bps"`
}

// ProcessStats is the matched ZCode client process tree: transfer counters,
// deltas, CPU share and the calibration factor applied to the rates.
type ProcessStats struct {
	Found       bool     `json:"found"`
	Name        string   `json:"name,omitempty"`
	PIDs        []uint32 `json:"pids,omitempty"`
	ReadBytes   uint64   `json:"read_bytes"`
	WriteBytes  uint64   `json:"write_bytes"`
	ReadBps     float64  `json:"read_bps"`
	WriteBps    float64  `json:"write_bps"`
	CPUPercent  float64  `json:"cpu_percent"`
	Calibration float64  `json:"calibration"`
}

// Snapshot is the GET /v1/net payload.
type Snapshot struct {
	At             time.Time     `json:"at"`
	Supported      bool          `json:"supported"`
	Error          string        `json:"error,omitempty"`
	TotalInBps     float64       `json:"total_in_bps"`
	TotalOutBps    float64       `json:"total_out_bps"`
	Interfaces     []Interface   `json:"interfaces"`
	TCPConnections int           `json:"tcp_connections"`
	TCPEstablished int           `json:"tcp_established"`
	Process        *ProcessStats `json:"process,omitempty"`
}

// rawSample is one device read before rates are derived.
type rawSample struct {
	at    time.Time
	ifs   []Interface
	total int
	estab int
	err   error
}

type sampler struct {
	once         sync.Once
	port         int
	mu           sync.Mutex
	last         Snapshot
	previous     *rawSample
	previousProc *procRaw
}

// procRaw is one read of the ZCode client process tree counters.
type procRaw struct {
	at      time.Time
	found   bool
	name    string
	pids    []uint32
	read    uint64
	write   uint64
	cpu100n uint64
}

// processCalibration is a display scale applied to process transfer rates.
// The IO counters include file and pipe traffic, so a user comparing the card
// with interface counters may set ZCODE_PROCESS_CALIBRATION (default 1).
func processCalibration() float64 {
	raw := os.Getenv("ZCODE_PROCESS_CALIBRATION")
	if raw == "" {
		return 1
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 || value > 1000 {
		return 1
	}
	return value
}

var live = &sampler{}

// readFn is swapped by tests to drive deterministic counter deltas.
var readFn = read

// Start begins sampling once. port is the local bridge port used to
// attribute TCP connections; 0 counts every connection.
func Start(port int) {
	live.once.Do(func() {
		live.port = port
		live.refresh()
		go live.loop()
	})
}

func (s *sampler) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.refresh()
	}
}

func (s *sampler) refresh() {
	now := time.Now()
	sample := readFn(now)
	proc := readProcess(now)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.previous == nil {
		s.last = Snapshot{At: now, Supported: sample.err == nil, Interfaces: sample.ifs,
			TCPConnections: sample.total, TCPEstablished: sample.estab, Process: processStats(proc, nil, 0)}
		if sample.err != nil {
			s.last.Error = sample.err.Error()
		}
		s.previous = &sample
		s.previousProc = &proc
		return
	}
	elapsed := now.Sub(s.previous.at).Seconds()
	prev := map[uint32]Interface{}
	for _, iface := range s.previous.ifs {
		prev[iface.Index] = iface
	}
	snap := Snapshot{At: now, Supported: sample.err == nil,
		TCPConnections: sample.total, TCPEstablished: sample.estab}
	if sample.err != nil {
		snap.Error = sample.err.Error()
	}
	for i := range sample.ifs {
		iface := &sample.ifs[i]
		if before, ok := prev[iface.Index]; ok && elapsed > 0 {
			if iface.InBytes >= before.InBytes {
				iface.InBps = float64(iface.InBytes-before.InBytes) / elapsed
			}
			if iface.OutBytes >= before.OutBytes {
				iface.OutBps = float64(iface.OutBytes-before.OutBytes) / elapsed
			}
		}
		snap.TotalInBps += iface.InBps
		snap.TotalOutBps += iface.OutBps
	}
	sortInterfaces(sample.ifs)
	snap.Interfaces = sample.ifs
	snap.Process = processStats(proc, s.previousProc, elapsed)
	s.last = snap
	s.previous = &sample
	s.previousProc = &proc
}

// processStats converts raw counters into displayed rates, guarding against
// counter wrap and against a process that was not running for the whole
// window. Gating: Found=false means the card hides the process rows entirely.
func processStats(current procRaw, previous *procRaw, elapsed float64) *ProcessStats {
	calibration := processCalibration()
	stats := &ProcessStats{
		Found: current.found, Name: current.name, PIDs: current.pids,
		ReadBytes: current.read, WriteBytes: current.write, Calibration: calibration,
	}
	if previous == nil || !current.found || !previous.found || elapsed <= 0 {
		return stats
	}
	if current.read >= previous.read {
		stats.ReadBps = float64(current.read-previous.read) / elapsed * calibration
	}
	if current.write >= previous.write {
		stats.WriteBps = float64(current.write-previous.write) / elapsed * calibration
	}
	if current.cpu100n >= previous.cpu100n {
		stats.CPUPercent = float64(current.cpu100n-previous.cpu100n) / 1e7 / elapsed * 100
	}
	return stats
}

// Current returns the newest sample. Rates are zero until two reads landed.
func Current() Snapshot {
	live.mu.Lock()
	defer live.mu.Unlock()
	snap := live.last
	snap.Interfaces = append([]Interface(nil), live.last.Interfaces...)
	return snap
}

// ResetForTest clears the sampler state so tests can drive refresh directly.
func ResetForTest(port int) {
	live.mu.Lock()
	live.last = Snapshot{}
	live.previous = nil
	live.previousProc = nil
	live.port = port
	live.mu.Unlock()
}

// sortInterfaces puts the busiest adapters first so the card lists the ones
// that matter without client-side work.
func sortInterfaces(ifs []Interface) {
	sort.SliceStable(ifs, func(i, j int) bool {
		left := ifs[i].InBps + ifs[i].OutBps
		right := ifs[j].InBps + ifs[j].OutBps
		return left > right
	})
}
