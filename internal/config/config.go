// Package config loads the optional config.yaml and maps its keys onto the
// environment variables the rest of the bridge already reads (env always
// wins, so Docker/env deployments never need the file). Only the scalar
// subset used by this fork is supported: nested mapping keys, quoted or bare
// values, `#` comments. No third-party YAML library.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// envFor maps dotted config keys to bridge environment variables.
var envFor = map[string]string{
	"server.host":         "HOST",
	"server.port":         "PORT",
	"auth.proxyApiKey":    "ZCODE_BRIDGE_TOKEN",
	"defaultModel":        "ZCODE_DEFAULT_MODEL",
	"capture.enabled":     "ZCODE_CAPTURE",
	"capture.capacity":    "ZCODE_CAPTURE_SIZE",
	"updateCheck.enabled": "ZCODE_UPDATE_CHECK",

	"async.enabled":             "ZCODE_ASYNC_ENABLED",
	"async.origin":              "ZCODE_ASYNC_ORIGIN",
	"async.maxRetries":          "ZCODE_ASYNC_MAX_RETRIES",
	"async.maxWaitMs":           "ZCODE_ASYNC_MAX_WAIT_MS",
	"async.pollIntervalMs":      "ZCODE_ASYNC_POLL_MS",
	"async.keepAliveIntervalMs": "ZCODE_ASYNC_KEEPALIVE_MS",
	"async.controlTimeoutMs":    "ZCODE_ASYNC_CONTROL_TIMEOUT_MS",

	"claim.enabled":        "ZCODE_CLAIM_ENABLED",
	"claim.auto":           "ZCODE_CLAIM_AUTO",
	"claim.origin":         "ZCODE_CLAIM_ORIGIN",
	"claim.pollIntervalMs": "ZCODE_CLAIM_POLL_INTERVAL_MS",
	"claim.cooldownMs":     "ZCODE_CLAIM_COOLDOWN_MS",
	"claim.planId":         "ZCODE_CLAIM_PLAN_ID",
}

// FilePath returns the config file to load: ZCODE_CONFIG when set, else
// ./config.yaml when it exists, else "" (nothing to do).
func FilePath() string {
	if p := strings.TrimSpace(os.Getenv("ZCODE_CONFIG")); p != "" {
		return p
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return ""
}

// Load reads the config file (if any) and applies each mapped key with
// environment-over-file precedence. A missing file is not an error; a
// malformed key with no mapping is ignored so foreign config keys do not
// break startup.
func Load() error {
	path := FilePath()
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: %w", err)
	}
	defer f.Close()

	flat := map[string]string{}
	stack := []string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := stripComment(sc.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		depth := indent / 2
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			continue // lists are informational (models); nothing maps them
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if depth < len(stack) {
			stack = stack[:depth]
		}
		if val == "" {
			if depth == len(stack) {
				stack = append(stack, key)
			} else {
				stack = append(stack[:depth], key)
			}
			continue
		}
		full := key
		if depth > 0 && depth <= len(stack) {
			full = strings.Join(append(append([]string{}, stack[:depth]...), key), ".")
		}
		flat[full] = unquote(val)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	for key, val := range flat {
		env, ok := envFor[key]
		if !ok {
			continue
		}
		if strings.TrimSpace(os.Getenv(env)) == "" {
			_ = os.Setenv(env, val)
		}
	}
	return nil
}

// stripComment removes a trailing `#` comment that is not inside quotes.
func stripComment(line string) string {
	var quote rune
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			return line[:i]
		}
	}
	return line
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			if v[0] == '"' {
				if s, err := strconv.Unquote(v); err == nil {
					return s
				}
			}
			return v[1 : len(v)-1]
		}
	}
	return v
}
