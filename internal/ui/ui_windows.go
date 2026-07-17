//go:build windows

package ui

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"github.com/mewisme/vutils/internal/app"
	"github.com/mewisme/vutils/internal/config"
	"github.com/mewisme/vutils/internal/update"
	"github.com/mewisme/vutils/internal/version"
)

// Control IDs — keep stable where possible for handlers.
const (
	classMain = "VUtilsMain"

	idEnabled = 1001
	idCalib   = 1002
	idCircle  = 1003
	idHLine   = 1004
	idVLine   = 1005

	idX    = 1010
	idY    = 1011
	idW    = 1012
	idH    = 1013
	idStep = 1014

	idUp    = 1050
	idDown  = 1051
	idLeft  = 1052
	idRight = 1053
	idWP    = 1054
	idWM    = 1055
	idHP    = 1056
	idHM    = 1057

	idWDisp = 1060 // Size section width readout
	idHDisp = 1061 // Size section height readout

	idColor        = 1020
	idColorPreview = 1024
	idColorPick    = 1026
	idThick        = 1021
	idOpacity      = 1022
	idOpacityBar   = 1025

	idSave  = 1040
	idReset = 1041

	idProfileCombo = 1070
	idStatus       = 1071
	idFooterCopy   = 1072
	idFooterVer    = 1073

	idAboutGitHub      = 1100
	idMenuExit         = 1101
	idAbout            = 1102
	idCheckUpdate      = 1103
	idProfileNew       = 1104
	idProfileDelete    = 1105
	idProfileDuplicate = 1106
	idProfileRename    = 1107

	holdTimerID  = 1
	holdDelayMs  = 400
	holdRepeatMs = 60
)

// Layout constants (client-area coordinates, pixels).
const (
	winW = 460 // outer window width
	winH = 552 // outer window height (menu + profile + footer)

	padX            = 14 // left/right page margin
	padTop          = 0  // Overlay flush under caption
	padBottom       = 2  // gap above footer
	footerH         = 22 // status + meta one line
	padFooterBottom = 2  // empty space under footer

	contentW = 420 // usable width inside margins

	rowH  = 22 // shared height for edits + small buttons (keeps baselines aligned)
	btnH  = 22 // same as rowH — avoid tall buttons next to edits
	gapY  = 6  // vertical gap between sections
	editW = 64 // numeric edit width
)

// Win32 extras not always exported by lxn/win.
const (
	bsGroupBox     = win.BS_GROUPBOX
	tbsAutoTicks   = 0x0001
	tbsHorz        = 0x0000
	tbClass        = "msctls_trackbar32"
	spiGetWorkArea = 0x0030

	holdNone = 0
	holdPos  = 1
	holdSize = 2
)

type form struct {
	svc          *app.Service
	hwnd         win.HWND
	font         win.HFONT
	previewBrush win.HBRUSH
	custColors   [16]win.COLORREF // persisted across ChooseColor opens
	updating     bool             // suppress live-apply while loading fields

	// Hold-to-repeat for D-pad / Size buttons.
	holdKind      int
	holdA, holdB  int
	holdRepeating bool
}

// formByHWND keeps *form alive for the wndproc without storing Go pointers as uintptr.
var formByHWND = map[win.HWND]*form{}

// holdBtnOrig / holdBtnForm support subclassed nudge buttons.
var (
	holdBtnCallback = syscall.NewCallback(holdBtnProc)
	holdBtnOrig     = map[win.HWND]uintptr{}
	holdBtnForm     = map[win.HWND]*form{}
	holdBtnAct      = map[win.HWND][3]int{} // kind, a, b
)

// utf16Ptr wraps UTF16PtrFromString for Win32 APIs (NUL in s → empty string).
func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

// Run opens the native Win32 settings window and blocks until closed.
// Standalone only — never touches Valorant / Vanguard.
func Run(svc *app.Service) error {
	f := &form{svc: svc}
	return f.run()
}

func (f *form) run() error {
	initCommonControls()

	instance := win.GetModuleHandle(nil)
	if err := registerMainClass(instance); err != nil {
		return err
	}

	f.font = createUIFont()
	defer func() {
		if f.font != 0 {
			win.DeleteObject(win.HGDIOBJ(f.font))
		}
		if f.previewBrush != 0 {
			win.DeleteObject(win.HGDIOBJ(f.previewBrush))
		}
	}()

	x, y := centerPos(winW, winH)
	hwnd := win.CreateWindowEx(
		0,
		utf16Ptr(classMain),
		utf16Ptr("Valorant Utils"),
		win.WS_OVERLAPPED|win.WS_CAPTION|win.WS_SYSMENU|win.WS_MINIMIZEBOX|win.WS_VISIBLE,
		x, y, winW, winH,
		0, 0, instance, nil,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx main failed: %d", win.GetLastError())
	}
	f.hwnd = hwnd
	formByHWND[hwnd] = f
	defer delete(formByHWND, hwnd)
	attachMenu(hwnd)

	f.buildControls()
	f.loadFields(f.svc.GetConfig())
	f.refreshProfileCombo()

	if err := f.svc.Start(); err != nil {
		win.MessageBox(hwnd, utf16Ptr(err.Error()), utf16Ptr("Valorant Utils"), win.MB_OK|win.MB_ICONERROR)
	}

	win.ShowWindow(hwnd, win.SW_SHOW)
	win.UpdateWindow(hwnd)

	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) > 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	f.svc.Shutdown()
	return nil
}

func centerPos(w, h int32) (x, y int32) {
	var wa win.RECT
	if win.SystemParametersInfo(spiGetWorkArea, 0, unsafe.Pointer(&wa), 0) {
		return wa.Left + (wa.Right-wa.Left-w)/2, wa.Top + (wa.Bottom-wa.Top-h)/2
	}
	sw := win.GetSystemMetrics(win.SM_CXSCREEN)
	sh := win.GetSystemMetrics(win.SM_CYSCREEN)
	return (sw - w) / 2, (sh - h) / 2
}

