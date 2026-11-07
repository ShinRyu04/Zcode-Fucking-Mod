//go:build windows

package netmon

import (
	"os"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32     = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW      = kernel32.NewProc("Process32FirstW")
	procProcess32NextW       = kernel32.NewProc("Process32NextW")
	procCloseHandle          = kernel32.NewProc("CloseHandle")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procGetProcessIoCounters = kernel32.NewProc("GetProcessIoCounters")
	procGetProcessTimes      = kernel32.NewProc("GetProcessTimes")
)

const (
	th32csSnapProcess       = 0x00000002
	processQueryLimitedInfo = 0x1000
	invalidHandleValue      = ^uintptr(0)
)

type processEntry32 struct {
	Size          uint32
	Usage         uint32
	ProcessID     uint32
	_             uint32
	DefaultHeapID uintptr
	ModuleID      uint32
	Threads       uint32
	ParentPID     uint32
	PriClassBase  int32
	Flags         uint32
	ExeFile       [260]uint16
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherOperationCount uint64
	OtherTransferCount  uint64
}

// readProcess sums IO and CPU time for every process whose executable name
// contains "zcode" (the desktop app and its helpers share that name). No
// match means Found=false and the dashboard hides the process row.
func readProcess(now time.Time) procRaw {
	raw := procRaw{at: now}
	handle, _, _ := procCreateToolhelp32.Call(th32csSnapProcess, 0)
	if handle == 0 || handle == invalidHandleValue {
		return raw
	}
	defer procCloseHandle.Call(handle)

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ok, _, _ := procProcess32FirstW.Call(handle, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := fixedUTF16(entry.ExeFile[:])
		lower := strings.ToLower(name)
		if entry.ProcessID != uint32(os.Getpid()) && strings.Contains(lower, "zcode") && !strings.Contains(lower, "bridge") {
			if read, write, cpu, ok := processCounters(entry.ProcessID); ok {
				raw.found = true
				raw.name = name
				raw.pids = append(raw.pids, entry.ProcessID)
				raw.read += read
				raw.write += write
				raw.cpu100n += cpu
			}
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		ok, _, _ = procProcess32NextW.Call(handle, uintptr(unsafe.Pointer(&entry)))
	}
	return raw
}

func processCounters(pid uint32) (read, write, cpu100n uint64, ok bool) {
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInfo, 0, uintptr(pid))
	if handle == 0 {
		return 0, 0, 0, false
	}
	defer procCloseHandle.Call(handle)

	var counters ioCounters
	if ret, _, _ := procGetProcessIoCounters.Call(handle, uintptr(unsafe.Pointer(&counters))); ret == 0 {
		return 0, 0, 0, false
	}
	var creation, exit, kernel, user [2]uint32
	if ret, _, _ := procGetProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); ret == 0 {
		return 0, 0, 0, false
	}
	cpu := uint64(kernel[0]) | uint64(kernel[1])<<32
	cpu += uint64(user[0]) | uint64(user[1])<<32
	return counters.ReadTransferCount, counters.WriteTransferCount, cpu, true
}

func fixedUTF16(units []uint16) string {
	for i, unit := range units {
		if unit == 0 {
			units = units[:i]
			break
		}
	}
	return string(utf16.Decode(units))
}
