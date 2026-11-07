// Package update checks GitHub releases for this fork. It only notifies:
// downloads are never applied in place (the release carries source + exe,
// and the companion/bridge may be running).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository that publishes this fork's releases.
const Repo = "ShinRyu04/zcode-bridge-mod"

// Release is the newest published release.
type Release struct {
	Tag  string
	URL  string
	Name string
}

// Enabled reports whether the startup check is on (default on).
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ZCODE_UPDATE_CHECK"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

// Latest fetches the newest release tag.
func Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "zcode-bridge-update-check")
	res, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil // no release published yet
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: HTTP %d", res.StatusCode)
	}
	var gr githubRelease
	if err := json.NewDecoder(res.Body).Decode(&gr); err != nil {
		return nil, err
	}
	if strings.TrimSpace(gr.TagName) == "" {
		return nil, fmt.Errorf("github releases: empty tag")
	}
	return &Release{Tag: gr.TagName, URL: gr.HTMLURL, Name: gr.Name}, nil
}

// Check returns a human line for one current version. An empty string means
// "already latest" or no release exists; err means the check was unavailable.
func Check(ctx context.Context, current string) (string, error) {
	rel, err := Latest(ctx)
	if err != nil {
		return "", err
	}
	if rel == nil {
		return "", nil
	}
	if !Greater(rel.Tag, current) {
		return "", nil
	}
	return fmt.Sprintf("versi baru %s tersedia (sekarang v%s): %s", rel.Tag, strings.TrimPrefix(current, "v"), rel.URL), nil
}

// Greater reports whether tag a is a newer dotted version than b. Pre-release
// suffixes and a leading v are ignored; non-numeric segments compare as 0.
func Greater(a, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)
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

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		out = append(out, n)
	}
	return out
}
