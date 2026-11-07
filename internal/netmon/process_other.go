//go:build !windows

package netmon

import "time"

// readProcess is a no-op off Windows; gating keeps the card hidden.
func readProcess(now time.Time) procRaw {
	return procRaw{at: now}
}
