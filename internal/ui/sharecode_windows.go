//go:build windows

package ui

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

func setClipboardText(owner win.HWND, text string) error {
	u16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	nbytes := len(u16) * 2
	hMem := win.GlobalAlloc(win.GMEM_MOVEABLE, uintptr(nbytes))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}
	ptr := win.GlobalLock(hMem)
	if ptr == nil {
		win.GlobalFree(hMem)
		return fmt.Errorf("GlobalLock failed")
	}
	mem := unsafe.Slice((*uint16)(ptr), len(u16))
	copy(mem, u16)
	win.GlobalUnlock(hMem)

	if !win.OpenClipboard(owner) {
		win.GlobalFree(hMem)
		return fmt.Errorf("OpenClipboard failed")
	}
	defer win.CloseClipboard()
	if !win.EmptyClipboard() {
		win.GlobalFree(hMem)
		return fmt.Errorf("EmptyClipboard failed")
	}
	if win.SetClipboardData(win.CF_UNICODETEXT, win.HANDLE(hMem)) == 0 {
		win.GlobalFree(hMem)
		return fmt.Errorf("SetClipboardData failed")
	}
	return nil
}

func getClipboardText(owner win.HWND) (string, bool) {
	if !win.OpenClipboard(owner) {
		return "", false
	}
	defer win.CloseClipboard()
	h := win.GetClipboardData(win.CF_UNICODETEXT)
	if h == 0 {
		return "", false
	}
	ptr := win.GlobalLock(win.HGLOBAL(h))
	if ptr == nil {
		return "", false
	}
	defer win.GlobalUnlock(win.HGLOBAL(h))
	return syscall.UTF16ToString((*[1 << 20]uint16)(ptr)[:]), true
}

type shareDlgState struct {
	hwnd   win.HWND
	edit   win.HWND
	code   string
	copied bool
	ok     bool
	done   bool
}

var shareDlgByHWND = map[win.HWND]*shareDlgState{}

func centerOver(owner, hwnd win.HWND) {
	var orc, rc win.RECT
	win.GetWindowRect(owner, &orc)
	win.GetWindowRect(hwnd, &rc)
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	win.SetWindowPos(hwnd, 0,
		orc.Left+(orc.Right-orc.Left-w)/2,
		orc.Top+(orc.Bottom-orc.Top-h)/2,
		0, 0, win.SWP_NOSIZE|win.SWP_NOZORDER)
}

func registerShareDlgClass(instance win.HINSTANCE) {
	var wc win.WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(shareDlgWndProc)
	wc.HInstance = instance
	wc.HCursor = win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW))
	wc.HbrBackground = win.COLOR_BTNFACE + 1
	wc.LpszClassName = utf16Ptr(classShareDlg)
	if atom := win.RegisterClassEx(&wc); atom == 0 {
		_ = win.GetLastError()
	}
}

func runShareDlg(owner win.HWND, font win.HFONT, title string, w, h int32, st *shareDlgState, build func(hwnd win.HWND, add func(class, text string, style uint32, x, y, cw, ch, id int32) win.HWND)) bool {
	instance := win.GetModuleHandle(nil)
	registerShareDlgClass(instance)
	hwnd := win.CreateWindowEx(
		win.WS_EX_DLGMODALFRAME,
		utf16Ptr(classShareDlg),
		utf16Ptr(title),
		win.WS_POPUP|win.WS_CAPTION|win.WS_SYSMENU|win.WS_VISIBLE,
		0, 0, w, h,
		owner, 0, instance, nil,
	)
	if hwnd == 0 {
		return false
	}
	st.hwnd = hwnd
	shareDlgByHWND[hwnd] = st
	defer delete(shareDlgByHWND, hwnd)

	add := func(class, text string, style uint32, x, y, cw, ch, id int32) win.HWND {
		child := win.CreateWindowEx(0, utf16Ptr(class), utf16Ptr(text),
			win.WS_CHILD|win.WS_VISIBLE|style, x, y, cw, ch, hwnd, win.HMENU(id), instance, nil)
		if font != 0 {
			win.SendMessage(child, win.WM_SETFONT, uintptr(font), 1)
		}
		return child
	}
	build(hwnd, add)
	centerOver(owner, hwnd)
	win.EnableWindow(owner, false)

	var msg win.MSG
	for !st.done && win.GetMessage(&msg, 0, 0, 0) > 0 {
		if !win.IsDialogMessage(hwnd, &msg) {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		}
	}
	win.EnableWindow(owner, true)
	win.SetFocus(owner)
	return st.ok || st.copied
}