func registerMainClass(instance win.HINSTANCE) error {
	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(mainWndProc)
	wc.HInstance = instance
	wc.HCursor = win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
	wc.HbrBackground = win.COLOR_BTNFACE + 1
	wc.LpszClassName = utf16Ptr(classMain)
	// ID 1 = icon from rsrc_windows_amd64.syso (built from icon.ico).
	if h := win.LoadIcon(instance, win.MAKEINTRESOURCE(1)); h != 0 {
		wc.HIcon = h
		wc.HIconSm = h
	}
	if atom := win.RegisterClassEx(&wc); atom == 0 {
		if e := win.GetLastError(); e != 1410 {
			return fmt.Errorf("RegisterClassEx main: %d", e)
		}
	}
	return nil
}

func initCommonControls() {
	var icc win.INITCOMMONCONTROLSEX
	icc.DwSize = uint32(unsafe.Sizeof(icc))
	icc.DwICC = win.ICC_BAR_CLASSES | win.ICC_STANDARD_CLASSES | win.ICC_WIN95_CLASSES
	win.InitCommonControlsEx(&icc)
}

func createUIFont() win.HFONT {
	var lf win.LOGFONT
	lf.LfHeight = -12
	lf.LfWeight = win.FW_NORMAL
	name, _ := syscall.UTF16FromString("Segoe UI")
	copy(lf.LfFaceName[:], name)
	if h := win.CreateFontIndirect(&lf); h != 0 {
		return h
	}
	return win.HFONT(win.GetStockObject(win.DEFAULT_GUI_FONT))
}

var (
	libUser32             = syscall.NewLazyDLL("user32.dll")
	libGdi32              = syscall.NewLazyDLL("gdi32.dll")
	procSetWindowTextW    = libUser32.NewProc("SetWindowTextW")
	procGetWindowTextW    = libUser32.NewProc("GetWindowTextW")
	procGetWindowTextLenW = libUser32.NewProc("GetWindowTextLengthW")
	procAppendMenuW       = libUser32.NewProc("AppendMenuW")
	procCreateSolidBrush  = libGdi32.NewProc("CreateSolidBrush")
)

