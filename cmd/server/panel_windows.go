//go:build windows

package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The panel window: host status, verification code, the oldest pending
// pairing request, the paired phones and the autostart switch. It is one
// custom-painted window (GDI, double buffered, no child controls) that lives
// on the tray thread (created, refreshed and destroyed there), so it shares
// the tray's message loop and needs no locking. Closing it only destroys
// the window; the server keeps running in the tray.

var (
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	dwmapi = syscall.NewLazyDLL("dwmapi.dll")

	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procEllipse                = gdi32.NewProc("Ellipse")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procGetTextFaceW           = gdi32.NewProc("GetTextFaceW")
	procSetTextCharacterExtra  = gdi32.NewProc("SetTextCharacterExtra")
	procDwmSetWindowAttribute  = dwmapi.NewProc("DwmSetWindowAttribute")

	procShowWindow       = user32.NewProc("ShowWindow")
	procUpdateWindow     = user32.NewProc("UpdateWindow")
	procSetTimer         = user32.NewProc("SetTimer")
	procKillTimer        = user32.NewProc("KillTimer")
	procAdjustWindowRect = user32.NewProc("AdjustWindowRect")
	procIsIconic         = user32.NewProc("IsIconic")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procSetCursor        = user32.NewProc("SetCursor")
	procBeginPaint       = user32.NewProc("BeginPaint")
	procEndPaint         = user32.NewProc("EndPaint")
	procFillRect         = user32.NewProc("FillRect")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procInvalidateRect   = user32.NewProc("InvalidateRect")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetClientRect    = user32.NewProc("GetClientRect")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procTrackMouseEvent  = user32.NewProc("TrackMouseEvent")
	procScreenToClient   = user32.NewProc("ScreenToClient")

	panelWndProcPtr         = syscall.NewCallback(panelWndProc)
	activePanel             *winPanel
	panelClassRegistered    bool
	errPanelWindowNotCreate = errors.New("CreateWindowExW failed for the panel")
)

const (
	panelWindowClass = "NudgePanelWindow"
	panelTitle       = "Nudge"
	panelWidth       = 480 // client area, in pixels (the exe is DPI unaware)
	panelPad         = 16  // outer horizontal margin
	panelCardPad     = 14  // padding inside the banner and the card
	panelRowH        = 58  // one paired-phone row
	panelBtnH        = 36
	panelFooterH     = 47 // 1px border + 46
	panelMaxRows     = 4
	panelTimerID     = 1
	panelRefresh     = time.Second
	panelCopiedFor   = 1500 * time.Millisecond

	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000

	// panelWindowStyle is used both to create the window and to size it
	// (AdjustWindowRect) so the client area stays exact.
	panelWindowStyle = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox

	wmPaint       = 0x000F
	wmEraseBkgnd  = 0x0014
	wmSetCursor   = 0x0020
	wmKeyDown     = 0x0100
	wmTimer       = 0x0113
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmMouseLeave  = 0x02A3
	vkEscape      = 0x1B
	htClient      = 1
	tmeLeave      = 0x2

	swShow     = 5
	swRestore  = 9
	idcArrow   = 32512
	idcHand    = 32649
	smCxScreen = 0
	smCyScreen = 1
	smCxIcon   = 11
	smCyIcon   = 12

	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpFrameChg   = 0x0020

	fwNormal      = 400
	fwSemiBold    = 600
	fwBold        = 700
	defaultCharst = 1
	cleartypeQual = 5
	bkTransparent = 1
	psSolid       = 0
	nullBrush     = 5
	srcCopy       = 0x00CC0020

	dtCenter     = 0x0001
	dtRight      = 0x0002
	dtVCenter    = 0x0004
	dtBottom     = 0x0008
	dtWordBreak  = 0x0010
	dtSingleLine = 0x0020
	dtCalcRect   = 0x0400
	dtNoPrefix   = 0x0800
	dtEndEllip   = 0x8000

	dwmwaDarkModeOld = 19
	dwmwaDarkMode    = 20
	dwmwaCaption     = 35
)

