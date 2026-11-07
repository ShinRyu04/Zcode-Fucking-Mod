//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procCreateMutexW       = kernel32.NewProc("CreateMutexW")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	procAppendMenuW        = user32.NewProc("AppendMenuW")
	procTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	procDestroyMenu        = user32.NewProc("DestroyMenu")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procSetForegroundWnd   = user32.NewProc("SetForegroundWindow")
	procPostMessageW       = user32.NewProc("PostMessageW")
	procMessageBoxW        = user32.NewProc("MessageBoxW")
	procLoadIconW          = user32.NewProc("LoadIconW")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateDIBitmap     = gdi32.NewProc("CreateDIBitmap")
	procCreateBitmap       = gdi32.NewProc("CreateBitmap")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procShellExecuteW      = shell32.NewProc("ShellExecuteW")
	procShellNotifyIconW   = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmDestroy       = 0x0002
	wmCommand       = 0x0111
	wmNull          = 0x0000
	wmApp           = 0x8000
	wmTray          = wmApp + 1
	wmRButtonUp     = 0x0205
	wmLButtonDblClk = 0x0203
	nimAdd          = 0
	nimModify       = 1
	nimDelete       = 2
	nifMessage      = 0x0001
	nifIcon         = 0x0002
	nifTip          = 0x0004
	mfString        = 0x0000
	mfSeparator     = 0x0800
	mfChecked       = 0x0008
	tpmRightButton  = 0x0002
	tpmBottomAlign  = 0x0020
	swShowNormal    = 1
	mbOK            = 0x0000
	mbIconInfo      = 0x0040
	mbIconWarning   = 0x0030
	cbInit          = 4
	dibRGBColors    = 0
)

type notifyIconData struct {
	CbSize            uint32
	_                 uint32
	HWnd              uintptr
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	_                 uint32
	HIcon             uintptr
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          [16]byte
	HBalloonIcon      uintptr
}

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type point struct{ X, Y int32 }

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [3]uint32
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

const (
	runKeyPath      = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue        = "ZCodeCompanion"
	hkeyCurrentUser = 0x80000001
	keySetValue     = 0x0002
	keyQueryValue   = 0x0001
	regSZ           = 1
)

var (
	menuIDs = struct{ dashboard, overlay, autostart, update, restart, quit, petShow, petClick uintptr }{
		1, 2, 3, 4, 5, 6, 7, 8,
	}
	companionHWnd uintptr
	companionIcon uintptr
	sup           *supervisor
	quitRequested bool
)

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func bridgePort() string {
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		return p
	}
	return "20131"
}

func bridgeBaseURL() string {
	host := strings.TrimSpace(os.Getenv("HOST"))
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + host + ":" + bridgePort()
}

// supervisor keeps one bridge process alive unless a healthy bridge (started
// elsewhere) already answers /health.
type supervisor struct {
	mu       sync.Mutex
	dir      string
	exe      string
	child    *exec.Cmd
	stopping bool
	lastErr  string
	healthy  bool
	managed  bool
}

func newSupervisor() *supervisor {
	self, err := os.Executable()
	dir := "."
	if err == nil {
		dir = filepath.Dir(self)
	}
	return &supervisor{dir: dir, exe: filepath.Join(dir, "zcode-bridge.exe")}
}

func (s *supervisor) checkHealth() bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	res, err := client.Get(bridgeBaseURL() + "/health")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	return res.StatusCode == http.StatusOK
}

func (s *supervisor) poll() {
	healthy := s.checkHealth()
	s.mu.Lock()
	s.healthy = healthy
	needStart := !healthy && s.child == nil && !s.stopping
	s.mu.Unlock()
	if needStart {
		s.start()
	}
}

func (s *supervisor) start() {
	if _, err := os.Stat(s.exe); err != nil {
		s.setErr("zcode-bridge.exe tidak ditemukan di " + s.dir)
		return
	}
	cmd := exec.Command(s.exe)
	cmd.Dir = s.dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		s.setErr("gagal menjalankan bridge: " + err.Error())
		return
	}
	s.mu.Lock()
	s.child = cmd
	s.managed = true
	s.lastErr = ""
	s.mu.Unlock()
	go func(c *exec.Cmd) {
		_ = c.Wait()
		s.mu.Lock()
		if s.child == c {
			s.child = nil
		}
		failErr := ""
		if c.ProcessState != nil && !c.ProcessState.Success() && !s.stopping {
			failErr = "bridge berhenti: " + c.ProcessState.String()
		}
		s.mu.Unlock()
		if failErr != "" {
			s.setErr(failErr)
		}
	}(cmd)
}