func setWindowText(hwnd win.HWND, text string) {
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func getWindowTextLength(hwnd win.HWND) int {
	r, _, _ := procGetWindowTextLenW.Call(uintptr(hwnd))
	return int(r)
}

func getWindowText(hwnd win.HWND) string {
	n := getWindowTextLength(hwnd)
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func createSolidBrush(color win.COLORREF) win.HBRUSH {
	r, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return win.HBRUSH(r)
}

func appendMenu(hMenu win.HMENU, flags uint32, id uintptr, text string) bool {
	r, _, _ := procAppendMenuW.Call(
		uintptr(hMenu),
		uintptr(flags),
		id,
		uintptr(unsafe.Pointer(utf16Ptr(text))),
	)
	return r != 0
}

func attachMenu(hwnd win.HWND) {
	bar := win.CreateMenu()

	file := win.CreatePopupMenu()
	appendMenu(file, win.MF_STRING, uintptr(idSave), "&Save")
	appendMenu(file, win.MF_STRING, uintptr(idReset), "&Reset Defaults")
	appendMenu(file, win.MF_SEPARATOR, 0, "")
	appendMenu(file, win.MF_STRING, uintptr(idMenuExit), "E&xit")

	profile := win.CreatePopupMenu()
	appendMenu(profile, win.MF_STRING, uintptr(idProfileNew), "&New Profile…")
	appendMenu(profile, win.MF_STRING, uintptr(idProfileDuplicate), "Du&plicate…")
	appendMenu(profile, win.MF_STRING, uintptr(idProfileRename), "&Rename…")
	appendMenu(profile, win.MF_SEPARATOR, 0, "")
	appendMenu(profile, win.MF_STRING, uintptr(idProfileDelete), "&Delete Profile")

	help := win.CreatePopupMenu()
	appendMenu(help, win.MF_STRING, uintptr(idCheckUpdate), "Check for &Updates…")
	appendMenu(help, win.MF_SEPARATOR, 0, "")
	appendMenu(help, win.MF_STRING, uintptr(idAbout), "&About…")
	appendMenu(help, win.MF_STRING, uintptr(idAboutGitHub), "&GitHub: mewisme/vutils")

	appendMenu(bar, win.MF_POPUP, uintptr(file), "&File")
	appendMenu(bar, win.MF_POPUP, uintptr(profile), "&Profile")
	appendMenu(bar, win.MF_POPUP, uintptr(help), "&Help")
	win.SetMenu(hwnd, bar)
	win.DrawMenuBar(hwnd)
}

func openURL(url string) {
	win.ShellExecute(0, utf16Ptr("open"), utf16Ptr(url), nil, nil, win.SW_SHOWNORMAL)
}

func (f *form) showAbout() {
	text := "Valorant Utils (vutils) " + version.String() + "\n\n" +
		"Standalone minimap guide overlay.\n" +
		"Does not attach to Valorant or Vanguard.\n\n" +
		"https://github.com/mewisme/vutils"
	win.MessageBox(f.hwnd, utf16Ptr(text), utf16Ptr("About Valorant Utils"), win.MB_OK|win.MB_ICONINFORMATION)
}

func (f *form) onCheckUpdate() {
	res, err := update.Check(version.Version)
	if err != nil {
		win.MessageBox(f.hwnd, utf16Ptr(err.Error()), utf16Ptr("Check for Updates"), win.MB_OK|win.MB_ICONWARNING)
		return
	}
	if !res.Newer {
		msg := "You're up to date.\n\nCurrent: " + version.String() + "\nLatest: v" + res.Latest
		win.MessageBox(f.hwnd, utf16Ptr(msg), utf16Ptr("Check for Updates"), win.MB_OK|win.MB_ICONINFORMATION)
		return
	}
	msg := "Update available.\n\nCurrent: " + version.String() + "\nLatest: v" + res.Latest + "\n\nOpen download page?"
	if win.MessageBox(f.hwnd, utf16Ptr(msg), utf16Ptr("Check for Updates"), win.MB_YESNO|win.MB_ICONQUESTION) == win.IDYES {
		openURL(res.URL)
	}
}

func (f *form) buildControls() {
	font := f.font
	add := func(class string, text string, style, exStyle uint32, x, y, w, h int32, id int) win.HWND {
		child := win.CreateWindowEx(
			exStyle,
			utf16Ptr(class),
			utf16Ptr(text),
			win.WS_CHILD|win.WS_VISIBLE|style,
			x, y, w, h,
			f.hwnd, win.HMENU(id), win.GetModuleHandle(nil), nil,
		)
		if font != 0 {
			win.SendMessage(child, win.WM_SETFONT, uintptr(font), 1)
		}
		return child
	}
	groupBox := func(title string, x, y, w, h int32) {
		add("BUTTON", title, bsGroupBox, 0, x, y, w, h, 0)
	}

	// Start flush at top; padBottom leaves space under Actions via winH.
	y := int32(padTop)

	// --- 0. Profile ---
	groupH := int32(48)
	groupBox("Profile", padX, y, contentW, groupH)
	add("STATIC", "Active", 0, 0, padX+12, y+18, 44, rowH, 0)
	add("COMBOBOX", "", win.CBS_DROPDOWNLIST|win.WS_VSCROLL, 0, padX+60, y+16, contentW-80, 200, idProfileCombo)
	y += groupH + gapY

	// --- 1. Overlay ---
	groupH = int32(78)
	groupBox("Overlay", padX, y, contentW, groupH)
	add("BUTTON", "Enable Overlay", win.BS_AUTOCHECKBOX, 0, padX+12, y+20, 120, rowH, idEnabled)
	add("BUTTON", "Calibration", win.BS_AUTOCHECKBOX, 0, padX+140, y+20, 100, rowH, idCalib)
	add("BUTTON", "Circle Mode", win.BS_AUTOCHECKBOX, 0, padX+250, y+20, 110, rowH, idCircle)
	add("BUTTON", "Horizontal", win.BS_AUTOCHECKBOX, 0, padX+12, y+48, 100, rowH, idHLine)
	add("BUTTON", "Vertical", win.BS_AUTOCHECKBOX, 0, padX+120, y+48, 90, rowH, idVLine)
	y += groupH + gapY

	// --- 2. Minimap Position (fields left | compact D-pad right) ---
	groupH = 100
	groupBox("Minimap Position", padX, y, contentW, groupH)
	gy := y + 20
	// Left: X/Y, W/H, Step — fixed columns
	add("STATIC", "X", 0, 0, padX+12, gy+2, 16, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+30, gy, editW, rowH, idX)
	add("STATIC", "Y", 0, 0, padX+104, gy+2, 16, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+122, gy, editW, rowH, idY)
	gy += 26
	add("STATIC", "W", 0, 0, padX+12, gy+2, 16, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+30, gy, editW, rowH, idW)
	add("STATIC", "H", 0, 0, padX+104, gy+2, 16, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+122, gy, editW, rowH, idH)
	gy += 26
	add("STATIC", "Step", 0, 0, padX+12, gy+2, 36, rowH, 0)
	add("EDIT", "5", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+50, gy, 48, rowH, idStep)

	// Right: tight D-pad (24px cells, 2px gap)
	const (
		padCell = int32(24)
		padGap  = int32(2)
	)
	ax := int32(padX) + 250
	ay := y + 14 // flush under group title
	add("BUTTON", "↑", win.BS_PUSHBUTTON, 0, ax+padCell+padGap, ay, padCell, padCell, idUp)
	add("BUTTON", "←", win.BS_PUSHBUTTON, 0, ax, ay+padCell+padGap, padCell, padCell, idLeft)
	add("BUTTON", "→", win.BS_PUSHBUTTON, 0, ax+2*(padCell+padGap), ay+padCell+padGap, padCell, padCell, idRight)
	add("BUTTON", "↓", win.BS_PUSHBUTTON, 0, ax+padCell+padGap, ay+2*(padCell+padGap), padCell, padCell, idDown)
	f.wireHoldButton(idUp, holdPos, 0, -1)
	f.wireHoldButton(idDown, holdPos, 0, 1)
	f.wireHoldButton(idLeft, holdPos, -1, 0)
	f.wireHoldButton(idRight, holdPos, 1, 0)
	y += groupH + gapY

	// --- 3. Size (aligned label | − | value | + columns) ---
	groupH = 76 // room for bottom pad under Height row
	groupBox("Size", padX, y, contentW, groupH)
	const (
		szLabel = int32(52)
		szBtn   = int32(36)
		szVal   = int32(56)
		szGap   = int32(6)
	)
	szL := int32(padX) + 12
	szM := szL + szLabel       // minus button
	szV := szM + szBtn + szGap // value
	szP := szV + szVal + szGap // plus button
	gy = y + 14                // flush under group title (no extra top pad)
	add("STATIC", "Width", 0, 0, szL, gy+2, szLabel, rowH, 0)
	add("BUTTON", "W−", win.BS_PUSHBUTTON, 0, szM, gy, szBtn, rowH, idWM)
	add("STATIC", "300", win.SS_CENTER|win.SS_CENTERIMAGE, win.WS_EX_CLIENTEDGE, szV, gy, szVal, rowH, idWDisp)
	add("BUTTON", "W+", win.BS_PUSHBUTTON, 0, szP, gy, szBtn, rowH, idWP)
	gy += 28
	add("STATIC", "Height", 0, 0, szL, gy+2, szLabel, rowH, 0)
	add("BUTTON", "H−", win.BS_PUSHBUTTON, 0, szM, gy, szBtn, rowH, idHM)
	add("STATIC", "300", win.SS_CENTER|win.SS_CENTERIMAGE, win.WS_EX_CLIENTEDGE, szV, gy, szVal, rowH, idHDisp)
	add("BUTTON", "H+", win.BS_PUSHBUTTON, 0, szP, gy, szBtn, rowH, idHP)
	f.wireHoldButton(idWP, holdSize, 1, 0)
	f.wireHoldButton(idWM, holdSize, -1, 0)
	f.wireHoldButton(idHP, holdSize, 0, 1)
	f.wireHoldButton(idHM, holdSize, 0, -1)
	// bottom pad ≈ groupH - (14 + 28 + rowH) ≈ 14px
	y += groupH + gapY

	// --- 4. Style (one baseline: same height for edit/preview/pick) ---
	groupH = 92
	groupBox("Style", padX, y, contentW, groupH)
	gy = y + 22
	add("STATIC", "Color", 0, 0, padX+12, gy+2, 40, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+54, gy, 86, rowH, idColor)
	add("STATIC", "", win.SS_CENTER, win.WS_EX_CLIENTEDGE, padX+146, gy, rowH, rowH, idColorPreview)
	add("BUTTON", "Pick…", win.BS_PUSHBUTTON, 0, padX+174, gy, 56, rowH, idColorPick)
	add("STATIC", "Thick", 0, 0, padX+240, gy+2, 40, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+284, gy, 48, rowH, idThick)
	gy += 30
	add("STATIC", "Opacity", 0, 0, padX+12, gy+2, 52, rowH, 0)
	add("EDIT", "", win.WS_BORDER|win.ES_NUMBER|win.ES_CENTER, win.WS_EX_CLIENTEDGE, padX+68, gy, 48, rowH, idOpacity)
	add("STATIC", "%", 0, 0, padX+120, gy+2, 16, rowH, 0)
	bar := add(tbClass, "", tbsHorz|tbsAutoTicks, 0, padX+144, gy, contentW-168, rowH, idOpacityBar)
	win.SendMessage(bar, win.TBM_SETRANGEMIN, 0, 0)
	win.SendMessage(bar, win.TBM_SETRANGEMAX, 0, 100)
	win.SendMessage(bar, win.TBM_SETPAGESIZE, 0, 5)
	y += groupH + gapY

	// --- 5. Actions (centered equal buttons) ---
	groupH = 52
	groupBox("Actions", padX, y, contentW, groupH)
	const (
		actW   = int32(120)
		actGap = int32(16)
		actH   = int32(28)
	)
	actTotal := actW*2 + actGap
	actX := padX + (contentW-actTotal)/2
	actY := y + (groupH-actH)/2 + 4 // optically center in groupbox
	add("BUTTON", "Save", win.BS_DEFPUSHBUTTON, 0, actX, actY, actW, actH, idSave)
	add("BUTTON", "Reset", win.BS_PUSHBUTTON, 0, actX+actW+actGap, actY, actW, actH, idReset)
	y += groupH + padBottom

	// --- 6. Footer: status left | © center | version right ---
	const footCol = contentW / 3
	add("STATIC", "Ready", 0, 0, padX, y, footCol, footerH, idStatus)
	add("STATIC", "© 2026 Mew", win.SS_CENTER, 0, padX+footCol, y, footCol, footerH, idFooterCopy)
	add("STATIC", version.String(), win.SS_RIGHT, 0, padX+2*footCol, y, contentW-2*footCol, footerH, idFooterVer)
}

func mainWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	f := formFrom(hwnd)
	switch msg {
	case win.WM_CTLCOLORSTATIC:
		if f == nil {
			break
		}
		ctrl := win.HWND(lParam)
		if ctrl == f.ctrl(idColorPreview) && f.previewBrush != 0 {
			hdc := win.HDC(wParam)
			// Match brush fill to COLORREF so preview equals overlay HEX.
			color := strings.ToUpper(f.getText(idColor))
			if r, g, b, ok := parseHexRGB(color); ok {
				cr := win.COLORREF(r | (g << 8) | (b << 16))
				win.SetBkColor(hdc, cr)
				win.SetTextColor(hdc, cr)
			}
			win.SetBkMode(hdc, win.OPAQUE)
			return uintptr(f.previewBrush)
		}
	case win.WM_HSCROLL:
		if f == nil {
			break
		}
		if win.HWND(lParam) == f.ctrl(idOpacityBar) {
			pos := int(win.SendMessage(f.ctrl(idOpacityBar), win.TBM_GETPOS, 0, 0))
			f.updating = true
			f.setText(idOpacity, strconv.Itoa(pos))
			f.updating = false
			f.applyLive()
		}
		return 0
	case win.WM_TIMER:
		if f != nil && wParam == holdTimerID {
			f.onHoldTimer()
			return 0
		}
	case win.WM_COMMAND:
		if f == nil {
			break
		}
		id := int(win.LOWORD(uint32(wParam)))
		notify := win.HIWORD(uint32(wParam))
		switch id {
		case idEnabled:
			if notify == win.BN_CLICKED {
				f.onToggleEnabled()
			}
		case idCalib:
			if notify == win.BN_CLICKED {
				f.onToggleCalib()
			}
		case idCircle:
			if notify == win.BN_CLICKED {
				f.onToggleCircle()
			}
		case idHLine, idVLine:
			if notify == win.BN_CLICKED {
				f.onToggleLines()
			}
		case idSave:
			// Menu HIWORD=0; BN_CLICKED is also 0 — covers button + File menu.
			if notify == win.BN_CLICKED {
				f.onSave()
			}
		case idReset:
			if notify == win.BN_CLICKED {
				f.onReset()
			}
		case idProfileNew:
			f.onProfileNew()
		case idProfileDuplicate:
			f.onProfileDuplicate()
		case idProfileRename:
			f.onProfileRename()
		case idProfileDelete:
			f.onProfileDelete()
		case idProfileCombo:
			if notify == win.CBN_SELCHANGE {
				f.onProfileSelect()
			}
		case idMenuExit:
			win.DestroyWindow(f.hwnd)
		case idAbout:
			f.showAbout()
		case idCheckUpdate:
			f.onCheckUpdate()
		case idAboutGitHub:
			openURL("https://github.com/mewisme/vutils")
		case idColorPick:
			if notify == win.BN_CLICKED {
				f.onPickColor()
			}
		// D-pad / Size: hold subclass handles nudges (avoid double-fire on BN_CLICKED).
		case idX, idY, idW, idH, idStep, idColor, idThick, idOpacity:
			if notify == win.EN_CHANGE && !f.updating {
				if id == idOpacity {
					f.syncOpacityBarFromEdit()
				}
				if id == idW || id == idH {
					f.syncSizeDisp()
				}
				if id == idColor {
					f.updateColorPreview()
				}
				f.applyLive()
			}
		}
		return 0
	case win.WM_DESTROY:
		if f != nil {
			f.stopHold()
		}
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func formFrom(hwnd win.HWND) *form {
	return formByHWND[hwnd]
}

func (f *form) ctrl(id int) win.HWND {
	return win.GetDlgItem(f.hwnd, int32(id))
}

func (f *form) setStatus(s string) {
	if f.hwnd == 0 {
		return
	}
	f.setText(idStatus, s)
}

func (f *form) getText(id int) string {
	return strings.TrimSpace(getWindowText(f.ctrl(id)))
}

func (f *form) setText(id int, s string) {
	setWindowText(f.ctrl(id), s)
}

func (f *form) checked(id int) bool {
	return win.SendMessage(f.ctrl(id), win.BM_GETCHECK, 0, 0) == win.BST_CHECKED
}

func (f *form) setChecked(id int, v bool) {
	val := uintptr(win.BST_UNCHECKED)
	if v {
		val = win.BST_CHECKED
	}
	win.SendMessage(f.ctrl(id), win.BM_SETCHECK, val, 0)
}

func (f *form) step() int {
	if n, err := strconv.Atoi(f.getText(idStep)); err == nil && n >= 1 {
		return n
	}
	if s := f.svc.GetConfig().Step; s >= 1 {
		return s
	}
	return 5
}

func (f *form) readCfg() (config.OverlayConfig, error) {
	x, errX := strconv.Atoi(f.getText(idX))
	y, errY := strconv.Atoi(f.getText(idY))
	w, errW := strconv.Atoi(f.getText(idW))
	h, errH := strconv.Atoi(f.getText(idH))
	step, errStep := strconv.Atoi(f.getText(idStep))
	th, errTh := strconv.Atoi(f.getText(idThick))
	opPct, errOp := strconv.Atoi(f.getText(idOpacity))

	if errW != nil || errH != nil || w < 1 || h < 1 {
		return config.OverlayConfig{}, errInvalidSize
	}
	if errStep != nil || step < 1 {
		step = 5
	}
	if errTh != nil || th < 1 {
		th = 1
	}
	if errOp != nil {
		opPct = 60
	}
	if opPct < 0 {
		opPct = 0
	}
	if opPct > 100 {
		opPct = 100
	}
	_ = errX
	_ = errY

	color := strings.ToUpper(f.getText(idColor))
	c := config.OverlayConfig{
		MapX: x, MapY: y, MapW: w, MapH: h,
		Step:           step,
		Thickness:      th,
		Opacity:        float64(opPct) / 100.0,
		Color:          color,
		Enabled:        f.checked(idEnabled),
		Calibration:    f.checked(idCalib),
		CircleMode:     f.checked(idCircle),
		ShowHorizontal: f.checked(idHLine),
		ShowVertical:   f.checked(idVLine),
	}
	validated, err := config.Validate(c)
	if err != nil {
		return c, err
	}
	return validated, nil
}

var (
	errInvalidSize  = fmt.Errorf("invalid size")
	errInvalidColor = fmt.Errorf("invalid color")
)

func statusForErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case err == errInvalidSize || strings.Contains(msg, "mapW") || strings.Contains(msg, "mapH"):
		return "Invalid minimap size"
	case strings.Contains(msg, "color") || err == errInvalidColor:
		return "Invalid color"
	case strings.Contains(msg, "thickness"):
		return "Thickness must be at least 1"
	case strings.Contains(msg, "opacity"):
		return "Opacity must be 0–100"
	default:
		return msg
	}
}

func (f *form) loadFields(c config.OverlayConfig) {
	f.updating = true
	defer func() { f.updating = false }()

	f.setChecked(idEnabled, c.Enabled)
	f.setChecked(idCalib, c.Calibration)
	f.setChecked(idCircle, c.CircleMode)
	f.setChecked(idHLine, c.ShowHorizontal)
	f.setChecked(idVLine, c.ShowVertical)
	f.setText(idX, strconv.Itoa(c.MapX))
	f.setText(idY, strconv.Itoa(c.MapY))
	f.setText(idW, strconv.Itoa(c.MapW))
	f.setText(idH, strconv.Itoa(c.MapH))
	step := c.Step
	if step < 1 {
		step = 5
	}
	f.setText(idStep, strconv.Itoa(step))
	f.setText(idThick, strconv.Itoa(c.Thickness))
	f.setText(idColor, c.Color)
	op := int(c.Opacity*100 + 0.5)
	if op < 0 {
		op = 0
	}
	if op > 100 {
		op = 100
	}
	f.setText(idOpacity, strconv.Itoa(op))
	win.SendMessage(f.ctrl(idOpacityBar), win.TBM_SETPOS, 1, uintptr(op))
	f.syncSizeDisp()
	f.updateColorPreview()
}

func (f *form) syncSizeDisp() {
	f.setText(idWDisp, f.getText(idW))
	f.setText(idHDisp, f.getText(idH))
}

func (f *form) syncOpacityBarFromEdit() {
	op, err := strconv.Atoi(f.getText(idOpacity))
	if err != nil {
		return
	}
	if op < 0 {
		op = 0
	}
	if op > 100 {
		op = 100
	}
	win.SendMessage(f.ctrl(idOpacityBar), win.TBM_SETPOS, 1, uintptr(op))
}

func (f *form) updateColorPreview() {
	color := strings.ToUpper(f.getText(idColor))
	r, g, b, ok := parseHexRGB(color)
	if !ok {
		return
	}
	if f.previewBrush != 0 {
		win.DeleteObject(win.HGDIOBJ(f.previewBrush))
	}
	f.previewBrush = createSolidBrush(win.COLORREF(r | (g << 8) | (b << 16)))
	win.InvalidateRect(f.ctrl(idColorPreview), nil, true)
}

// onPickColor opens the native Win32 ChooseColor dialog (standalone UI only).
func (f *form) onPickColor() {
	r, g, b, ok := parseHexRGB(f.getText(idColor))
	if !ok {
		r, g, b = 0, 255, 255 // fallback cyan if HEX invalid
	}
	initial := win.COLORREF(r | (g << 8) | (b << 16))

	cc := win.CHOOSECOLOR{
		LStructSize:  uint32(unsafe.Sizeof(win.CHOOSECOLOR{})),
		HwndOwner:    f.hwnd,
		RgbResult:    initial,
		LpCustColors: &f.custColors,
		Flags:        win.CC_FULLOPEN | win.CC_RGBINIT | win.CC_ANYCOLOR,
	}
	if !win.ChooseColor(&cc) {
		return // user cancelled
	}

	cr := uint32(cc.RgbResult)
	hex := fmt.Sprintf("#%02X%02X%02X", cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
	f.updating = true
	f.setText(idColor, hex)
	f.updating = false
	f.updateColorPreview()
	f.applyLive()
	f.setStatus("Color updated")
}

func parseHexRGB(s string) (r, g, b uint32, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	rv, err1 := strconv.ParseUint(s[0:2], 16, 8)
	gv, err2 := strconv.ParseUint(s[2:4], 16, 8)
	bv, err3 := strconv.ParseUint(s[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return uint32(rv), uint32(gv), uint32(bv), true
}

func (f *form) applyLive() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	// Clamp opacity field display if needed.
	op := int(c.Opacity*100 + 0.5)
	cur := f.getText(idOpacity)
	if cur != strconv.Itoa(op) {
		f.updating = true
		f.setText(idOpacity, strconv.Itoa(op))
		f.updating = false
	}
	if c.Thickness < 1 {
		c.Thickness = 1
	}
	if err := f.svc.UpdateOverlayConfig(c); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.syncSizeDisp()
	f.setStatus("Live")
}

func (f *form) onToggleEnabled() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if err := f.svc.SetOverlayEnabled(c.Enabled); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if c.Enabled {
		f.setStatus("Overlay on")
	} else {
		f.setStatus("Overlay off")
	}
}

func (f *form) onToggleCalib() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if err := f.svc.UpdateOverlayConfig(c); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if c.Calibration {
		f.setStatus("Calibration on")
	} else {
		f.setStatus("Calibration off")
	}
}

func (f *form) onToggleCircle() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if err := f.svc.UpdateOverlayConfig(c); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if c.CircleMode {
		f.setStatus("Circle mode on")
	} else {
		f.setStatus("Circle mode off")
	}
}