// Palette (COLORREF is 0x00BBGGRR).
var (
	colBg          = rgb(0x1A, 0x1A, 0x1E)
	colCard        = rgb(0x25, 0x25, 0x2B)
	colBtn         = rgb(0x2D, 0x2D, 0x34)
	colBtnHover    = rgb(0x38, 0x38, 0x41)
	colLine        = rgb(0x45, 0x45, 0x4D)
	colText        = rgb(0xF5, 0xF5, 0xF7)
	colMuted       = rgb(0xA8, 0xA8, 0xB3)
	colAccent      = rgb(0x00, 0x69, 0x6D)
	colAccentHover = rgb(0x00, 0x7D, 0x82)
	colAccentHi    = rgb(0x4F, 0xD1, 0xD5)
	colOK          = rgb(0x5B, 0xD6, 0x8A)
	colDanger      = rgb(0xFF, 0x8A, 0x80)
	colFooter      = rgb(0x20, 0x20, 0x24)
	colBanner      = rgb(22, 38, 42)  // accent at 15% over the background
	colPillFill    = rgb(31, 41, 39)  // ok at 8% over the background
	colPillBorder  = rgb(52, 101, 73) // ok at 40% over the background
	colKnob        = rgb(0xFF, 0xFF, 0xFF)
	colKnobOff     = rgb(0xD0, 0xD0, 0xD6)
	colOffDisabled = rgb(0x5A, 0x5A, 0x62)
)

func rgb(r, g, b uint8) uintptr { return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16 }

type rect struct{ left, top, right, bottom int32 }

func (r rect) contains(x, y int32) bool {
	return x >= r.left && x < r.right && y >= r.top && y < r.bottom
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type trackMouseEvent struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
}

// Clickable areas.
const (
	hitNone = iota
	hitCopy
	hitDeny
	hitApprove
	hitRevoke
	hitAutostart
)

type hitKey struct {
	kind int
	arg  string // request or device ID
}

type panelHit struct {
	key hitKey
	r   rect
}

type panelFonts struct {
	body, bold, semi, small, link, section, title, code uintptr
}

type winPanel struct {
	tray *winTray
	hwnd uintptr

	fonts   panelFonts
	hits    []panelHit
	clientH int32

	hover, pressed hitKey
	tracking       bool
	busy           bool // a modal confirmation is open
	copiedUntil    time.Time
	copiedShown    bool

	model         panelModel
	haveModel     bool
	autostartOn   bool
	autostartOK   bool
	haveAutostart bool
}

// openPanel shows the panel, creating it if needed. Tray thread only.
func (t *winTray) openPanel() {
	if p := activePanel; p != nil && p.hwnd != 0 {
		if r, _, _ := procIsIconic.Call(p.hwnd); r != 0 {
			procShowWindow.Call(p.hwnd, swRestore)
		}
		procSetForegroundWnd.Call(p.hwnd)
		return
	}
	p := &winPanel{tray: t}
	if err := p.create(); err != nil {
		t.logf("panel window unavailable: %v", err)
		p.destroyResources()
		return
	}
	procShowWindow.Call(p.hwnd, swShow)
	procUpdateWindow.Call(p.hwnd)
	procSetForegroundWnd.Call(p.hwnd)
}

// wstr converts s for a Win32 call. Convert it with uintptr(unsafe.Pointer())
// only inside the call's argument list, so the buffer stays alive.
func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func makeFont(face string, height, weight int, underline bool) uintptr {
	u := uintptr(0)
	if underline {
		u = 1
	}
	f, _, _ := procCreateFontW.Call(uintptr(-height), 0, 0, 0, uintptr(weight), 0, u, 0,
		defaultCharst, 0, 0, cleartypeQual, 0, uintptr(unsafe.Pointer(wstr(face))))
	return f
}

