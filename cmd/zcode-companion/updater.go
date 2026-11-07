package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	releaseRepo = "ShinRyu04/zcode-bridge-mod"
	releaseAPI  = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
)

// releaseInfo is the outcome of an update check. A project that publishes no
// releases (this fork) yields Note instead of an error.
type releaseInfo struct {
	Current string
	Latest  string
	Newer   bool
	URL     string
	Note    string
}

func (r releaseInfo) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "versi sekarang: %s\n", r.Current)
	if r.Latest != "" {
		fmt.Fprintf(&b, "rilis terbaru: %s\n", r.Latest)
	}
	if r.Newer {
		b.WriteString("pembaruan tersedia; unduh dari " + r.URL + "\n")
	} else if r.Latest != "" {
		b.WriteString("sudah versi terbaru\n")
	}
	if r.Note != "" {
		b.WriteString(r.Note + "\n")
	}
	return b.String()
}

// checkLatestRelease queries the public GitHub API for the newest release tag.
// A missing release (404) is reported, not treated as a failure: this fork
// ships source only.
func checkLatestRelease(current string) (releaseInfo, error) {
	out := releaseInfo{Current: current}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, releaseAPI, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "zcode-companion/"+companionVersion)
	res, err := client.Do(req)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound {
		out.Note = "repo ini belum menerbitkan rilis; build dari sumber (go build ./cmd/zcode-bridge)"
		return out, nil
	}
	if res.StatusCode != http.StatusOK {
		return out, fmt.Errorf("github api status %d", res.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return out, fmt.Errorf("parse github reply: %w", err)
	}
	out.Latest = payload.TagName
	out.URL = payload.HTMLURL
	out.Newer = versionGreater(out.Latest, current)
	if !out.Newer && out.Latest == "" {
		out.Note = "rilis tidak memuat tag versi"
	}
	return out, nil
}

// versionGreater reports whether a is a newer dotted version than b. Prefixes
// like "v" and any pre-release suffix are ignored; unparsable versions are
// never considered newer.
func versionGreater(a, b string) bool {
	pa, oka := parseVersion(a)
	pb, okb := parseVersion(b)
	if !oka || !okb {
		return false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "v"))
	if v == "" {
		return nil, false
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
