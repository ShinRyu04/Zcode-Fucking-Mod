//go:build windows

package main

// A small layered pet window that floats above the ZCode desktop app. It is
// drawn per pixel with UpdateLayeredWindow so transparency is real, not a
// chroma key, and it never steals focus. State follows /v1/usage: offline,
// sleep, idle, thinking, happy or turbo.

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	petSize         = 190
	wsPopup         = 0x80000000
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExTopmost     = 0x00000008
	wsExNoActivate  = 0x08000000
	gwlExStyle      = ^uintptr(19)
	ulwAlpha        = 0x00000002
	acSrcOver       = 0
	acSrcAlpha      = 1
	wmNCHitTest     = 0x0084
	wmExitSizeMove  = 0x0232
	swHide          = 0
	swShow          = 5
	htTransparent   = ^uintptr(0)
	htCaption       = 2
	spiGetWorkArea  = 0x0030
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	hwndTopmost     = ^uintptr(0)
)

type petSize2 struct{ Cx, Cy int32 }

type petRect struct{ Left, Top, Right, Bottom int32 }

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

var (
	procUpdateLayeredWindow           = user32.NewProc("UpdateLayeredWindow")
	procCreateCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection              = gdi32.NewProc("CreateDIBSection")
	procSelectObject                  = gdi32.NewProc("SelectObject")
	procDeleteDC                      = gdi32.NewProc("DeleteDC")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW             = user32.NewProc("GetWindowLongPtrW")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
)

var pet = struct {
	sync.Mutex
	hwnd         uintptr
	hdc          uintptr
	hbm          uintptr
	bits         []uint32
	stop         chan struct{}
	stopOnce     sync.Once
	visible      bool
	clickThrough bool
	state        string
	tps          float64
	lastSeen     time.Time
	fails        int
}{state: "offline", clickThrough: true}

func petPositionPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "zcode-companion", "pet.json")
}

type petPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func petDefaultPos() (int, int) {
	var r petRect
	procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if r.Right == 0 {
		w, _, _ := procGetSystemMetrics.Call(0)
		h, _, _ := procGetSystemMetrics.Call(1)
		r.Right, r.Bottom = int32(w), int32(h)
	}
	x := int(r.Right) - petSize - 28
	y := int(r.Bottom) - petSize - 28
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

func petLoadPos() (int, int) {
	x, y := petDefaultPos()
	path := petPositionPath()
	if path == "" {
		return x, y
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return x, y
	}
	var p petPos
	if json.Unmarshal(raw, &p) != nil {
		return x, y
	}
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	if p.X < -petSize || p.X > int(sw) || p.Y < -petSize || p.Y > int(sh) {
		return x, y
	}
	return p.X, p.Y
}

func petSavePos() {
	hwnd := pet.hwnd
	if hwnd == 0 {
		return
	}
	var r petRect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return
	}
	path := petPositionPath()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, _ := json.Marshal(petPos{X: int(r.Left), Y: int(r.Top)})
	_ = os.WriteFile(path, raw, 0o644)
}

func petWndProc(hwnd, msgID, wParam, lParam uintptr) uintptr {
	switch msgID {
	case wmNCHitTest:
		pet.Lock()
		through := pet.clickThrough
		pet.Unlock()
		if through {
			return htTransparent
		}
		return htCaption
	case wmExitSizeMove:
		petSavePos()
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msgID, wParam, lParam)
	return ret
}

func enableDPIAwareness() {
	// Per-monitor aware v2 (context -4) keeps Win32 coordinates in physical
	// pixels on scaled displays; otherwise a 125% monitor puts the pet off the
	// right edge because the process works in virtualized coordinates.
	if ret, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3)); ret != 0 {
		return
	}
	procSetProcessDPIAware.Call()
}

