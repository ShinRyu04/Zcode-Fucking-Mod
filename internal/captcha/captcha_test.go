package captcha

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func validParam() string {
	raw, _ := json.Marshal(map[string]any{
		"certifyId":     "c1",
		"securityToken": strings.Repeat("s", 60),
		"isSign":        true,
	})
	return base64.StdEncoding.EncodeToString(raw)
}

func TestParseOutput(t *testing.T) {
	p, r, err := ParseOutput("noise\nVERIFY_PARAM=" + validParam() + "\nVERIFY_REGION=sgp\n")
	if err != nil {
		t.Fatal(err)
	}
	if p != validParam() || r != "sgp" {
		t.Fatalf("param/region = %q/%q", p, r)
	}
	if _, _, err := ParseOutput("nothing"); err == nil {
		t.Fatal("missing param must fail")
	}
	short, _ := json.Marshal(map[string]any{"certifyId": "c", "securityToken": "short", "isSign": true})
	if _, _, err := ParseOutput("VERIFY_PARAM=" + base64.StdEncoding.EncodeToString(short)); err == nil {
		t.Fatal("degraded param must fail")
	}
}

func TestReady(t *testing.T) {
	if !Ready(validParam()) {
		t.Fatal("valid param rejected")
	}
	raw, _ := json.Marshal(map[string]any{"certifyId": "c", "securityToken": strings.Repeat("s", 60), "isSign": false})
	if Ready(base64.StdEncoding.EncodeToString(raw)) {
		t.Fatal("unsigned param accepted")
	}
	if Ready("not-base64") {
		t.Fatal("garbage accepted")
	}
}
