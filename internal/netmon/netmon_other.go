//go:build !windows

package netmon

import "time"

// read reports unsupported on non-Windows hosts; the dashboard hides the
// network card when Supported is false.
func read(now time.Time) rawSample {
	return rawSample{at: now, err: errUnsupported}
}

var errUnsupported = platformError("netmon: interface counters require Windows iphlpapi")

type platformError string

func (e platformError) Error() string { return string(e) }