func startPet() error {
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cls := utf16Ptr("ZCodePet")
	wc := wndClassEx{
		CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		LpfnWndProc:   syscall.NewCallback(petWndProc),
		HInstance:     hInstance,
		HIcon:         companionIcon,
		LpszClassName: cls,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	exStyle := uintptr(wsExLayered | wsExTransparent | wsExToolWindow | wsExTopmost | wsExNoActivate)
	x, y := petLoadPos()
	title := utf16Ptr("zcode-pet")
	hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		wsPopup, uintptr(x), uintptr(y), petSize, petSize, 0, 0, hInstance, 0)
	if hwnd == 0 {
		return err
	}

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	bmi := bitmapInfo{Header: bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    petSize,
		Height:   -petSize,
		Planes:   1,
		BitCount: 32,
	}}
	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bmi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		procDeleteDC.Call(hdc)
		procDestroyWindow.Call(hwnd)
		return err
	}
	procSelectObject.Call(hdc, hbm)

	pet.Lock()
	pet.hwnd = hwnd
	pet.hdc = hdc
	pet.hbm = hbm
	pet.bits = unsafe.Slice((*uint32)(bits), petSize*petSize)
	pet.visible = true
	pet.clickThrough = true
	pet.stop = make(chan struct{})
	pet.stopOnce = sync.Once{}
	pet.Unlock()

	procShowWindow.Call(hwnd, swShow)
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	go petLoop()
	return nil
}

func stopPet() {
	pet.stopOnce.Do(func() {
		pet.Lock()
		if pet.stop != nil {
			close(pet.stop)
		}
		hwnd, hdc, hbm := pet.hwnd, pet.hdc, pet.hbm
		pet.hwnd, pet.hdc, pet.hbm, pet.bits = 0, 0, 0, nil
		pet.visible = false
		pet.Unlock()
		petSavePosFrom(hwnd, hdc)
		if hbm != 0 {
			procDeleteObject.Call(hbm)
		}
		if hdc != 0 {
			procDeleteDC.Call(hdc)
		}
		if hwnd != 0 {
			procDestroyWindow.Call(hwnd)
		}
	})
}

func petSavePosFrom(hwnd, hdc uintptr) {
	if hwnd == 0 {
		return
	}
	var r petRect
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return
	}
	path := petPositionPath()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, _ := json.Marshal(petPos{X: int(r.Left), Y: int(r.Top)})
	_ = os.WriteFile(path, raw, 0o644)
}

func petToggleVisible() bool {
	pet.Lock()
	hwnd, visible := pet.hwnd, pet.visible
	pet.visible = !visible
	pet.Unlock()
	if hwnd == 0 {
		return false
	}
	if visible {
		procShowWindow.Call(hwnd, swHide)
		return false
	}
	procShowWindow.Call(hwnd, swShow)
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	return true
}

func petToggleClickThrough() bool {
	pet.Lock()
	hwnd := pet.hwnd
	pet.clickThrough = !pet.clickThrough
	through := pet.clickThrough
	pet.Unlock()
	if hwnd == 0 {
		return through
	}
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle))
	if through {
		ex |= wsExTransparent
	} else {
		ex &^= wsExTransparent
	}
	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlExStyle), ex)
	return through
}

func petShown() bool {
	pet.Lock()
	defer pet.Unlock()
	return pet.visible
}

func petThrough() bool {
	pet.Lock()
	defer pet.Unlock()
	return pet.clickThrough
}

type petUsage struct {
	Rings struct {
		LastTPS float64 `json:"last_tps"`
	} `json:"rings"`
	Recent []struct {
		At  time.Time `json:"at"`
		TPS float64   `json:"tps"`
	} `json:"recent"`
}

func petLoop() {
	start := time.Now()
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	fetch := time.NewTicker(2 * time.Second)
	defer fetch.Stop()

	petPollUsage(client)
	for {
		select {
		case <-pet.stop:
			return
		case <-fetch.C:
			petPollUsage(client)
		case <-ticker.C:
			pet.Lock()
			visible := pet.visible
			pet.Unlock()
			if !visible {
				continue
			}
			petFrame(start, time.Now())
		}
	}
}

func petPollUsage(client *http.Client) {
	req, err := http.NewRequest(http.MethodGet, bridgeBaseURL()+"/v1/usage", nil)
	if err == nil {
		if tok := strings.TrimSpace(os.Getenv("ZCODE_BRIDGE_TOKEN")); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		var res *http.Response
		res, err = client.Do(req)
		if err == nil {
			defer res.Body.Close()
			if res.StatusCode != 200 {
				err = errPetStatus(res.StatusCode)
			} else {
				var u petUsage
				err = json.NewDecoder(res.Body).Decode(&u)
				if err == nil {
					petApplyUsage(u)
				}
			}
		}
	}
	pet.Lock()
	if err != nil {
		pet.fails++
		if pet.fails >= 2 {
			pet.state = "offline"
		}
	} else {
		pet.fails = 0
		pet.state = petStateFor(pet.tps, pet.lastSeen)
	}
	pet.Unlock()
}

type petStatusError int

