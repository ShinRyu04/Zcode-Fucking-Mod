package server

import (
	"os"
	"sync"
	"time"

	"github.com/azrinaldi/zcode-bridge/internal/desktop"
)

// desktopCache keeps the last successful snapshot of the ZCode CLI usage
// database. Reading costs ~100 ms on a 127 MB file, so the panel never pays
// that price per request.
type desktopCache struct {
	mu    sync.Mutex
	at    time.Time
	snap  *desktop.Snapshot
	err   error
	path  string
	ttl   time.Duration
	force bool
}

var desktopUsage = &desktopCache{ttl: 5 * time.Second}

func desktopEnabled() bool {
	switch os.Getenv("ZCODE_DESKTOP") {
	case "0", "off", "false":
		return false
	}
	return true
}

// snapshot returns the cached desktop snapshot, re-reading when stale.
func (c *desktopCache) snapshot() (*desktop.Snapshot, error) {
	if !desktopEnabled() {
		return nil, desktop.ErrUnavailable
	}
	path := desktop.DefaultPath()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snap != nil && c.path == path && !c.force && time.Since(c.at) < c.ttl {
		return c.snap, nil
	}
	snap, err := desktop.Read(path, time.Now())
	c.at = time.Now()
	c.path = path
	c.force = false
	if err != nil {
		c.snap = nil
		c.err = err
		return nil, err
	}
	c.snap = snap
	c.err = nil
	return snap, nil
}

// desktopPayload is what /v1/usage exposes for the desktop application.
type desktopPayload struct {
	Available bool              `json:"available"`
	Path      string            `json:"path,omitempty"`
	Error     string            `json:"error,omitempty"`
	Snapshot  *desktop.Snapshot `json:"snapshot,omitempty"`
}

func currentDesktop() desktopPayload {
	snap, err := desktopUsage.snapshot()
	if err != nil || snap == nil {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		return desktopPayload{Available: false, Path: desktop.DefaultPath(), Error: msg}
	}
	return desktopPayload{Available: true, Path: snap.Path, Snapshot: snap}
}

func resetDesktopForTest() {
	desktopUsage.mu.Lock()
	desktopUsage.snap = nil
	desktopUsage.err = nil
	desktopUsage.at = time.Time{}
	desktopUsage.path = ""
	desktopUsage.mu.Unlock()
}