func showExportCode(owner win.HWND, font win.HFONT, code string) bool {
	st := &shareDlgState{code: code}
	return runShareDlg(owner, font, "Export Code", 480, 160, st, func(hwnd win.HWND, add func(class, text string, style uint32, x, y, cw, ch, id int32) win.HWND) {
		add("STATIC", "Share code (copy and send):", 0, 16, 12, 440, 20, 0)
		st.edit = add("EDIT", code, win.WS_BORDER|win.ES_AUTOHSCROLL|win.ES_READONLY, 16, 36, 440, 24, idPromptEdit)
		add("BUTTON", "Copy", win.BS_DEFPUSHBUTTON, 280, 80, 80, 28, idShareCopy)
		add("BUTTON", "Close", win.BS_PUSHBUTTON, 372, 80, 80, 28, idShareClose)
		win.SendMessage(st.edit, win.EM_SETSEL, 0, ^uintptr(0))
		win.SetFocus(st.edit)
	})
}

func promptShareCode(owner win.HWND, font win.HFONT) (string, bool) {
	st := &shareDlgState{}
	ok := runShareDlg(owner, font, "Import Code", 480, 220, st, func(hwnd win.HWND, add func(class, text string, style uint32, x, y, cw, ch, id int32) win.HWND) {
		add("STATIC", "Paste a vutils share code:", 0, 16, 12, 440, 20, 0)
		st.edit = win.CreateWindowEx(win.WS_EX_CLIENTEDGE, utf16Ptr("EDIT"), utf16Ptr(""),
			win.WS_CHILD|win.WS_VISIBLE|win.WS_VSCROLL|win.ES_MULTILINE|win.ES_AUTOVSCROLL|win.ES_WANTRETURN,
			16, 36, 440, 80, hwnd, win.HMENU(idPromptEdit), win.GetModuleHandle(nil), nil)
		if font != 0 {
			win.SendMessage(st.edit, win.WM_SETFONT, uintptr(font), 1)
		}
		if clip, ok := getClipboardText(owner); ok && strings.Contains(clip, "vutils;") {
			setWindowText(st.edit, strings.TrimSpace(clip))
		}
		add("BUTTON", "Import", win.BS_DEFPUSHBUTTON, 280, 132, 80, 28, idPromptOK)
		add("BUTTON", "Cancel", win.BS_PUSHBUTTON, 372, 132, 80, 28, idPromptCancel)
		win.SetFocus(st.edit)
	})
	if !ok {
		return "", false
	}
	return st.code, true
}

func shareDlgWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	st := shareDlgByHWND[hwnd]
	switch msg {
	case win.WM_COMMAND:
		id := int(win.LOWORD(uint32(wParam)))
		switch id {
		case idShareCopy:
			if st != nil {
				text := getWindowText(st.edit)
				if text == "" {
					text = st.code
				}
				if err := setClipboardText(hwnd, text); err != nil {
					win.MessageBox(hwnd, utf16Ptr(err.Error()), utf16Ptr("Copy"), win.MB_OK|win.MB_ICONWARNING)
					return 0
				}
				st.copied = true
				st.done = true
				win.DestroyWindow(hwnd)
			}
			return 0
		case idShareClose:
			if st != nil {
				st.done = true
			}
			win.DestroyWindow(hwnd)
			return 0
		case idPromptOK:
			if st != nil {
				st.code = strings.TrimSpace(getWindowText(st.edit))
				if st.code == "" {
					return 0
				}
				st.ok = true
				st.done = true
			}
			win.DestroyWindow(hwnd)
			return 0
		case idPromptCancel:
			if st != nil {
				st.done = true
			}
			win.DestroyWindow(hwnd)
			return 0
		}
	case win.WM_CLOSE:
		if st != nil {
			st.done = true
		}
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		if st != nil {
			st.done = true
		}
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}