func (e petStatusError) Error() string { return "bridge status " + itoaPet(int(e)) }

func errPetStatus(code int) error { return petStatusError(code) }

func itoaPet(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func petApplyUsage(u petUsage) {
	tps := u.Rings.LastTPS
	seen := time.Time{}
	for _, row := range u.Recent {
		if row.At.After(seen) {
			seen = row.At
			if row.TPS > 0 {
				tps = row.TPS
			}
		}
	}
	pet.Lock()
	pet.tps = tps
	if !seen.IsZero() {
		pet.lastSeen = seen
	}
	pet.Unlock()
}

func petStateFor(tps float64, seen time.Time) string {
	if seen.IsZero() {
		return "idle"
	}
	age := time.Since(seen)
	switch {
	case age < 20*time.Second && tps >= 80:
		return "turbo"
	case age < 20*time.Second && tps > 0:
		return "happy"
	case age < 20*time.Second:
		return "thinking"
	case age < 150*time.Second:
		return "idle"
	default:
		return "sleep"
	}
}

// ---- rendering ----

type petFrameSpec struct {
	rimR, rimG, rimB float64
	alpha            float64
	bob              float64
	speed            float64
	eyes             string
	look             [2]float64
	badge            bool
}

func petFrame(start, now time.Time) {
	pet.Lock()
	defer pet.Unlock()
	bits := pet.bits
	state := pet.state
	tps := pet.tps
	hwnd, hdc := pet.hwnd, pet.hdc
	if bits == nil || hwnd == 0 || hdc == 0 {
		return
	}
	for i := range bits {
		bits[i] = 0
	}
	spec := petSpec(state)
	t := now.Sub(start).Seconds()
	bob := math.Sin(t*spec.speed) * spec.bob
	if state == "turbo" || state == "happy" {
		bob = -math.Abs(math.Sin(t*spec.speed)) * spec.bob
	}
	cx := float64(petSize) / 2
	cy := float64(petSize)/2 - 4 + bob

	fillRoundRect(bits, cx, cy, 62, 58, 26, spec.rimR, spec.rimG, spec.rimB, 0.95*spec.alpha)
	fillRoundRect(bits, cx, cy, 58, 54, 23, 11, 18, 32, 0.97*spec.alpha)

	// antennae
	line(bits, cx-34, cy-52, cx-40, cy-70, 2.4, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	fillCircle(bits, cx-40, cy-72, 4.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	line(bits, cx+34, cy-52, cx+40, cy-70, 2.4, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	fillCircle(bits, cx+40, cy-72, 4.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)

	blink := int(now.UnixMilli())%4300 < 130
	eyeY := cy - 8 + spec.look[1]
	switch spec.eyes {
	case "closed":
		line(bits, cx-20, eyeY, cx-8, eyeY, 2.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
		line(bits, cx+8, eyeY, cx+20, eyeY, 2.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	case "happy":
		arc(bits, cx-14, eyeY+3, 7, 0.35, 2.75, 2.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
		arc(bits, cx+14, eyeY+3, 7, 0.35, 2.75, 2.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	default:
		if blink {
			line(bits, cx-20, eyeY, cx-8, eyeY, 2.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
			line(bits, cx+8, eyeY, cx+20, eyeY, 2.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
		} else {
			fillCircle(bits, cx-14+spec.look[0], eyeY, 4.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
			fillCircle(bits, cx+14+spec.look[0], eyeY, 4.6, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
		}
	}

	switch {
	case state == "happy" || state == "turbo":
		arc(bits, cx, cy+12, 12, 0.5, 2.64, 2.0, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	case state == "thinking":
		fillCircle(bits, cx, cy+16, 2.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	default:
		line(bits, cx-7, cy+16, cx+7, cy+16, 2.2, spec.rimR, spec.rimG, spec.rimB, spec.alpha)
	}

	// throughput bars inside the body footer
	base := cy + 40
	for i := 0; i < 3; i++ {
		level := tps / 140 * float64(i+1)
		if level > 1 {
			level = 1
		}
		if level < 0.12 {
			level = 0.12
		}
		h := 4 + level*9
		bx := cx - 18 + float64(i)*14
		fillRoundRect(bits, bx, base-h/2, 4.5, h/2, 2, spec.rimR, spec.rimG, spec.rimB, 0.85*spec.alpha)
	}

	if spec.badge {
		fillCircle(bits, cx+46, cy-46, 6, 248, 113, 113, 0.95)
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return
	}
	var rect petRect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	dst := point{X: rect.Left, Y: rect.Top}
	size := petSize2{Cx: petSize, Cy: petSize}
	var src point
	blend := blendFunction{BlendOp: acSrcOver, SourceConstantAlpha: 255, AlphaFormat: acSrcAlpha}
	procUpdateLayeredWindow.Call(hwnd, screenDC, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		hdc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
	procReleaseDC.Call(0, screenDC)
}

func petSpec(state string) petFrameSpec {
	switch state {
	case "turbo":
		return petFrameSpec{rimR: 52, rimG: 211, rimB: 153, alpha: 1, bob: 7, speed: 6.2, eyes: "happy"}
	case "happy":
		return petFrameSpec{rimR: 52, rimG: 211, rimB: 153, alpha: 1, bob: 5, speed: 4.4, eyes: "happy"}
	case "thinking":
		return petFrameSpec{rimR: 251, rimG: 191, rimB: 36, alpha: 1, bob: 2.5, speed: 2.8, eyes: "open", look: [2]float64{2.5, -3}}
	case "idle":
		return petFrameSpec{rimR: 52, rimG: 211, rimB: 153, alpha: 0.92, bob: 2, speed: 1.7, eyes: "open"}
	case "sleep":
		return petFrameSpec{rimR: 148, rimG: 163, rimB: 182, alpha: 0.72, bob: 1, speed: 1.1, eyes: "closed"}
	default:
		return petFrameSpec{rimR: 120, rimG: 134, rimB: 156, alpha: 0.6, bob: 0.6, speed: 1, eyes: "closed", badge: true}
	}
}

func blendPixel(bits []uint32, x, y int, r, g, b, a float64) {
	if x < 0 || y < 0 || x >= petSize || y >= petSize || a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	idx := y*petSize + x
	d := bits[idx]
	da := float64(d>>24) / 255
	dia := da * (1 - a)
	dr := float64((d>>16)&0xff) * (1 - a)
	dg := float64((d>>8)&0xff) * (1 - a)
	db := float64(d&0xff) * (1 - a)
	sr := r * a / 255 * 255
	sg := g * a / 255 * 255
	sb := b * a / 255 * 255
	na := a + dia
	if na <= 0 {
		return
	}
	nr := sr + dr
	ng := sg + dg
	nb := sb + db
	clamp := func(v float64) uint32 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint32(v)
	}
	bits[idx] = clamp(na*255)<<24 | clamp(nr)<<16 | clamp(ng)<<8 | clamp(nb)
}

func fillCircle(bits []uint32, cx, cy, rad, r, g, b, a float64) {
	minX, maxX := int(cx-rad)-1, int(cx+rad)+1
	minY, maxY := int(cy-rad)-1, int(cy+rad)+1
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy) - rad
			cover := 0.5 - d
			if cover <= 0 {
				continue
			}
			if cover > 1 {
				cover = 1
			}
			blendPixel(bits, x, y, r, g, b, a*cover)
		}
	}
}

func fillRoundRect(bits []uint32, cx, cy, hw, hh, rad, r, g, b, a float64) {
	minX, maxX := int(cx-hw)-1, int(cx+hw)+1
	minY, maxY := int(cy-hh)-1, int(cy+hh)+1
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5-cx, float64(y)+0.5-cy
			qx, qy := math.Abs(px)-(hw-rad), math.Abs(py)-(hh-rad)
			ox, oy := math.Max(qx, 0), math.Max(qy, 0)
			d := math.Hypot(ox, oy) + math.Min(math.Max(qx, qy), 0) - rad
			cover := 0.5 - d
			if cover <= 0 {
				continue
			}
			if cover > 1 {
				cover = 1
			}
			blendPixel(bits, x, y, r, g, b, a*cover)
		}
	}
}

func line(bits []uint32, x0, y0, x1, y1, rad, r, g, b, a float64) {
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	steps := int(length*2) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		fillCircle(bits, x0+dx*t, y0+dy*t, rad, r, g, b, a)
	}
}

func arc(bits []uint32, cx, cy, rad, from, to, width, r, g, b, a float64) {
	steps := int((to - from) * rad * 3)
	if steps < 8 {
		steps = 8
	}
	for i := 0; i <= steps; i++ {
		t := from + (to-from)*float64(i)/float64(steps)
		fillCircle(bits, cx+math.Cos(t)*rad, cy+math.Sin(t)*rad, width, r, g, b, a)
	}
}