// makeCodeFont prefers Cascadia Mono and falls back to Consolas when the
// font mapper substituted another face.
func makeCodeFont(height, weight int) uintptr {
	f := makeFont("Cascadia Mono", height, weight, false)
	if f == 0 {
		return makeFont("Consolas", height, weight, false)
	}
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return f
	}
	defer procReleaseDC.Call(0, dc)
	old, _, _ := procSelectObject.Call(dc, f)
	var buf [64]uint16
	procGetTextFaceW.Call(dc, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	procSelectObject.Call(dc, old)
	if !strings.EqualFold(syscall.UTF16ToString(buf[:]), "Cascadia Mono") {
		procDeleteObject.Call(f)
		f = makeFont("Consolas", height, weight, false)
	}
	return f
}

func (p *winPanel) create() error {
	for _, proc := range []*syscall.LazyProc{procCreateFontW, procBeginPaint, procDrawTextW, procBitBlt, procRoundRect, procTrackMouseEvent} {
		if err := proc.Find(); err != nil {
			return err
		}
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	if !panelClassRegistered {
		// The class icon and background brush live as long as the class
		// (the whole process); they are never destroyed.
		cx, _, _ := procGetSystemMetrics.Call(smCxIcon)
		cy, _, _ := procGetSystemMetrics.Call(smCyIcon)
		bigIcon, _, _ := procLoadImageW.Call(hInst, 1, imageIcon, cx, cy, lrDefaultColor)
		cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
		bg, _, _ := procCreateSolidBrush.Call(colBg)
		wc := wndClassExW{
			lpfnWndProc:   panelWndProcPtr,
			hInstance:     hInst,
			hIcon:         bigIcon,
			hIconSm:       p.tray.icon,
			hCursor:       cursor,
			hbrBackground: bg,
			lpszClassName: wstr(panelWindowClass),
		}
		wc.cbSize = uint32(unsafe.Sizeof(wc))
		if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
			if bg != 0 {
				procDeleteObject.Call(bg)
			}
			return errors.New("RegisterClassExW: " + callErr.Error())
		}
		panelClassRegistered = true
	}

	p.fonts = panelFonts{
		body:    makeFont("Segoe UI", 14, fwNormal, false),
		bold:    makeFont("Segoe UI", 14, fwBold, false),
		semi:    makeFont("Segoe UI", 14, fwSemiBold, false),
		small:   makeFont("Segoe UI", 13, fwNormal, false),
		link:    makeFont("Segoe UI", 13, fwNormal, true),
		section: makeFont("Segoe UI", 12, fwSemiBold, false),
		title:   makeFont("Segoe UI", 16, fwSemiBold, false),
		code:    makeCodeFont(38, fwSemiBold),
	}

	// Content first, so the window is created at its final height.
	p.refresh()

	style := uintptr(panelWindowStyle)
	w, h := panelWindowSize(style, p.clientH)
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	x, y := (int32(sw)-w)/2, (int32(sh)-h)/2

	// create() owns activePanel: it is set before CreateWindowExW (the window
	// procedure receives messages during creation) and cleared on failure.
	activePanel = p
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(wstr(panelWindowClass))), uintptr(unsafe.Pointer(wstr(panelTitle))), style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, hInst, 0)
	if hwnd == 0 {
		activePanel = nil
		return errPanelWindowNotCreate
	}
	p.hwnd = hwnd
	darkTitleBar(hwnd)
	procSetTimer.Call(hwnd, panelTimerID, uintptr(panelRefresh/time.Millisecond), 0)
	return nil
}

func panelWindowSize(style uintptr, clientH int32) (int32, int32) {
	r := rect{0, 0, panelWidth, clientH}
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(&r)), style, 0)
	return r.right - r.left, r.bottom - r.top
}

