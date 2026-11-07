package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFlattensNestedKeysWithoutOverridingEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `server:
  host: "0.0.0.0"
  port: 24680
auth:
  proxyApiKey: "file-token"   # trailing comment
async:
  enabled: true
  origin: https://example.test
claim:
  auto: false
  planId: "weekend-1"
capture:
  enabled: true
unmapped:
  nonsense: 1
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZCODE_CONFIG", path)
	t.Setenv("PORT", "9999")
	t.Setenv("HOST", "")
	t.Setenv("ZCODE_BRIDGE_TOKEN", "")
	t.Setenv("ZCODE_ASYNC_ENABLED", "")
	t.Setenv("ZCODE_ASYNC_ORIGIN", "")
	t.Setenv("ZCODE_CLAIM_AUTO", "")
	t.Setenv("ZCODE_CLAIM_PLAN_ID", "")
	t.Setenv("ZCODE_CAPTURE", "")

	if err := Load(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PORT"); got != "9999" {
		t.Fatalf("env must win over file: PORT=%q", got)
	}
	if got := os.Getenv("HOST"); got != "0.0.0.0" {
		t.Fatalf("HOST=%q", got)
	}
	if got := os.Getenv("ZCODE_BRIDGE_TOKEN"); got != "file-token" {
		t.Fatalf("token=%q", got)
	}
	if got := os.Getenv("ZCODE_ASYNC_ORIGIN"); got != "https://example.test" {
		t.Fatalf("async origin=%q", got)
	}
	if got := os.Getenv("ZCODE_CLAIM_AUTO"); got != "false" {
		t.Fatalf("claim auto=%q", got)
	}
	if got := os.Getenv("ZCODE_CLAIM_PLAN_ID"); got != "weekend-1" {
		t.Fatalf("claim plan=%q", got)
	}
	if got := os.Getenv("ZCODE_CAPTURE"); got != "true" {
		t.Fatalf("capture=%q", got)
	}
}

func TestMissingFileIsNotAnError(t *testing.T) {
	t.Setenv("ZCODE_CONFIG", filepath.Join(t.TempDir(), "absent.yaml"))
	if err := Load(); err != nil {
		t.Fatalf("missing config must be tolerated: %v", err)
	}
}

func TestStripCommentRespectsQuotes(t *testing.T) {
	if got := stripComment(`key: "a # b" # tail`); got != `key: "a # b" ` {
		t.Fatalf("stripComment = %q", got)
	}
}
