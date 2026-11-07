//go:build !windows

package tui

import (
	"bufio"
	"context"
	"os"
)

func consoleWidth() int { return 100 }

func enableANSI() {}

// RunInput reads lines on POSIX terminals; single-key raw mode would need
// termios, which the standard library does not expose portably.
func RunInput(ctx context.Context, keys chan<- byte) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		select {
		case keys <- line[0]:
		case <-ctx.Done():
			return
		}
	}
}
