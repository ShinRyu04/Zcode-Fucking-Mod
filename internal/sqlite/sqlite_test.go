package sqlite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openFixture(t *testing.T, name string) *DB {
	t.Helper()
	db, err := Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSchema(t *testing.T) {
	db := openFixture(t, "usage.db")
	for _, name := range []string{"model_usage", "message", "part"} {
		if db.Table(name) == nil {
			t.Fatalf("table %q missing", name)
		}
	}
	if db.Table("nope") != nil {
		t.Fatal("unknown table reported present")
	}

	usage := db.Table("model_usage")
	var names []string
	for _, c := range usage.Columns {
		names = append(names, c.Name)
	}
	want := []string{
		"id", "session_id", "model_id", "status", "started_at", "first_token_at",
		"completed_at", "input_tokens", "output_tokens", "reasoning_tokens",
		"cache_read_input_tokens", "cache_creation_input_tokens", "raw_usage_json",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("columns = %v, want %v", names, want)
	}
	if usage.Columns[4].Type != "INTEGER" {
		t.Fatalf("started_at type = %q", usage.Columns[4].Type)
	}
	if len(db.Tables()) != 3 {
		t.Fatalf("tables = %d, want 3", len(db.Tables()))
	}
}

func TestScanValues(t *testing.T) {
	db := openFixture(t, "usage.db")
	usage := db.Table("model_usage")
	idx := map[string]int{}
	for i, c := range usage.Columns {
		idx[c.Name] = i
	}
	rows := map[string][]Value{}
	err := usage.Scan(func(rowid int64, vals []Value) error {
		if rowid <= 0 {
			t.Fatalf("rowid %d for %q", rowid, vals[idx["id"]].String())
		}
		rows[vals[idx["id"]].String()] = vals
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	a := rows["a"]
	if got, _ := a[idx["started_at"]].Int64(); got != 1000 {
		t.Fatalf("started_at = %d", got)
	}
	if got, _ := a[idx["input_tokens"]].Int64(); got != 33799 {
		t.Fatalf("input_tokens = %d", got)
	}
	if a[idx["raw_usage_json"]].String() != `{"inputTokens":33799}` {
		t.Fatalf("raw json = %q", a[idx["raw_usage_json"]].String())
	}
	b := rows["b"]
	if !b[idx["first_token_at"]].IsNull() {
		t.Fatalf("first_token_at should be NULL, got %v", b[idx["first_token_at"]])
	}
	if !b[idx["raw_usage_json"]].IsNull() {
		t.Fatalf("raw_usage_json should be NULL")
	}
	if got, _ := rows["c"][idx["started_at"]].Int64(); got != -5 {
		t.Fatalf("negative integer = %d", got)
	}
}

func TestOverflowPayload(t *testing.T) {
	db := openFixture(t, "usage.db")
	part := db.Table("part")
	dataIdx := part.ColumnIndex("data")
	if dataIdx != 2 {
		t.Fatalf("data column index = %d", dataIdx)
	}
	found := false
	err := part.Scan(func(_ int64, vals []Value) error {
		text := vals[dataIdx].String()
		if len(text) != 20000 {
			t.Fatalf("payload length = %d", len(text))
		}
		if strings.Trim(text, "x") != "" {
			t.Fatal("payload content corrupted")
		}
		found = true
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !found {
		t.Fatal("part row not returned")
	}
}

// TestWALOverlay opens a database whose main file holds only page 1; every
// table page lives in the -wal file.
func TestWALOverlay(t *testing.T) {
	if _, err := os.Stat(filepath.Join("testdata", "usage_wal.db-wal")); err != nil {
		t.Skip("wal fixture missing")
	}
	info, err := os.Stat(filepath.Join("testdata", "usage_wal.db"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > int64(4096) {
		t.Fatalf("fixture main file = %d bytes, expected only page 1", info.Size())
	}
	db := openFixture(t, "usage_wal.db")
	usage := db.Table("model_usage")
	if usage == nil {
		t.Fatal("schema not recovered from WAL")
	}
	count := 0
	err = usage.Scan(func(_ int64, _ []Value) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if count != 4 {
		t.Fatalf("rows via WAL = %d, want 4", count)
	}
	part := db.Table("part")
	part.Scan(func(_ int64, vals []Value) error {
		if got := len(vals[2].String()); got != 20000 {
			t.Fatalf("wal payload length = %d", got)
		}
		return nil
	})
}