func (s *supervisor) setErr(e string) {
	s.mu.Lock()
	s.lastErr = e
	s.mu.Unlock()
}

func (s *supervisor) restart() {
	s.mu.Lock()
	child := s.child
	healthy := s.healthy
	s.mu.Unlock()
	if child != nil && child.Process != nil {
		_ = child.Process.Kill()
		time.Sleep(400 * time.Millisecond)
		s.poll()
		return
	}
	if healthy {
		messageBox("Bridge berjalan dari proses lain (mis. start-bridge.cmd); hentikan proses itu untuk memakai supervisor.", mbIconWarning)
		return
	}
	s.poll()
}

func (s *supervisor) stop() {
	s.mu.Lock()
	s.stopping = true
	child := s.child
	s.mu.Unlock()
	if child != nil && child.Process != nil {
		_ = child.Process.Kill()
	}
}

func (s *supervisor) tooltip() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := "offline"
	if s.healthy {
		state = "online"
	}
	mode := "luar"
	if s.managed {
		mode = "dikelola"
	}
	tip := "zcode-bridge " + state + " (" + mode + ")"
	if s.lastErr != "" {
		tip = "zcode-bridge " + state + " · " + s.lastErr
	}
	if len(tip) > 120 {
		tip = tip[:120]
	}
	return tip
}

