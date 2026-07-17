//go:build windows

package overlay

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"github.com/mewisme/vutils/internal/config"
)

const (
	className     = "VUtilsOverlay"
	wsExBase      = win.WS_EX_LAYERED | win.WS_EX_TOPMOST | win.WS_EX_TOOLWINDOW | win.WS_EX_NOACTIVATE | win.WS_EX_TRANSPARENT
	swpNoActivate = win.SWP_NOACTIVATE | win.SWP_SHOWWINDOW
	ulwAlpha      = 0x00000002
	biRGB         = 0
	dibRGBColors  = 0
)

var (
	libUser32               = syscall.NewLazyDLL("user32.dll")
	libGdi32                = syscall.NewLazyDLL("gdi32.dll")
	procUpdateLayeredWindow = libUser32.NewProc("UpdateLayeredWindow")
	procCreateDIBSection    = libGdi32.NewProc("CreateDIBSection")

	hosts sync.Map // win.HWND -> *nativeHost
)

type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type sizeStruct struct {
	Cx, Cy int32
}

type pointStruct struct {
	X, Y int32
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

type cmdKind int

const (
	cmdShow cmdKind = iota
	cmdHide
	cmdRedraw
	cmdDestroy
)

type command struct {
	kind cmdKind
	done chan error
}

type nativeHost struct {
	state *State

	mu      sync.Mutex
	started bool
	hwnd    win.HWND
	cmdCh   chan command
}

func newNativeHost(state *State) *nativeHost {
	return &nativeHost{
		state: state,
		cmdCh: make(chan command, 8),
	}
}

func (h *nativeHost) ensureLoop() error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	ready := make(chan error, 1)
	go h.loop(ready)
	// Must unlock before waiting — loop locks mu to store hwnd.
	h.mu.Unlock()

	if err := <-ready; err != nil {
		return err
	}

	h.mu.Lock()
	h.started = true
	h.mu.Unlock()
	return nil
}

func (h *nativeHost) send(kind cmdKind) error {
	if err := h.ensureLoop(); err != nil {
		return err
	}
	done := make(chan error, 1)
	h.cmdCh <- command{kind: kind, done: done}
	return <-done
}

func (h *nativeHost) show() error {
	if err := h.send(cmdShow); err != nil {
		return err
	}
	return h.send(cmdRedraw)
}

func (h *nativeHost) hide() error {
	h.mu.Lock()
	started := h.started
	h.mu.Unlock()
	if !started {
		return nil
	}
	return h.send(cmdHide)
}

func (h *nativeHost) destroy() error {
	h.mu.Lock()
	started := h.started
	h.mu.Unlock()
	if !started {
		return nil
	}
	err := h.send(cmdDestroy)
	h.mu.Lock()
	h.started = false
	h.hwnd = 0
	h.mu.Unlock()
	return err
}

func (h *nativeHost) redraw() {
	h.mu.Lock()
	started := h.started
	h.mu.Unlock()
	if !started {
		return
	}
	_ = h.send(cmdRedraw)
}

func (h *nativeHost) loop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance := win.GetModuleHandle(nil)
	if err := registerClass(instance); err != nil {
		ready <- err
		return
	}

	vx := win.GetSystemMetrics(win.SM_XVIRTUALSCREEN)
	vy := win.GetSystemMetrics(win.SM_YVIRTUALSCREEN)
	vw := win.GetSystemMetrics(win.SM_CXVIRTUALSCREEN)
	vh := win.GetSystemMetrics(win.SM_CYVIRTUALSCREEN)
	if vw <= 0 || vh <= 0 {
		ready <- fmt.Errorf("invalid virtual screen size %dx%d", vw, vh)
		return
	}

	hwnd := win.CreateWindowEx(
		wsExBase,
		syscall.StringToUTF16Ptr(className),
		syscall.StringToUTF16Ptr("Overlay"),
		win.WS_POPUP,
		vx, vy, vw, vh,
		0, 0, instance, nil,
	)
	if hwnd == 0 {
		ready <- fmt.Errorf("CreateWindowEx failed: %d", win.GetLastError())
		return
	}

	hosts.Store(hwnd, h)
	h.mu.Lock()
	h.hwnd = hwnd
	h.mu.Unlock()

	// Initial empty layered buffer so window is valid before first show.
	_ = updateLayeredContent(hwnd, h.state.Get(), vx, vy, vw, vh)
	ready <- nil

	var msg win.MSG
	for {
		select {
		case c := <-h.cmdCh:
			switch c.kind {
			case cmdShow:
				win.SetWindowPos(hwnd, win.HWND_TOPMOST, vx, vy, vw, vh, swpNoActivate)
				win.ShowWindow(hwnd, win.SW_SHOWNOACTIVATE)
				err := updateLayeredContent(hwnd, h.state.Get(), vx, vy, vw, vh)
				c.done <- err
			case cmdHide:
				win.ShowWindow(hwnd, win.SW_HIDE)
				c.done <- nil
			case cmdRedraw:
				err := updateLayeredContent(hwnd, h.state.Get(), vx, vy, vw, vh)
				if win.IsWindowVisible(hwnd) {
					win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
						win.SWP_NOACTIVATE|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW)
				}
				c.done <- err
			case cmdDestroy:
				hosts.Delete(hwnd)
				win.DestroyWindow(hwnd)
				c.done <- nil
				return
			}
		default:
			if win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
				if msg.Message == win.WM_QUIT {
					return
				}
				win.TranslateMessage(&msg)
				win.DispatchMessage(&msg)
			} else {
				time.Sleep(16 * time.Millisecond)
			}
		}
	}
}

