package netmon

import (
	"math"
	"testing"
	"time"
)

func TestProcessStatsApplyCalibrationAndGating(t *testing.T) {
	previous := &procRaw{at: time.Unix(100, 0), found: true, read: 1000, write: 2000, cpu100n: 1e7}
	current := procRaw{found: true, name: "ZCode.exe", pids: []uint32{7}, read: 3000, write: 2000, cpu100n: 2e7}

	t.Setenv("ZCODE_PROCESS_CALIBRATION", "2")
	stats := processStats(current, previous, 2)
	if !stats.Found || stats.Name != "ZCode.exe" {
		t.Fatalf("stats = %#v", stats)
	}
	if stats.ReadBps != 2000 { // (2000 bytes / 2s) * 2
		t.Fatalf("read bps = %v", stats.ReadBps)
	}
	if stats.WriteBps != 0 {
		t.Fatalf("write bps = %v", stats.WriteBps)
	}
	if stats.CPUPercent != 50 { // 1s CPU over 2s wall
		t.Fatalf("cpu = %v", stats.CPUPercent)
	}
	if stats.Calibration != 2 {
		t.Fatalf("calibration = %v", stats.Calibration)
	}

	t.Setenv("ZCODE_PROCESS_CALIBRATION", "bogus")
	if got := processStats(current, previous, 2).Calibration; got != 1 {
		t.Fatalf("bogus calibration fell back to %v", got)
	}

	missing := processStats(procRaw{found: false}, previous, 2)
	if missing.Found {
		t.Fatalf("missing process reported found: %#v", missing)
	}
}

func TestRatesDeriveFromCounterDeltas(t *testing.T) {
	ResetForTest(0)
	original := readFn
	defer func() {
		readFn = original
		ResetForTest(0)
	}()

	samples := []rawSample{
		{ifs: []Interface{{Index: 1, Name: "eth0", InBytes: 1000, OutBytes: 2000}}, total: 4, estab: 2},
		{ifs: []Interface{{Index: 1, Name: "eth0", InBytes: 3000, OutBytes: 2000}}, total: 6, estab: 3},
	}
	next := 0
	readFn = func(now time.Time) rawSample {
		sample := samples[next]
		if next < len(samples)-1 {
			next++
		}
		sample.at = now
		return sample
	}

	live.refresh()
	live.mu.Lock()
	live.previous.at = time.Now().Add(-2 * time.Second)
	live.mu.Unlock()
	live.refresh()

	snap := Current()
	if !snap.Supported {
		t.Fatalf("expected supported sample: %#v", snap)
	}
	if len(snap.Interfaces) != 1 {
		t.Fatalf("interfaces = %d", len(snap.Interfaces))
	}
	if got := snap.Interfaces[0].InBps; math.Abs(got-1000) > 0.5 {
		t.Fatalf("in bps = %v", got)
	}
	if got := snap.Interfaces[0].OutBps; got != 0 {
		t.Fatalf("out bps = %v", got)
	}
	if snap.TotalInBps < 999.5 || snap.TotalInBps > 1000.5 || snap.TCPConnections != 6 || snap.TCPEstablished != 3 {
		t.Fatalf("snapshot = %#v", snap)
	}
}

func TestCounterResetDoesNotProduceNegativeRate(t *testing.T) {
	ResetForTest(0)
	original := readFn
	defer func() {
		readFn = original
		ResetForTest(0)
	}()

	samples := []rawSample{
		{ifs: []Interface{{Index: 2, Name: "wlan", InBytes: 9000, OutBytes: 5000}}},
		{ifs: []Interface{{Index: 2, Name: "wlan", InBytes: 100, OutBytes: 40}}},
	}
	next := 0
	readFn = func(now time.Time) rawSample {
		sample := samples[next]
		if next < len(samples)-1 {
			next++
		}
		sample.at = now
		return sample
	}

	live.refresh()
	live.mu.Lock()
	live.previous.at = time.Now().Add(-time.Second)
	live.mu.Unlock()
	live.refresh()

	if got := Current().Interfaces[0].InBps; got != 0 {
		t.Fatalf("wrapped counter produced %v", got)
	}
}
