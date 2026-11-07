//go:build !windows

package main

import (
	"fmt"
	"runtime"
)

func runCompanion(bool) error {
	return fmt.Errorf("desktop companion hanya tersedia di Windows (host ini %s)", runtime.GOOS)
}

func runDiagnose() error { return runCompanion(true) }

func setAutostart(bool) error { return runCompanion(true) }
