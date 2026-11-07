//go:build windows

package netmon

import (
	"encoding/binary"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	iphlpapi                 = syscall.NewLazyDLL("iphlpapi.dll")
	procGetIfTable           = iphlpapi.NewProc("GetIfTable")
	procGetExtendedTcpTable  = iphlpapi.NewProc("GetExtendedTcpTable")
	procGetAdaptersAddresses = iphlpapi.NewProc("GetAdaptersAddresses")
)

const (
	ifRowSize               = 860
	tcpRowSize              = 24
	tcpTableConnections     = 4 // TCP_TABLE_OWNER_PID_CONNECTIONS
	afInet                  = 2
	errorInsufficientBuffer = 122
	errorBufferOverflow     = 111
	mibTcpStateEstablished  = 5
)

// read pulls interface counters and TCP owner table state. A nil error with
// an empty slice is a valid "nothing to report" read.
func read(now time.Time) rawSample {
	sample := rawSample{at: now}
	ifaces, err := ifTable()
	if err != nil {
		sample.err = err
		return sample
	}
	sample.ifs = ifaces
	total, estab, err := tcpTable(uint16(live.port))
	if err != nil {
		sample.err = err
		return sample
	}
	sample.total, sample.estab = total, estab
	return sample
}

// ifTable decodes MIB_IFTABLE (GetIfTable). MIB_IFROW is 860 bytes on both
// 32- and 64-bit Windows; every field used here sits at a fixed offset.
func ifTable() ([]Interface, error) {
	var size uint32
	ret, _, _ := procGetIfTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if ret != errorInsufficientBuffer && ret != 0 {
		return nil, syscall.Errno(ret)
	}
	if size == 0 {
		return nil, nil
	}
	buf := make([]byte, int(size)+ifRowSize)
	size = uint32(len(buf))
	ret, _, _ = procGetIfTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0)
	if ret != 0 {
		return nil, syscall.Errno(ret)
	}
	count := int(binary.LittleEndian.Uint32(buf[0:4]))
	adapters, adaptersErr := adapterNames()
	out := make([]Interface, 0, count)
	for i := 0; i < count; i++ {
		row := buf[4+i*ifRowSize:]
		if len(row) < ifRowSize {
			break
		}
		index := binary.LittleEndian.Uint32(row[512:516])
		if adaptersErr == nil {
			// GetIfTable lists per-filter-layer rows; GetAdaptersAddresses
			// names only real adapters, so it is the authoritative index set.
			adapter, ok := adapters[index]
			if !ok {
				continue
			}
			out = append(out, Interface{
				Name:        adapter.name,
				Description: adapter.description,
				Index:       index,
				Up:          adapter.up,
				InBytes:     uint64(binary.LittleEndian.Uint32(row[552:556])),
				OutBytes:    uint64(binary.LittleEndian.Uint32(row[576:580])),
			})
			continue
		}
		name := decodeUTF16(row[0:512])
		descLen := int(binary.LittleEndian.Uint32(row[600:604]))
		if descLen > 256 {
			descLen = 256
		}
		out = append(out, Interface{
			Name:        name,
			Description: string(row[604 : 604+descLen]),
			Index:       index,
			Up:          binary.LittleEndian.Uint32(row[544:548]) == 1,
			InBytes:     uint64(binary.LittleEndian.Uint32(row[552:556])),
			OutBytes:    uint64(binary.LittleEndian.Uint32(row[576:580])),
		})
	}
	if adaptersErr != nil {
		// Fall back to descriptor dedupe when GetAdaptersAddresses is absent.
		seen := map[string]bool{}
		unique := out[:0]
		for _, iface := range out {
			key := iface.Description
			if key == "" {
				key = iface.Name
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			unique = append(unique, iface)
		}
		out = unique
	}
	return out, nil
}

type adapterInfo struct {
	name        string
	description string
	up          bool
}