// darkTitleBar asks DWM for a dark caption; failures (older Windows) are
// ignored and leave the default title bar.
func darkTitleBar(hwnd uintptr) {
	if procDwmSetWindowAttribute.Find() != nil {
		return
	}
	on := int32(1)
	if r, _, _ := procDwmSetWindowAttribute.Call(hwnd, dwmwaDarkMode, uintptr(unsafe.Pointer(&on)), 4); r != 0 {
		procDwmSetWindowAttribute.Call(hwnd, dwmwaDarkModeOld, uintptr(unsafe.Pointer(&on)), 4)
	}
	caption := uint32(colFooter)
	procDwmSetWindowAttribute.Call(hwnd, dwmwaCaption, uintptr(unsafe.Pointer(&caption)), 4)
}

// refresh rebuilds the model from a fresh broker snapshot and repaints
// (and resizes) only when something visible changed.
func (p *winPanel) refresh() {
	t := p.tray
	devices, requests := t.broker.Snapshot()
	_, code := t.broker.Verification()
	host, _ := os.Hostname()
	m := panelModelFrom(host, code, devices, requests, time.Now())

	changed := !p.haveModel || !reflect.DeepEqual(m, p.model)
	p.model, p.haveModel = m, true
	if !t.autostartBusy.Load() {
		on, ok := autostartState()
		if !p.haveAutostart || on != p.autostartOn || ok != p.autostartOK {
			p.autostartOn, p.autostartOK, p.haveAutostart = on, ok, true
			changed = true
		}
	}
	if copied := time.Now().Before(p.copiedUntil); copied != p.copiedShown {
		p.copiedShown = copied
		changed = true
	}
	if changed {
		p.relayout()
	}
}

// relayout measures the content (hit areas and height) without drawing,
// resizes the window when the height changed and schedules a repaint.
func (p *winPanel) relayout() {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return
	}
	h := p.render(&painter{dc: dc})
	procReleaseDC.Call(0, dc)
	if p.hwnd == 0 {
		p.clientH = h
		return
	}
	if h != p.clientH {
		p.clientH = h
		w, wh := panelWindowSize(uintptr(panelWindowStyle), h)
		procSetWindowPos.Call(p.hwnd, 0, 0, 0, uintptr(w), uintptr(wh), swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChg)
	}
	procInvalidateRect.Call(p.hwnd, 0, 0)
}

// painter draws on dc, or only measures when draw is false. Every GDI
// object it creates is deleted before the call returns.
type painter struct {
	dc   uintptr
	draw bool
}

func (g *painter) fill(r rect, color uintptr) {
	if !g.draw {
		return
	}
	br, _, _ := procCreateSolidBrush.Call(color)
	if br == 0 {
		return
	}
	procFillRect.Call(g.dc, uintptr(unsafe.Pointer(&r)), br)
	procDeleteObject.Call(br)
}

// shape draws a rounded rectangle (radius > 0) or an ellipse (radius < 0)
// with a 1px border; noFill leaves the inside transparent.
func (g *painter) shape(r rect, radius int32, fill, border uintptr, noFill bool) {
	if !g.draw {
		return
	}
	pen, _, _ := procCreatePen.Call(psSolid, 1, border)
	var br uintptr
	if noFill {
		br, _, _ = procGetStockObject.Call(nullBrush)
	} else {
		br, _, _ = procCreateSolidBrush.Call(fill)
	}
	if pen == 0 || br == 0 {
		if pen != 0 {
			procDeleteObject.Call(pen)
		}
		if br != 0 && !noFill {
			procDeleteObject.Call(br)
		}
		return
	}
	oldPen, _, _ := procSelectObject.Call(g.dc, pen)
	oldBr, _, _ := procSelectObject.Call(g.dc, br)
	if radius < 0 {
		procEllipse.Call(g.dc, uintptr(r.left), uintptr(r.top), uintptr(r.right), uintptr(r.bottom))
	} else {
		procRoundRect.Call(g.dc, uintptr(r.left), uintptr(r.top), uintptr(r.right), uintptr(r.bottom), uintptr(2*radius), uintptr(2*radius))
	}
	procSelectObject.Call(g.dc, oldBr)
	procSelectObject.Call(g.dc, oldPen)
	procDeleteObject.Call(pen)
	if !noFill {
		procDeleteObject.Call(br)
	}
}

