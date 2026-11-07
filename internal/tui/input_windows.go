//go:build windows

package tui

import (
	"context"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	kernel32t = syscall.NewLazyDLL("kernel32.dll")

	procGetStdHandle     = kernel32t.NewProc("GetStdHandle")
	procGetConsoleMode   = kernel32t.NewProc("GetConsoleMode")
	procSetConsoleMode   = kernel32t.NewProc("SetConsoleMode")
	procGetConsoleScreen = kernel32t.NewProc("GetConsoleScreenBufferInfo")
)

const (
	stdInputHandle  = ^uintptr(9) // (DWORD)-10
	stdOutputHandle = ^uintptr(10)

	enableProcessedInput   = 0x0001
	enableLineInput        = 0x0002
	enableEchoInput        = 0x0004
	enableVirtualTermInput = 0x0200
	enableVirtualTermProc  = 0x0004
)

type coord struct{ X, Y int16 }

type smallRect struct{ Left, Top, Right, Bottom int16 }

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func consoleWidth() int {
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 40 {
			return n
		}
	}
	out, _, _ := procGetStdHandle.Call(stdOutputHandle)
	var info consoleScreenBufferInfo
	if ret, _, _ := procGetConsoleScreen.Call(out, uintptr(unsafe.Pointer(&info))); ret == 0 {
		return 100
	}
	w := int(info.Window.Right-info.Window.Left) + 1
	if w <= 0 {
		return 100
	}
	return w
}

func enableANSI() {
	out, _, _ := procGetStdHandle.Call(stdOutputHandle)
	var mode uint32
	if ret, _, _ := procGetConsoleMode.Call(out, uintptr(unsafe.Pointer(&mode))); ret == 0 {
		return
	}
	procSetConsoleMode.Call(out, uintptr(mode|enableVirtualTermProc))
}

// RunInput switches the console to raw input and streams single key bytes
// until ctx is cancelled, then restores the previous mode.
func RunInput(ctx context.Context, keys chan<- byte) {
	in, _, _ := procGetStdHandle.Call(stdInputHandle)
	var mode uint32
	haveMode := false
	if ret, _, _ := procGetConsoleMode.Call(in, uintptr(unsafe.Pointer(&mode))); ret != 0 {
		haveMode = true
		raw := mode &^ (enableProcessedInput | enableLineInput | enableEchoInput)
		procSetConsoleMode.Call(in, uintptr(raw|enableVirtualTermInput))
		defer procSetConsoleMode.Call(in, uintptr(mode))
	}
	_ = haveMode
	buf := make([]byte, 1)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		select {
		case keys <- buf[0]:
		case <-ctx.Done():
			return
		}
	}
}
