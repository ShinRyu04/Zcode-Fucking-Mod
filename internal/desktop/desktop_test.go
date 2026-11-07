package desktop

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAggregateWindows(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	ms := func(d time.Duration) int64 { return now.Add(d).UnixMilli() }
	calls := []Call{
		{Model: "glm-5.3-flash", Status: "completed", Input: 10, Output: 100, Reasoning: 50, CacheCreate: 5,
			StartedAt: ms(-8 * time.Second), FirstTokenAt: ms(-5 * time.Second), CompletedAt: ms(-3 * time.Second)},
		{Model: "GLM-5.3", Status: "completed", Input: 20, Output: 1000,
			StartedAt: ms(-10 * time.Second), FirstTokenAt: ms(-6 * time.Second), CompletedAt: ms(-4 * time.Second)},
		{Model: "glm-5.3-flash", Status: "completed", Output: 400,
			StartedAt: -72 * time.Hour.Milliseconds() + ms(0), FirstTokenAt: ms(-72 * time.Hour), CompletedAt: ms(-72*time.Hour + time.Second)},
		{Model: "glm-5.3", Status: "completed", Output: 5000,
			StartedAt: ms(-240 * time.Hour), FirstTokenAt: ms(-240 * time.Hour), CompletedAt: ms(-240*time.Hour + time.Second)},
		{Model: "glm-5.3", Status: "cancelled", Output: 999, CompletedAt: ms(-time.Minute)},
	}
	snap := aggregate(calls, now)

	if snap.Today.Requests != 2 {
		t.Fatalf("today requests = %d, want 2", snap.Today.Requests)
	}
	if snap.Today.Tokens != 10+100+50+5+20+1000 {
		t.Fatalf("today tokens = %d", snap.Today.Tokens)
	}
	wantAvg := float64(150+1000) * 1000 / float64(2000+2000)
	if diff := snap.Today.AvgTPS - wantAvg; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("today avg = %f, want %f", snap.Today.AvgTPS, wantAvg)
	}
	if snap.High7d != 500 {
		t.Fatalf("7d high = %f, want 500", snap.High7d)
	}
	if snap.AllTime.Requests != 4 {
		t.Fatalf("all time requests = %d, want 4 (cancelled excluded)", snap.AllTime.Requests)
	}
	if len(snap.Models) != 2 || snap.Models[0].Model != "glm-5.3" {
		t.Fatalf("models = %+v", snap.Models)
	}
	var fifteenth *Range
	for i := range snap.Ranges {
		if snap.Ranges[i].Key == "15m" {
			fifteenth = &snap.Ranges[i]
		}
	}
	if fifteenth == nil || len(fifteenth.Buckets) != 90 {
		t.Fatalf("15m range missing or wrong size")
	}
	nonzero := 0
	for _, b := range fifteenth.Buckets {
		if b.Tokens > 0 {
			nonzero++
		}
	}
	if nonzero == 0 {
		t.Fatal("15m buckets are empty for today's calls")
	}
}

func TestReadFixture(t *testing.T) {
	// The fixture timestamps are milliseconds after the epoch; pick a clock a
	// second later so the open assistant message still counts as in flight.
	now := time.UnixMilli(1000)
	snap, err := Read(filepath.Join("..", "sqlite", "testdata", "usage.db"), now)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if snap.AllTime.Requests != 2 {
		t.Fatalf("completed calls = %d, want 2", snap.AllTime.Requests)
	}
	if snap.InFlight != 1 {
		t.Fatalf("in flight = %d, want 1", snap.InFlight)
	}
	if snap.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", snap.Sessions)
	}
	if len(snap.Models) != 2 {
		t.Fatalf("models = %+v", snap.Models)
	}
}

func TestReadMissingDatabase(t *testing.T) {
	if _, err := Read(filepath.Join("testdata", "nope.sqlite"), time.Now()); err == nil {
		t.Fatal("expected an error for a missing database")
	}
}