func (f *form) onToggleLines() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if err := f.svc.UpdateOverlayConfig(c); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.setStatus("Lines updated")
}

func (f *form) onSave() {
	c, err := f.readCfg()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	if err := f.svc.SaveConfig(c); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.setStatus("Saved")
}

func (f *form) onReset() {
	c, err := f.svc.ResetConfig()
	if err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.loadFields(c)
	f.setStatus("Reset to defaults")
}

func (f *form) nudgePos(dx, dy int) {
	step := f.step()
	c, err := f.readCfg()
	if err != nil {
		// Allow nudge of position even if color incomplete — use service cfg as base.
		c = f.svc.GetConfig()
		c.Enabled = f.checked(idEnabled)
		c.Calibration = f.checked(idCalib)
		c.CircleMode = f.checked(idCircle)
		c.ShowHorizontal = f.checked(idHLine)
		c.ShowVertical = f.checked(idVLine)
	}
	c.MapX += dx * step
	c.MapY += dy * step
	f.updating = true
	f.setText(idX, strconv.Itoa(c.MapX))
	f.setText(idY, strconv.Itoa(c.MapY))
	f.updating = false
	if err := f.svc.UpdateOverlayConfig(mustValidPos(c)); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.setStatus("Nudged")
}

func (f *form) nudgeSize(dw, dh int) {
	c, err := f.readCfg()
	if err != nil {
		c = f.svc.GetConfig()
		c.Enabled = f.checked(idEnabled)
		c.Calibration = f.checked(idCalib)
		c.CircleMode = f.checked(idCircle)
		c.ShowHorizontal = f.checked(idHLine)
		c.ShowVertical = f.checked(idVLine)
	}
	oldW, oldH := c.MapW, c.MapH
	c.MapW = max(1, oldW+dw)
	c.MapH = max(1, oldH+dh)
	// Keep crosshair intersection fixed; grow/shrink at edges.
	c.MapX += (oldW - c.MapW) / 2
	c.MapY += (oldH - c.MapH) / 2
	f.updating = true
	f.setText(idX, strconv.Itoa(c.MapX))
	f.setText(idY, strconv.Itoa(c.MapY))
	f.setText(idW, strconv.Itoa(c.MapW))
	f.setText(idH, strconv.Itoa(c.MapH))
	f.syncSizeDisp()
	f.updating = false
	if err := f.svc.UpdateOverlayConfig(mustValidPos(c)); err != nil {
		f.setStatus(statusForErr(err))
		return
	}
	f.setStatus("Resized")
}

