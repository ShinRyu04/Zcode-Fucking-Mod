//go:build windows

package main

import (
	"testing"
	"unsafe"
)

// The Win32 ABI expects exactly 976 bytes for NOTIFYICONDATAW on amd64;
// a wrong layout means a tray icon that silently fails to appear.
func TestNotifyIconDataLayout(t *testing.T) {
	if got := unsafe.Sizeof(notifyIconData{}); got != 976 {
		t.Fatalf("notifyIconData size = %d, want 976", got)
	}
	if got := unsafe.Offsetof(notifyIconData{}.HIcon); got != 32 {
		t.Fatalf("HIcon offset = %d, want 32", got)
	}
	if got := unsafe.Offsetof(notifyIconData{}.SzTip); got != 40 {
		t.Fatalf("SzTip offset = %d, want 40", got)
	}
}