func runDiagnose() error {
	s := newSupervisor()
	fmt.Println("companion", companionVersion)
	fmt.Println("dir", s.dir)
	if _, err := os.Stat(s.exe); err == nil {
		fmt.Println("bridge-exe", s.exe, "ada")
	} else {
		fmt.Println("bridge-exe", s.exe, "tidak ditemukan")
	}
	fmt.Println("url", bridgeBaseURL())
	fmt.Println("health", s.checkHealth())
	if state, err := autostartState(); err == nil {
		fmt.Println("autostart", state)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(bridgeBaseURL() + "/health")
	if err == nil {
		defer res.Body.Close()
		var h struct {
			Lane     string `json:"lane"`
			Version  string `json:"version"`
			Accounts []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"accounts"`
		}
		if json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&h) == nil {
			fmt.Println("lane", h.Lane, "versi", h.Version, "akun", len(h.Accounts))
		}
	}
	return nil
}

func messageBox(text string, flags uintptr) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString("zcode-companion")
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
}

func openURL(u string) {
	target, _ := syscall.UTF16PtrFromString(u)
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16Ptr("open"))), uintptr(unsafe.Pointer(target)), 0, 0, swShowNormal)
}

func runCompanion(noTray bool) error {
	name, _ := syscall.UTF16PtrFromString("ZCodeCompanionSingleton")
	mutex, _, lastErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if mutex == 0 {
		return fmt.Errorf("CreateMutex gagal: %v", lastErr)
	}
	if lastErr == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		return fmt.Errorf("companion sudah berjalan")
	}

	sup = newSupervisor()
	enableDPIAwareness()
	go func() {
		sup.poll()
		for !quitRequested {
			time.Sleep(3 * time.Second)
			sup.poll()
		}
	}()

	if noTray {
		for !quitRequested {
			time.Sleep(time.Second)
		}
		return nil
	}
	return runTray()
}

func runTray() error {
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cls := utf16Ptr("ZCodeCompanionTray")
	wndProc := syscall.NewCallback(trayWndProc)
	companionIcon = makeIcon()
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   wndProc,
		HInstance:     hInstance,
		HIcon:         companionIcon,
		LpszClassName: cls,
	}
	if ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		return fmt.Errorf("RegisterClassEx: %v", err)
	}
	title := utf16Ptr("zcode-companion")
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		0, 0, 0, 0, 0, 0, 0, hInstance, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %v", err)
	}
	companionHWnd = hwnd

	nid := notifyIconData{
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTray,
		HIcon:            companionIcon,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.SzTip[:], syscall.StringToUTF16("zcode-bridge"))
	if ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); ret == 0 {
		return fmt.Errorf("Shell_NotifyIcon: %v", err)
	}
	defer procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	_ = startPet()
	defer stopPet()

	go func() {
		for !quitRequested {
			time.Sleep(2 * time.Second)
			modifyTip(nid, sup.tooltip())
		}
	}()

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}

func modifyTip(nid notifyIconData, tip string) {
	nid.SzTip = [128]uint16{}
	copy(nid.SzTip[:], syscall.StringToUTF16(tip))
	nid.UFlags = nifTip
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func trayWndProc(hwnd, msgID, wParam, lParam uintptr) uintptr {
	switch msgID {
	case wmTray:
		event := uint32(lParam) & 0xffff
		if event == wmRButtonUp || event == wmLButtonDblClk {
			if event == wmLButtonDblClk {
				openURL(bridgeBaseURL() + "/dashboard")
				return 0
			}
			showTrayMenu(hwnd)
		}
		return 0
	case wmCommand:
		handleCommand(uint32(wParam) & 0xffff)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
	return ret
}

func showTrayMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)
	appendItem(hMenu, menuIDs.dashboard, "Buka dashboard")
	appendItem(hMenu, menuIDs.overlay, "Overlay kapsul")
	petShowFlags := uintptr(mfString)
	if petShown() {
		petShowFlags |= mfChecked
	}
	procAppendMenuW.Call(hMenu, petShowFlags, menuIDs.petShow, uintptr(unsafe.Pointer(utf16Ptr("Pet: tampilkan"))))
	petClickFlags := uintptr(mfString)
	if petThrough() {
		petClickFlags |= mfChecked
	}
	procAppendMenuW.Call(hMenu, petClickFlags, menuIDs.petClick, uintptr(unsafe.Pointer(utf16Ptr("Pet: klik-tembus"))))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	autoFlags := uintptr(mfString)
	if on, err := autostartState(); err == nil && on {
		autoFlags |= mfChecked
	}
	procAppendMenuW.Call(hMenu, autoFlags, menuIDs.autostart, uintptr(unsafe.Pointer(utf16Ptr("Autostart saat login"))))
	appendItem(hMenu, menuIDs.update, "Periksa pembaruan")
	appendItem(hMenu, menuIDs.restart, "Mulai ulang bridge")
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	appendItem(hMenu, menuIDs.quit, "Keluar")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(hwnd)
	procTrackPopupMenu.Call(hMenu, tpmRightButton|tpmBottomAlign, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, wmNull, 0, 0)
}

func appendItem(hMenu, id uintptr, text string) {
	procAppendMenuW.Call(hMenu, mfString, id, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func handleCommand(id uint32) {
	switch id {
	case 1:
		openURL(bridgeBaseURL() + "/dashboard")
	case 2:
		openURL(bridgeBaseURL() + "/overlay/capsule")
	case 3:
		on, err := autostartState()
		if err != nil {
			messageBox("Autostart: "+err.Error(), mbIconWarning)
			return
		}
		if err := setAutostart(!on); err != nil {
			messageBox("Autostart: "+err.Error(), mbIconWarning)
			return
		}
		if !on {
			messageBox("Autostart diaktifkan untuk pengguna ini.", mbIconInfo)
		} else {
			messageBox("Autostart dimatikan.", mbIconInfo)
		}
	case 4:
		info, err := checkLatestRelease(companionVersion)
		if err != nil {
			messageBox("Periksa pembaruan: "+err.Error(), mbIconWarning)
			return
		}
		messageBox(strings.TrimSpace(info.String()), mbIconInfo)
	case 5:
		sup.restart()
	case 6:
		quitRequested = true
		stopPet()
		sup.stop()
		procDestroyWindow.Call(companionHWnd)
	case 7:
		petToggleVisible()
	case 8:
		petToggleClickThrough()
	}
}

func makeIcon() uintptr {
	const size = 32
	bits := make([]uint32, size*size)
	mask := make([]byte, size*size/8)
	cx, cy := 15.5, 15.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			dist := dx*dx + dy*dy
			idx := y*size + x
			if dist > 15.5*15.5 {
				mask[idx/8] |= 0x80 >> uint(idx%8)
				continue
			}
			ratio := dist / (15.5 * 15.5)
			g := uint32(210 - 90*ratio)
			r := uint32(24)
			b := uint32(90 - 40*ratio)
			a := uint32(255)
			bits[idx] = a<<24 | r<<16 | g<<8 | b
		}
	}
	// white "Z" stroke across the disc
	for _, seg := range [][3]int{{9, 9, 22}, {9, 22, 22}} {
		for x := seg[0]; x <= seg[2]; x++ {
			putPixel(bits, size, x, seg[1], 0xFFFFFFFF)
		}
	}
	for i := 0; i < 12; i++ {
		putPixel(bits, size, 22-i, 10+i, 0xFFFFFFFF)
	}

	hdc, _, _ := procGetDC.Call(0)
	bmi := bitmapInfo{Header: bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    size,
		Height:   -size,
		Planes:   1,
		BitCount: 32,
	}}
	hbmColor, _, _ := procCreateDIBitmap.Call(hdc,
		uintptr(unsafe.Pointer(&bmi)), cbInit, uintptr(unsafe.Pointer(&bits[0])),
		uintptr(unsafe.Pointer(&bmi)), dibRGBColors)
	procReleaseDC.Call(0, hdc)
	hbmMask, _, _ := procCreateBitmap.Call(size, size, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	ii := iconInfo{FIcon: 1, HbmMask: hbmMask, HbmColor: hbmColor}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	procDeleteObject.Call(hbmColor)
	procDeleteObject.Call(hbmMask)
	if icon == 0 {
		icon, _, _ = procLoadIconW.Call(0, 32512) // IDI_APPLICATION
	}
	return icon
}

func putPixel(bits []uint32, size, x, y int, argb uint32) {
	if x < 0 || y < 0 || x >= size || y >= size {
		return
	}
	bits[y*size+x] = argb
}

// ---- autostart (HKCU Run) ----

func setAutostart(on bool) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	var key syscall.Handle
	path := utf16Ptr(runKeyPath)
	if ret, _, _ := advapi32.NewProc("RegCreateKeyExW").Call(
		hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, 0, 0, keySetValue, 0,
		uintptr(unsafe.Pointer(&key)), 0); ret != 0 {
		return fmt.Errorf("RegCreateKeyEx gagal (kode %d)", ret)
	}
	defer advapi32.NewProc("RegCloseKey").Call(uintptr(key))
	name := utf16Ptr(runValue)
	if !on {
		ret, _, _ := advapi32.NewProc("RegDeleteValueW").Call(uintptr(key), uintptr(unsafe.Pointer(name)))
		if ret == 2 { // ERROR_FILE_NOT_FOUND
			return nil
		}
		if ret != 0 {
			return fmt.Errorf("RegDeleteValue gagal (kode %d)", ret)
		}
		return nil
	}
	value := `"` + self + `"`
	data := syscall.StringToUTF16(value)
	ret, _, _ := advapi32.NewProc("RegSetValueExW").Call(uintptr(key),
		uintptr(unsafe.Pointer(name)), 0, regSZ,
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
	if ret != 0 {
		return fmt.Errorf("RegSetValueEx gagal (kode %d)", ret)
	}
	return nil
}

func autostartState() (bool, error) {
	var key syscall.Handle
	path := utf16Ptr(runKeyPath)
	if ret, _, _ := advapi32.NewProc("RegOpenKeyExW").Call(
		hkeyCurrentUser, uintptr(unsafe.Pointer(path)), 0, keyQueryValue,
		uintptr(unsafe.Pointer(&key))); ret != 0 {
		return false, nil
	}
	defer advapi32.NewProc("RegCloseKey").Call(uintptr(key))
	name := utf16Ptr(runValue)
	var size uint32
	ret, _, _ := advapi32.NewProc("RegQueryValueExW").Call(uintptr(key),
		uintptr(unsafe.Pointer(name)), 0, 0, 0, uintptr(unsafe.Pointer(&size)))
	if ret == 2 { // ERROR_FILE_NOT_FOUND
		return false, nil
	}
	if ret != 0 {
		return false, fmt.Errorf("RegQueryValueEx gagal (kode %d)", ret)
	}
	return size > 0, nil
}