func mustValidPos(c config.OverlayConfig) config.OverlayConfig {
	if c.MapW < 1 {
		c.MapW = 1
	}
	if c.MapH < 1 {
		c.MapH = 1
	}
	if c.Thickness < 1 {
		c.Thickness = 1
	}
	if c.Opacity < 0 {
		c.Opacity = 0
	}
	if c.Opacity > 1 {
		c.Opacity = 1
	}
	if c.Color == "" {
		c.Color = "#00FFFF"
	}
	if v, err := config.Validate(c); err == nil {
		return v
	}
	// Keep last-good color if typing incomplete during nudge.
	c.Color = "#00FFFF"
	v, _ := config.Validate(c)
	return v
}

func (f *form) wireHoldButton(id, kind, a, b int) {
	hwnd := f.ctrl(id)
	if hwnd == 0 {
		return
	}
	holdBtnForm[hwnd] = f
	holdBtnAct[hwnd] = [3]int{kind, a, b}
	orig := win.SetWindowLongPtr(hwnd, win.GWLP_WNDPROC, holdBtnCallback)
	holdBtnOrig[hwnd] = orig
}

func holdBtnProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	orig := holdBtnOrig[hwnd]
	f := holdBtnForm[hwnd]
	switch msg {
	case win.WM_LBUTTONDOWN:
		if f != nil {
			act := holdBtnAct[hwnd]
			f.startHold(act[0], act[1], act[2])
		}
	case win.WM_LBUTTONUP, win.WM_CAPTURECHANGED:
		if f != nil {
			f.stopHold()
		}
	case win.WM_DESTROY:
		delete(holdBtnOrig, hwnd)
		delete(holdBtnForm, hwnd)
		delete(holdBtnAct, hwnd)
	}
	if orig != 0 {
		return win.CallWindowProc(orig, hwnd, msg, wParam, lParam)
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func (f *form) startHold(kind, a, b int) {
	f.stopHold()
	f.holdKind = kind
	f.holdA, f.holdB = a, b
	f.holdRepeating = false
	f.doHoldNudge()
	win.SetTimer(f.hwnd, holdTimerID, holdDelayMs, 0)
}

func (f *form) stopHold() {
	win.KillTimer(f.hwnd, holdTimerID)
	f.holdKind = holdNone
	f.holdRepeating = false
}

func (f *form) onHoldTimer() {
	if f.holdKind == holdNone {
		return
	}
	f.doHoldNudge()
	if !f.holdRepeating {
		f.holdRepeating = true
		win.SetTimer(f.hwnd, holdTimerID, holdRepeatMs, 0)
	}
}

func (f *form) doHoldNudge() {
	switch f.holdKind {
	case holdPos:
		f.nudgePos(f.holdA, f.holdB)
	case holdSize:
		f.nudgeSize(f.holdA, f.holdB)
	}
}

func (f *form) refreshProfileCombo() {
	cb := f.ctrl(idProfileCombo)
	win.SendMessage(cb, win.CB_RESETCONTENT, 0, 0)
	active := f.svc.ActiveProfile()
	sel := 0
	for i, name := range f.svc.ListProfiles() {
		p, _ := syscall.UTF16PtrFromString(name)
		win.SendMessage(cb, win.CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p)))
		if name == active {
			sel = i
		}
	}
	win.SendMessage(cb, win.CB_SETCURSEL, uintptr(sel), 0)
}

