// Package captcha mints an Aliyun verification param through an operator
// supplied command. The bridge ships no solver: point ZCODE_CAPTCHA_CMD at a
// program that prints "VERIFY_PARAM=<param>" (and optionally
// "VERIFY_REGION=<region>") on stdout.
package captcha

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ErrNotConfigured means ZCODE_CAPTCHA_CMD is unset or empty.
var ErrNotConfigured = errors.New("captcha minter not configured (set ZCODE_CAPTCHA_CMD)")

// Command returns the configured minter command line.
func Command() string { return strings.TrimSpace(os.Getenv("ZCODE_CAPTCHA_CMD")) }

// Configured reports whether an external minter is available.
func Configured() bool { return Command() != "" }

// runner is injectable for tests.
var runner = runCommand

func runCommand(ctx context.Context, cmdline string) (string, error) {
	args := strings.Fields(cmdline)
	if len(args) == 0 {
		return "", ErrNotConfigured
	}
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("captcha minter: %w", err)
	}
	return string(out), nil
}

// SetRunnerForTest overrides the process runner and returns a restore func.
func SetRunnerForTest(fn func(context.Context, string) (string, error)) func() {
	prev := runner
	if fn == nil {
		runner = runCommand
	} else {
		runner = fn
	}
	return func() { runner = prev }
}

// Mint runs the configured minter and returns a full-strength param.
func Mint(ctx context.Context) (param, region string, err error) {
	cmdline := Command()
	if cmdline == "" {
		return "", "", ErrNotConfigured
	}
	out, err := runner(ctx, cmdline)
	if err != nil {
		return "", "", err
	}
	return ParseOutput(out)
}

// ParseOutput extracts VERIFY_PARAM/VERIFY_REGION lines and rejects degraded
// values (a short securityToken means the SDK ran without its device VM).
func ParseOutput(out string) (param, region string, err error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "VERIFY_PARAM="); ok {
			param = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "VERIFY_REGION="); ok {
			region = strings.TrimSpace(v)
		}
	}
	if param == "" {
		return "", "", fmt.Errorf("captcha minter printed no VERIFY_PARAM")
	}
	if !Ready(param) {
		return "", "", fmt.Errorf("captcha minter returned a degraded param")
	}
	return param, region, nil
}

// Ready validates that s is a base64 JSON verify param carrying a real
// security token.
func Ready(s string) bool {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil {
			return false
		}
	}
	var p struct {
		SecurityToken string `json:"securityToken"`
		CertifyID     string `json:"certifyId"`
		IsSign        bool   `json:"isSign"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return false
	}
	return len(p.SecurityToken) >= 50 && p.CertifyID != "" && p.IsSign
}
