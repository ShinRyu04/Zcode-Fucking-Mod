package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAggregatesTodayVersusAllTime(t *testing.T) {
	ResetForTest(t.TempDir())
	now := time.Now()
	Record(Row{At: now.AddDate(0, 0, -1), Model: "glm-5.3-flash", InputTokens: 50, OutputTokens: 100, GenMS: 2000, TPS: 50})
	Record(Row{At: now, Model: "glm-5.3-flash", InputTokens: 5, OutputTokens: 10, CacheReadTokens: 3, GenMS: 1000, TPS: 10})

	snap := Current()
	if snap.Today.Requests != 1 || snap.Today.InputTokens != 5 || snap.Today.OutputTokens != 10 || snap.Today.CacheReadTokens != 3 {
		t.Fatalf("today = %#v", snap.Today)
	}
	if snap.Today.AvgTPS != 10 {
		t.Fatalf("today avg tps = %v", snap.Today.AvgTPS)
	}
	if snap.AllTime.Requests != 2 || snap.AllTime.OutputTokens != 110 {
		t.Fatalf("all time = %#v", snap.AllTime)
	}
	if snap.AllTime.AvgTPS != 36.7 {
		t.Fatalf("all time avg tps = %v", snap.AllTime.AvgTPS)
	}
	model, ok := snap.AllTime.PerModel["glm-5.3-flash"]
	if !ok || model.Requests != 2 || model.OutputTokens != 110 {
		t.Fatalf("per model = %#v", snap.AllTime.PerModel)
	}
	if len(snap.Recent) != 2 {
		t.Fatalf("recent = %d", len(snap.Recent))
	}
}

func TestRingsTrackLastHighAndSevenDayAverage(t *testing.T) {
	ResetForTest(t.TempDir())
	now := time.Now()
	Record(Row{At: now.AddDate(0, 0, -10), Model: "old", OutputTokens: 1000, GenMS: 1000, TPS: 1000})
	Record(Row{At: now.Add(-time.Hour), Model: "glm-5.3-flash", OutputTokens: 200, GenMS: 1000, TPS: 200})
	Record(Row{At: now, Model: "glm-5.3-flash", OutputTokens: 100, GenMS: 2000, TPS: 50})

	rings := Current().Rings
	if rings.LastTPS != 50 {
		t.Fatalf("last tps = %v", rings.LastTPS)
	}
	if rings.HighTPS != 1000 {
		t.Fatalf("high tps = %v", rings.HighTPS)
	}
	if rings.SevenDayAvgTPS != 100 {
		t.Fatalf("7d avg tps = %v", rings.SevenDayAvgTPS)
	}
}

func TestRowsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	ResetForTest(dir)
	Record(Row{At: time.Now(), Model: "glm-5.3-flash", InputTokens: 7, OutputTokens: 9, GenMS: 500, TPS: 18})
	ResetForTest(dir) // fresh store, same directory: must re-read the JSONL
	snap := Current()
	if snap.Today.Requests != 1 || snap.Today.InputTokens != 7 || snap.Today.OutputTokens != 9 {
		t.Fatalf("reloaded today = %#v", snap.Today)
	}
	if snap.Recent[0].TPS != 18 {
		t.Fatalf("reloaded row = %#v", snap.Recent)
	}
}

func TestDayRotationWritesOneFilePerDay(t *testing.T) {
	dir := t.TempDir()
	ResetForTest(dir)
	now := time.Now()
	Record(Row{At: now.AddDate(0, 0, -1), Model: "glm-5.3-flash", OutputTokens: 1})
	Record(Row{At: now, Model: "glm-5.3-flash", OutputTokens: 2})

	files, err := filepath.Glob(filepath.Join(dir, "usage-*.jsonl"))
	if err != nil || len(files) != 2 {
		t.Fatalf("files = %v (%v)", files, err)
	}
	want := map[string]bool{
		"usage-" + dayKey(now.AddDate(0, 0, -1)) + ".jsonl": true,
		"usage-" + dayKey(now) + ".jsonl":                   true,
	}
	for _, name := range files {
		if !want[filepath.Base(name)] {
			t.Fatalf("unexpected file %q in %v", name, want)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "usage-"+dayKey(now)+".jsonl"))
	if err != nil || len(data) == 0 {
		t.Fatalf("today file = %q (%v)", data, err)
	}
}

func TestMeterTracksTPSAndUsage(t *testing.T) {
	ResetForTest(t.TempDir())
	meter := Start("glm-5.3-flash", "", true)
	meter.SetAccount("acct_1")
	meter.FirstToken()
	time.Sleep(20 * time.Millisecond)
	meter.NoteUsage(map[string]any{"input_tokens": float64(12), "cache_read_input_tokens": float64(4)})
	meter.NoteUsage(map[string]any{"output_tokens": float64(20), "input_tokens": float64(12)})
	meter.Finish()
	meter.Finish() // idempotent

	snap := Current()
	if snap.Today.Requests != 1 {
		t.Fatalf("requests = %d", snap.Today.Requests)
	}
	if snap.Today.InputTokens != 12 || snap.Today.OutputTokens != 20 || snap.Today.CacheReadTokens != 4 {
		t.Fatalf("today = %#v", snap.Today)
	}
	if snap.Today.AvgTPS <= 0 {
		t.Fatalf("avg tps = %v", snap.Today.AvgTPS)
	}
	row := snap.Recent[0]
	if row.Account != "acct_1" || !row.Stream || row.TPS <= 0 || row.FirstTokenMS < 0 || row.GenMS <= 0 {
		t.Fatalf("row = %#v", row)
	}
}

func TestMeterWithoutUsageWritesNothing(t *testing.T) {
	ResetForTest(t.TempDir())
	meter := Start("glm-5.3-flash", "", false)
	meter.FirstToken()
	meter.Finish()
	if snap := Current(); snap.Today.Requests != 0 || len(snap.Recent) != 0 {
		t.Fatalf("aborted request was recorded: %#v", snap)
	}
}

func TestNonStreamRowsKeepTPSOutOfAverage(t *testing.T) {
	ResetForTest(t.TempDir())
	// Non-streaming: generation time is the whole latency and there is no
	// first token, so it must not dilute the streaming average.
	Record(Row{At: time.Now(), Model: "glm-5.3-flash", OutputTokens: 40, GenMS: 8000, TPS: 0})
	Record(Row{At: time.Now(), Model: "glm-5.3-flash", OutputTokens: 40, GenMS: 1000, TPS: 40, FirstTokenMS: 100})

	snap := Current()
	if snap.Today.AvgTPS != 40 {
		t.Fatalf("avg tps = %v", snap.Today.AvgTPS)
	}
	if snap.Today.Requests != 2 {
		t.Fatalf("requests = %d", snap.Today.Requests)
	}
}