func (f *form) comboSelectedProfile() string {
	cb := f.ctrl(idProfileCombo)
	idx := int(win.SendMessage(cb, win.CB_GETCURSEL, 0, 0))
	if idx < 0 {
		return ""
	}
	names := f.svc.ListProfiles()
	if idx >= len(names) {
		return ""
	}
	return names[idx]
}

func (f *form) flushActiveProfile() {
	c, err := f.readCfg()
	if err != nil {
		c = f.svc.GetConfig()
	}
	_ = f.svc.UpdateOverlayConfig(c)
}

func (f *form) onProfileSelect() {
	name := f.comboSelectedProfile()
	if name == "" || name == f.svc.ActiveProfile() {
		return
	}
	f.flushActiveProfile()
	if err := f.svc.SetActiveProfile(name); err != nil {
		f.setStatus(statusForErr(err))
		f.refreshProfileCombo()
		return
	}
	f.loadFields(f.svc.GetConfig())
	f.setStatus("Profile: " + name)
}

func (f *form) onProfileNew() {
	name, ok := promptProfileName(f.hwnd, f.font, "New Profile", "")
	if !ok {
		return
	}
	f.flushActiveProfile()
	if err := f.svc.CreateProfile(name); err != nil {
		win.MessageBox(f.hwnd, utf16Ptr(err.Error()), utf16Ptr("New Profile"), win.MB_OK|win.MB_ICONWARNING)
		return
	}
	f.refreshProfileCombo()
	f.loadFields(f.svc.GetConfig())
	f.setStatus("Created " + name)
}