// text draws s in r. With dtCalcRect in flags it only measures, and the
// measured rect is returned in both modes.
func (g *painter) text(s string, font, color uintptr, r rect, flags uintptr, extra int32) rect {
	old, _, _ := procSelectObject.Call(g.dc, font)
	if extra != 0 {
		procSetTextCharacterExtra.Call(g.dc, uintptr(extra))
	}
	if !g.draw || flags&dtCalcRect != 0 {
		procDrawTextW.Call(g.dc, uintptr(unsafe.Pointer(wstr(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|dtCalcRect|dtNoPrefix)
	} else {
		procSetTextColor.Call(g.dc, color)
		procDrawTextW.Call(g.dc, uintptr(unsafe.Pointer(wstr(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|dtNoPrefix)
	}
	if extra != 0 {
		procSetTextCharacterExtra.Call(g.dc, 0)
	}
	procSelectObject.Call(g.dc, old)
	return r
}

// measure is the single-line width of s.
func (g *painter) measure(s string, font uintptr) int32 {
	r := g.text(s, font, 0, rect{0, 0, 10000, 100}, dtSingleLine|dtCalcRect, 0)
	return r.right - r.left
}

// wrapHeight is the height of s word-wrapped to width.
func (g *painter) wrapHeight(s string, font uintptr, width int32) int32 {
	r := g.text(s, font, 0, rect{0, 0, width, 1000}, dtWordBreak|dtCalcRect, 0)
	return r.bottom - r.top
}

func (p *winPanel) addHit(k hitKey, r rect) {
	p.hits = append(p.hits, panelHit{key: k, r: r})
}

func (p *winPanel) hitAt(x, y int32) (hitKey, bool) {
	for _, h := range p.hits {
		if h.r.contains(x, y) {
			return h.key, true
		}
	}
	return hitKey{}, false
}

// button draws a push button: primary (accent fill), secondary or ghost
// (transparent, danger text).
func (p *winPanel) button(g *painter, k hitKey, r rect, label string, kind int) {
	p.addHit(k, r)
	hover := p.hover == k
	fill, border, color, font, noFill := colBtn, colLine, colText, p.fonts.body, false
	switch kind {
	case btnPrimary:
		fill, border, font = colAccent, colAccent, p.fonts.semi
		if hover {
			fill, border = colAccentHover, colAccentHover
		}
	case btnGhost:
		color, noFill = colDanger, !hover
		if hover {
			fill = colBtn
		}
	default:
		if hover {
			fill = colBtnHover
		}
	}
	g.shape(r, 12, fill, border, noFill)
	g.text(label, font, color, r, dtCenter|dtVCenter|dtSingleLine, 0)
}

const (
	btnSecondary = iota
	btnPrimary
	btnGhost
)

// render lays out (and, when g.draw, paints) the whole client area top to
// bottom, rebuilding the hit areas; it returns the content height.
func (p *winPanel) render(g *painter) int32 {
	p.hits = p.hits[:0]
	m := p.model
	f := p.fonts
	const w = panelWidth
	const x0, x1 = panelPad, w - panelPad

	// Hero: status pill, code, copy link.
	y := int32(22)
	pillW := 10 + 8 + 6 + g.measure(m.Host, f.small) + 10
	if pillW > w-2*panelPad {
		pillW = w - 2*panelPad
	}
	pill := rect{(w - pillW) / 2, y, (w + pillW) / 2, y + 24}
	g.shape(pill, 12, colPillFill, colPillBorder, false)
	g.shape(rect{pill.left + 10, y + 8, pill.left + 18, y + 16}, -1, colOK, colOK, false)
	g.text(m.Host, f.small, colOK, rect{pill.left + 24, y, pill.right - 10, pill.bottom}, dtSingleLine|dtVCenter|dtEndEllip, 0)
	y = pill.bottom + 6

	code := m.Code
	if code == "" {
		code = "—"
	}
	g.text(code, f.code, colText, rect{x0, y, x1, y + 46}, dtCenter|dtVCenter|dtSingleLine, 2)
	y += 46 + 4

	hint := "Código de verificación · "
	link := "Copiar"
	if p.copiedShown {
		link = "Copiado"
	}
	if m.Code == "" {
		hint, link = "Código de verificación no disponible", ""
	}
	hw, lw := g.measure(hint, f.small), int32(0)
	if link != "" {
		lw = g.measure(link, f.small)
	}
	hx := (w - hw - lw) / 2
	g.text(hint, f.small, colMuted, rect{hx, y, hx + hw, y + 18}, dtSingleLine|dtVCenter, 0)
	if link != "" {
		lr := rect{hx + hw, y, hx + hw + lw, y + 18}
		k := hitKey{kind: hitCopy}
		p.addHit(k, rect{lr.left - 4, lr.top - 3, lr.right + 4, lr.bottom + 3})
		font := f.small
		if p.hover == k {
			font = f.link
		}
		g.text(link, font, colAccentHi, lr, dtSingleLine|dtVCenter, 0)
	}
	y += 18 + 18

	// Request banner: the oldest pending request.
	if len(m.Requests) > 0 {
		req := m.Requests[0]
		const ix0, ix1 = x0 + panelCardPad, x1 - panelCardPad
		detailH := g.wrapHeight(req.Detail, f.small, ix1-ix0)
		more := panelMoreRequests(len(m.Requests) - 1)
		h := int32(panelCardPad + 22 + 2 + detailH + 12 + panelBtnH + panelCardPad)
		if more != "" {
			h += 18
		}
		g.shape(rect{x0, y, x1, y + h}, 18, colBanner, colAccent, false)
		iy := y + panelCardPad
		g.text(req.Title, f.title, colText, rect{ix0, iy, ix1, iy + 22}, dtSingleLine|dtVCenter|dtEndEllip, 0)
		iy += 22 + 2
		g.text(req.Detail, f.small, colMuted, rect{ix0, iy, ix1, iy + detailH}, dtWordBreak, 0)
		iy += detailH
		if more != "" {
			g.text(more, f.small, colMuted, rect{ix0, iy, ix1, iy + 18}, dtSingleLine|dtVCenter, 0)
			iy += 18
		}
		iy += 12
		bw := int32(ix1-ix0-8) / 2
		p.button(g, hitKey{hitDeny, req.ID}, rect{ix0, iy, ix0 + bw, iy + panelBtnH}, "No, rechazar", btnSecondary)
		p.button(g, hitKey{hitApprove, req.ID}, rect{ix1 - bw, iy, ix1, iy + panelBtnH}, "Sí, aprobar", btnPrimary)
		y += h + 14
	}

	// Paired phones card.
	{
		const ix0, ix1 = x0 + panelCardPad, x1 - panelCardPad
		rows := m.Devices
		if len(rows) > panelMaxRows {
			rows = rows[:panelMaxRows]
		}
		more := panelMoreDevices(len(m.Devices) - len(rows))
		listH := int32(len(rows)) * panelRowH
		if len(rows) == 0 {
			listH = 40
		}
		if more != "" {
			listH += 30
		}
		h := int32(panelCardPad+16+8+panelCardPad) + listH
		g.shape(rect{x0, y, x1, y + h}, 18, colCard, colLine, false)
		iy := y + panelCardPad
		g.text("TELÉFONOS EMPAREJADOS", f.section, colMuted, rect{ix0, iy, ix1, iy + 16}, dtSingleLine|dtVCenter, 1)
		iy += 16 + 8
		if len(rows) == 0 {
			g.text("Ningún teléfono emparejado.", f.body, colMuted, rect{ix0, iy, ix1, iy + 40}, dtSingleLine|dtVCenter, 0)
		}
		for i, d := range rows {
			ry := iy + int32(i)*panelRowH
			if i > 0 {
				g.fill(rect{ix0, ry, ix1, ry + 1}, colLine)
			}
			bw := g.measure("Revocar", f.body) + 30
			b := rect{ix1 - bw, ry + (panelRowH-34)/2, ix1, ry + (panelRowH+34)/2}
			p.button(g, hitKey{hitRevoke, d.ID}, b, "Revocar", btnGhost)
			tx1 := b.left - 10
			nw := g.measure(d.Name, f.bold)
			if nw > tx1-ix0 {
				nw = tx1 - ix0
			}
			g.text(d.Name, f.bold, colText, rect{ix0, ry + 10, ix0 + nw, ry + 30}, dtSingleLine|dtBottom|dtEndEllip, 0)
			if ix0+nw < tx1 {
				g.text(" · "+d.IP, f.small, colMuted, rect{ix0 + nw, ry + 10, tx1, ry + 29}, dtSingleLine|dtBottom|dtEndEllip, 0)
			}
			if d.Date != "" {
				g.text(d.Date, f.small, colMuted, rect{ix0, ry + 30, tx1, ry + 48}, dtSingleLine|dtVCenter|dtEndEllip, 0)
			}
		}
		if more != "" {
			ry := iy + int32(len(rows))*panelRowH
			g.fill(rect{ix0, ry, ix1, ry + 1}, colLine)
			g.text(more, f.small, colMuted, rect{ix0, ry + 1, ix1, ry + 30}, dtSingleLine|dtVCenter, 0)
		}
		y += h + panelPad
	}

	// Footer: autostart switch.
	g.fill(rect{0, y, w, y + panelFooterH}, colFooter)
	g.fill(rect{0, y, w, y + 1}, colLine)
	fy := y + 1
	label := colText
	if !p.autostartOK {
		label = colMuted
	}
	g.text("Iniciar con Windows", f.body, label, rect{x0, fy, x1 - 50, fy + 46}, dtSingleLine|dtVCenter, 0)
	tr := rect{x1 - 40, fy + 12, x1, fy + 34}
	track, knob := colLine, colKnobOff
	switch {
	case !p.autostartOK:
		track, knob = colBtn, colOffDisabled
	case p.autostartOn:
		track, knob = colAccent, colKnob
		if p.hover.kind == hitAutostart {
			track = colAccentHover
		}
	case p.hover.kind == hitAutostart:
		track = rgb(0x55, 0x55, 0x5E)
	}
	g.shape(tr, 11, track, track, false)
	kx := tr.left + 3
	if p.autostartOn {
		kx = tr.right - 19
	}
	g.shape(rect{kx, tr.top + 3, kx + 16, tr.top + 19}, -1, knob, knob, false)
	if p.autostartOK {
		p.addHit(hitKey{kind: hitAutostart}, rect{x0, fy, x1, fy + 46})
	}
	return y + panelFooterH
}

func (p *winPanel) paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var cr rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	cw, ch := cr.right-cr.left, cr.bottom-cr.top
	if cw <= 0 || ch <= 0 {
		return
	}
	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	if mem == 0 {
		return
	}
	defer procDeleteDC.Call(mem)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(cw), uintptr(ch))
	if bmp == 0 {
		return
	}
	defer procDeleteObject.Call(bmp)
	old, _, _ := procSelectObject.Call(mem, bmp)
	defer procSelectObject.Call(mem, old) // runs before the deletes above

	g := &painter{dc: mem, draw: true}
	procSetBkMode.Call(mem, bkTransparent)
	g.fill(cr, colBg)
	p.render(g)
	procBitBlt.Call(hdc, 0, 0, uintptr(cw), uintptr(ch), mem, 0, 0, srcCopy)
}

func (p *winPanel) activate(k hitKey) {
	t := p.tray
	switch k.kind {
	case hitCopy:
		t.copyCode()
		p.copiedUntil = time.Now().Add(panelCopiedFor)
	case hitApprove, hitDeny:
		var err error
		if k.kind == hitApprove {
			if err = t.broker.Approve(k.arg); err == nil {
				t.logf("pairing request %s approved from the panel", shortID(k.arg))
			}
		} else if err = t.broker.Deny(k.arg); err == nil {
			t.logf("pairing request %s denied from the panel", shortID(k.arg))
		}
		if err != nil {
			t.logf("panel answer for %s failed: %v", shortID(k.arg), err)
		}
	case hitRevoke:
		// The confirmation is modal and pumps messages: ignore clicks
		// meanwhile; the panel may even be closed when it returns.
		p.busy = true
		t.revoke(k.arg)
		p.busy = false
	case hitAutostart:
		if t.autostartBusy.Load() || !p.autostartOK {
			return
		}
		if t.setAutostart(!p.autostartOn) {
			p.autostartOn = !p.autostartOn
		}
		p.relayout()
		return
	}
	if p.hwnd != 0 {
		p.refresh()
	}
}

func (p *winPanel) setHover(k hitKey) {
	if k != p.hover {
		p.hover = k
		procInvalidateRect.Call(p.hwnd, 0, 0)
	}
}

func (p *winPanel) cursorHit() bool {
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procScreenToClient.Call(p.hwnd, uintptr(unsafe.Pointer(&pt)))
	_, ok := p.hitAt(pt.x, pt.y)
	return ok
}

func (p *winPanel) destroyResources() {
	f := &p.fonts
	for _, h := range []*uintptr{&f.body, &f.bold, &f.semi, &f.small, &f.link, &f.section, &f.title, &f.code} {
		if *h != 0 {
			procDeleteObject.Call(*h)
			*h = 0
		}
	}
}

func lParamXY(l uintptr) (int32, int32) {
	return int32(int16(l & 0xFFFF)), int32(int16((l >> 16) & 0xFFFF))
}

func panelWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	p := activePanel
	if p == nil || (p.hwnd != 0 && hwnd != p.hwnd) {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	switch uint32(msg) {
	case wmEraseBkgnd:
		return 1 // WM_PAINT covers everything
	case wmPaint:
		p.paint(hwnd)
		return 0
	case wmSetCursor:
		if lParam&0xFFFF == htClient && p.hwnd != 0 && !p.busy && p.cursorHit() {
			hand, _, _ := procLoadCursorW.Call(0, idcHand)
			procSetCursor.Call(hand)
			return 1
		}
	case wmMouseMove:
		if !p.tracking {
			tme := trackMouseEvent{dwFlags: tmeLeave, hwndTrack: hwnd}
			tme.cbSize = uint32(unsafe.Sizeof(tme))
			if r, _, _ := procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme))); r != 0 {
				p.tracking = true
			}
		}
		k, _ := p.hitAt(lParamXY(lParam))
		p.setHover(k)
		return 0
	case wmMouseLeave:
		p.tracking = false
		p.pressed = hitKey{}
		p.setHover(hitKey{})
		return 0
	case wmLButtonDown:
		p.pressed, _ = p.hitAt(lParamXY(lParam))
		return 0
	case wmLButtonUp:
		k, ok := p.hitAt(lParamXY(lParam))
		pressed := p.pressed
		p.pressed = hitKey{}
		if ok && k == pressed && !p.busy {
			p.activate(k)
		}
		return 0
	case wmKeyDown:
		if wParam == vkEscape && !p.busy {
			procDestroyWindow.Call(hwnd)
			return 0
		}
	case wmTimer:
		if wParam == panelTimerID && !p.busy {
			p.refresh()
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procKillTimer.Call(hwnd, panelTimerID)
		p.destroyResources()
		p.hwnd = 0
		activePanel = nil
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}