// adapterNames decodes IP_ADAPTER_ADDRESSES into an IfIndex-keyed map of
// friendly names. Pointer offsets scale with the host word size; the trailing
// fixed fields (OperStatus) sit after the FriendlyName pointer.
func adapterNames() (map[uint32]adapterInfo, error) {
	var size uint32
	ret, _, _ := procGetAdaptersAddresses.Call(0, 0, 0, 0, uintptr(unsafe.Pointer(&size)))
	if ret != errorBufferOverflow && ret != 0 {
		return nil, syscall.Errno(ret)
	}
	if size == 0 {
		return map[uint32]adapterInfo{}, nil
	}
	buf := make([]byte, int(size)+64)
	size = uint32(len(buf))
	ret, _, _ = procGetAdaptersAddresses.Call(0, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret != 0 {
		return nil, syscall.Errno(ret)
	}
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	nameOff := 8 + 8*ptrSize
	operOff := nameOff + ptrSize + 8 + 4 + 4 + 4 + 4
	baseAddr := uintptr(unsafe.Pointer(&buf[0]))
	out := map[uint32]adapterInfo{}
	for base := 0; base+operOff+4 <= len(buf); {
		index := binary.LittleEndian.Uint32(buf[base+4 : base+8])
		friendly := utf16Ptr(buf, baseAddr, ptrAt(buf, base+nameOff, ptrSize))
		description := utf16Ptr(buf, baseAddr, ptrAt(buf, base+nameOff-ptrSize, ptrSize))
		oper := binary.LittleEndian.Uint32(buf[base+operOff : base+operOff+4])
		if friendly != "" {
			out[index] = adapterInfo{name: friendly, description: description, up: oper == 1}
		}
		next := ptrAt(buf, base+8, ptrSize)
		if next == 0 {
			break
		}
		offset := int(uintptr(next) - baseAddr)
		if offset <= base || offset >= len(buf) {
			break
		}
		base = offset
	}
	return out, nil
}

func ptrAt(buf []byte, off, ptrSize int) uint64 {
	if off < 0 || off+ptrSize > len(buf) {
		return 0
	}
	if ptrSize == 8 {
		return binary.LittleEndian.Uint64(buf[off : off+8])
	}
	return uint64(binary.LittleEndian.Uint32(buf[off : off+4]))
}

// utf16Ptr reads a NUL-terminated UTF-16 string pointed to by an absolute
// address inside buf; addresses outside the buffer yield "".
func utf16Ptr(buf []byte, baseAddr uintptr, ptr uint64) string {
	if ptr == 0 {
		return ""
	}
	start := int(uintptr(ptr) - baseAddr)
	if start < 0 || start >= len(buf) {
		return ""
	}
	end := start
	for end+1 < len(buf) {
		if buf[end] == 0 && buf[end+1] == 0 {
			break
		}
		end += 2
	}
	units := make([]uint16, 0, (end-start)/2)
	for i := start; i+1 < end; i += 2 {
		units = append(units, binary.LittleEndian.Uint16(buf[i:i+2]))
	}
	return string(utf16.Decode(units))
}

// tcpTable decodes MIB_TCPTABLE_OWNER_PID rows and attributes them to the
// bridge port. port==0 counts every IPv4 connection.
func tcpTable(port uint16) (total, established int, err error) {
	var size uint32
	ret, _, _ := procGetExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0,
		uintptr(afInet), uintptr(tcpTableConnections), 0)
	if ret != errorInsufficientBuffer && ret != 0 {
		return 0, 0, syscall.Errno(ret)
	}
	if size == 0 {
		return 0, 0, nil
	}
	buf := make([]byte, int(size)+tcpRowSize)
	size = uint32(len(buf))
	ret, _, _ = procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0,
		uintptr(afInet), uintptr(tcpTableConnections), 0)
	if ret != 0 {
		return 0, 0, syscall.Errno(ret)
	}
	count := int(binary.LittleEndian.Uint32(buf[0:4]))
	for i := 0; i < count; i++ {
		row := buf[4+i*tcpRowSize:]
		if len(row) < tcpRowSize {
			break
		}
		localPort := uint16(binary.BigEndian.Uint16(row[8:10]))
		if port != 0 && localPort != port {
			continue
		}
		total++
		if binary.LittleEndian.Uint32(row[0:4]) == mibTcpStateEstablished {
			established++
		}
	}
	return total, established, nil
}

func decodeUTF16(raw []byte) string {
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		unit := binary.LittleEndian.Uint16(raw[i : i+2])
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return string(utf16.Decode(units))
}