func (f *form) onProfileDuplicate() {
	cur := f.svc.ActiveProfile()
	suggest := cur + " copy"
	name, ok := promptProfileName(f.hwnd, f.font, "Duplicate Profile", suggest)
	if !ok {
		return
	}
	f.flushActiveProfile()
	if err := f.svc.CreateProfile(name); err != nil {
		win.MessageBox(f.hwnd, utf16Ptr(err.Error()), utf16Ptr("Duplicate Profile"), win.MB_OK|win.MB_ICONWARNING)
		return
	}
	f.refreshProfileCombo()
	f.loadFields(f.svc.GetConfig())
	f.setStatus("Duplicated as " + name)
}

func (f *form) onProfileRename() {
	cur := f.svc.ActiveProfile()
	if cur == config.DefaultProfileName {
		win.MessageBox(f.hwnd, utf16Ptr("Cannot rename the default profile."), utf16Ptr("Rename Profile"), win.MB_OK|win.MB_ICONINFORMATION)
		return
	}
	name, ok := promptProfileName(f.hwnd, f.font, "Rename Profile", cur)
	if !ok {
		return
	}
	f.flushActiveProfile()
	if err := f.svc.RenameProfile(cur, name); err != nil {
		win.MessageBox(f.hwnd, utf16Ptr(err.Error()), utf16Ptr("Rename Profile"), win.MB_OK|win.MB_ICONWARNING)
		return
	}
	f.refreshProfileCombo()
	f.setStatus("Renamed to " + name)
}