func registerClass(instance win.HINSTANCE) error {
	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(overlayWndProc)
	wc.HInstance = instance
	wc.HCursor = win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
	wc.HbrBackground = 0
	wc.LpszClassName = syscall.StringToUTF16Ptr(className)

	if atom := win.RegisterClassEx(&wc); atom == 0 {
		if errno := win.GetLastError(); errno != 1410 { // ERROR_CLASS_ALREADY_EXISTS
			return fmt.Errorf("RegisterClassEx failed: %d", errno)
		}
	}
	return nil
}

func overlayWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_NCHITTEST:
		return ^uintptr(0) // HTTRANSPARENT
	case win.WM_DESTROY:
		hosts.Delete(hwnd)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func updateLayeredContent(hwnd win.HWND, cfg config.OverlayConfig, vx, vy, vw, vh int32) error {
	screenDC := win.GetDC(0)
	if screenDC == 0 {
		return fmt.Errorf("GetDC failed")
	}
	defer win.ReleaseDC(0, screenDC)

	memDC := win.CreateCompatibleDC(screenDC)
	if memDC == 0 {
		return fmt.Errorf("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memDC)

	var bi bitmapInfo
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = vw
	bi.BmiHeader.BiHeight = -vh // top-down DIB
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = biRGB

	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(
		uintptr(memDC),
		uintptr(unsafe.Pointer(&bi)),
		uintptr(dibRGBColors),
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if bmp == 0 || bits == nil {
		return fmt.Errorf("CreateDIBSection failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bmp))

	old := win.SelectObject(memDC, win.HGDIOBJ(bmp))
	defer win.SelectObject(memDC, old)

	// Clear to fully transparent.
	pixelCount := int(vw * vh)
	pix := unsafe.Slice((*uint32)(bits), pixelCount)
	for i := range pix {
		pix[i] = 0
	}

	if cfg.Enabled {
		drawGuides(pix, int(vw), int(vh), int(vx), int(vy), cfg)
	}

	alpha := byte(cfg.Opacity * 255)
	if cfg.Enabled && alpha == 0 && cfg.Opacity > 0 {
		alpha = 1
	}
	if !cfg.Enabled {
		alpha = 0
	}

	var srcPos pointStruct
	var winPos = pointStruct{X: vx, Y: vy}
	var size = sizeStruct{Cx: vw, Cy: vh}
	blend := blendFunction{
		BlendOp:             0, // AC_SRC_OVER
		SourceConstantAlpha: alpha,
		AlphaFormat:         1, // AC_SRC_ALPHA
	}

	ret, _, err := procUpdateLayeredWindow.Call(
		uintptr(hwnd),
		uintptr(screenDC),
		uintptr(unsafe.Pointer(&winPos)),
		uintptr(unsafe.Pointer(&size)),
		uintptr(memDC),
		uintptr(unsafe.Pointer(&srcPos)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		uintptr(ulwAlpha),
	)
	if ret == 0 {
		return fmt.Errorf("UpdateLayeredWindow failed: %v", err)
	}
	return nil
}