func (f *form) onProfileDelete() {
	name := f.svc.ActiveProfile()
	if name == config.DefaultProfileName {
		win.MessageBox(f.hwnd, utf16Ptr("Cannot delete the default profile."), utf16Ptr("Delete Profile"), win.MB_OK|win.MB_ICONINFORMATION)
		return
	}
	msg := "Delete profile \"" + name + "\"?"
	if win.MessageBox(f.hwnd, utf16Ptr(msg), utf16Ptr("Delete Profile"), win.MB_YESNO|win.MB_ICONQUESTION) != win.IDYES {
		return
	}
	if err := f.svc.DeleteProfile(name); err != nil {
		win.MessageBox(f.hwnd, utf16Ptr(err.Error()), utf16Ptr("Delete Profile"), win.MB_OK|win.MB_ICONWARNING)
		return
	}
	f.refreshProfileCombo()
	f.loadFields(f.svc.GetConfig())
	f.setStatus("Deleted " + name)
}

const (
	classPrompt    = "VUtilsProfilePrompt"
	idPromptEdit   = 1
	idPromptOK     = 2
	idPromptCancel = 3
)

type promptState struct {
	hwnd   win.HWND
	edit   win.HWND
	result string
	ok     bool
	done   bool
}

var promptByHWND = map[win.HWND]*promptState{}

func promptProfileName(owner win.HWND, font win.HFONT, title, initial string) (string, bool) {
	instance := win.GetModuleHandle(nil)
	registerPromptClass(instance)

	var pr promptState
	hwnd := win.CreateWindowEx(
		win.WS_EX_DLGMODALFRAME,
		utf16Ptr(classPrompt),
		utf16Ptr(title),
		win.WS_POPUP|win.WS_CAPTION|win.WS_SYSMENU|win.WS_VISIBLE,
		0, 0, 320, 140,
		owner, 0, instance, nil,
	)
	if hwnd == 0 {
		return "", false
	}
	pr.hwnd = hwnd
	promptByHWND[hwnd] = &pr
	defer delete(promptByHWND, hwnd)

	// Center over owner.
	var orc, rc win.RECT
	win.GetWindowRect(owner, &orc)
	win.GetWindowRect(hwnd, &rc)
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	win.SetWindowPos(hwnd, 0,
		orc.Left+(orc.Right-orc.Left-w)/2,
		orc.Top+(orc.Bottom-orc.Top-h)/2,
		0, 0, win.SWP_NOSIZE|win.SWP_NOZORDER)

	add := func(class, text string, style uint32, x, y, cw, ch, id int32) win.HWND {
		child := win.CreateWindowEx(0, utf16Ptr(class), utf16Ptr(text),
			win.WS_CHILD|win.WS_VISIBLE|style, x, y, cw, ch, hwnd, win.HMENU(id), instance, nil)
		if font != 0 {
			win.SendMessage(child, win.WM_SETFONT, uintptr(font), 1)
		}
		return child
	}
	add("STATIC", "Name:", 0, 16, 18, 48, 22, 0)
	pr.edit = add("EDIT", initial, win.WS_BORDER|win.ES_AUTOHSCROLL, 70, 16, 220, 24, idPromptEdit)
	win.SendMessage(pr.edit, win.EM_SETLIMITTEXT, 64, 0)
	if initial != "" {
		win.SendMessage(pr.edit, win.EM_SETSEL, 0, ^uintptr(0)) // select all for easy overwrite
	}
	add("BUTTON", "OK", win.BS_DEFPUSHBUTTON, 120, 60, 80, 26, idPromptOK)
	add("BUTTON", "Cancel", win.BS_PUSHBUTTON, 210, 60, 80, 26, idPromptCancel)
	win.SetFocus(pr.edit)
	win.EnableWindow(owner, false)

	var msg win.MSG
	for !pr.done && win.GetMessage(&msg, 0, 0, 0) > 0 {
		if !win.IsDialogMessage(hwnd, &msg) {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}
	win.EnableWindow(owner, true)
	win.SetFocus(owner)
	return pr.result, pr.ok
}

func registerPromptClass(instance win.HINSTANCE) {
	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(promptWndProc)
	wc.HInstance = instance
	wc.HCursor = win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
	wc.HbrBackground = win.COLOR_BTNFACE + 1
	wc.LpszClassName = utf16Ptr(classPrompt)
	if atom := win.RegisterClassEx(&wc); atom == 0 {
		_ = win.GetLastError() // already registered is fine
	}
}

func promptWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	pr := promptByHWND[hwnd]
	switch msg {
	case win.WM_COMMAND:
		id := int(win.LOWORD(uint32(wParam)))
		switch id {
		case idPromptOK:
			if pr != nil {
				pr.result = strings.TrimSpace(getWindowText(pr.edit))
				if pr.result == "" {
					return 0
				}
				pr.ok = true
				pr.done = true
			}
			win.DestroyWindow(hwnd)
			return 0
		case idPromptCancel:
			if pr != nil {
				pr.done = true
			}
			win.DestroyWindow(hwnd)
			return 0
		}
	case win.WM_CLOSE:
		if pr != nil {
			pr.done = true
		}
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		if pr != nil {
			pr.done = true
		}
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}
