//go:build windows

package notify

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows: a tray icon owned by a hidden message-only window, and its
// balloons, which Windows 10 and 11 show as toasts (and keep in the
// notification centre) under the app's name. The icon is added with the
// first note and stays until Close, so the toast and a click on it keep
// working; clicking the toast or the icon calls onClick.

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW  = user32.NewProc("RegisterClassExW")
	pCreateWindowExW   = user32.NewProc("CreateWindowExW")
	pDefWindowProcW    = user32.NewProc("DefWindowProcW")
	pGetMessageW       = user32.NewProc("GetMessageW")
	pTranslateMessage  = user32.NewProc("TranslateMessage")
	pDispatchMessageW  = user32.NewProc("DispatchMessageW")
	pPostMessageW      = user32.NewProc("PostMessageW")
	pDestroyWindow     = user32.NewProc("DestroyWindow")
	pPostQuitMessage   = user32.NewProc("PostQuitMessage")
	pLoadIconW         = user32.NewProc("LoadIconW")
	pShellNotifyIconW  = shell32.NewProc("Shell_NotifyIconW")
	pExtractIconExW    = shell32.NewProc("ExtractIconExW")
	pGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	wndProcCallback    = syscall.NewCallback(wndProc)
	current            *trayNotifier // the one notifier the window procedure reports to
	currentMu          sync.Mutex
	errNoWindow        = errors.New("couldn't create the notification window")
	errNotifyIconAdded = errors.New("Windows refused the notification icon")
)

const (
	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nimSetVersion = 4

	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04
	nifInfo    = 0x10
	nifShowTip = 0x80

	niifUser             = 0x04
	niifLargeIcon        = 0x20
	niifRespectQuietTime = 0x80

	notifyIconVersion4 = 4

	wmDestroy           = 0x0002
	wmClose             = 0x0010
	wmLButtonUp         = 0x0202
	wmUser              = 0x0400
	wmApp               = 0x8000
	wmTray              = wmApp + 1 // the icon's callback message
	ninSelect           = wmUser + 0
	ninBalloonUserClick = wmUser + 5

	idiApplication = 32512
	iconID         = 1
)

var hwndMessage = ^uintptr(2) // HWND_MESSAGE, (HWND)-3

// notifyIconData is NOTIFYICONDATAW (Vista and later layout).
type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32 // union with uTimeout
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
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

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
	Private uint32
}

type trayNotifier struct {
	app     string
	onClick func(string)

	mu         sync.Mutex
	hwnd       uintptr
	small      uintptr // the tray icon
	large      uintptr // the toast's icon
	added      bool
	lastKey    string
	startErr   error
	startOnce  sync.Once
	windowDone chan struct{}
}

func newPlatform(app string, onClick func(string)) Notifier {
	t := &trayNotifier{app: app, onClick: onClick, windowDone: make(chan struct{})}
	currentMu.Lock()
	current = t
	currentMu.Unlock()
	return t
}

// start creates the hidden window on a thread of its own, which then pumps
// its messages until Close.
func (t *trayNotifier) start() error {
	t.startOnce.Do(func() {
		ready := make(chan error, 1)
		go t.loop(ready)
		t.startErr = <-ready
	})
	return t.startErr
}

func (t *trayNotifier) loop(ready chan<- error) {
	runtime.LockOSThread()
	defer close(t.windowDone)
	inst, _, _ := pGetModuleHandleW.Call(0)
	class, _ := windows.UTF16PtrFromString("UNCLINotifyWindow")
	wc := wndClassEx{LpfnWndProc: wndProcCallback, HInstance: inst, LpszClassName: class}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	_, _, _ = pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))) // fails harmlessly if already registered
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, inst, 0)
	if hwnd == 0 {
		ready <- errors.Join(errNoWindow, err)
		return
	}
	t.mu.Lock()
	t.hwnd = hwnd
	t.small, t.large = appIcons()
	t.mu.Unlock()
	ready <- nil
	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // WM_QUIT or an error
			return
		}
		_, _, _ = pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// appIcons takes the icons from UNCLI's own executable, falling back to
// the stock application icon.
func appIcons() (small, large uintptr) {
	if exe, err := os.Executable(); err == nil {
		if p, err := windows.UTF16PtrFromString(exe); err == nil {
			_, _, _ = pExtractIconExW.Call(uintptr(unsafe.Pointer(p)), 0,
				uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
		}
	}
	if small == 0 || large == 0 {
		stock, _, _ := pLoadIconW.Call(0, idiApplication)
		if small == 0 {
			small = stock
		}
		if large == 0 {
			large = stock
		}
	}
	return small, large
}

func (t *trayNotifier) data() notifyIconData {
	d := notifyIconData{HWnd: t.hwnd, UID: iconID}
	d.CbSize = uint32(unsafe.Sizeof(d))
	return d
}

// addLocked puts the icon in the tray. Caller holds t.mu.
func (t *trayNotifier) addLocked() error {
	d := t.data()
	d.UFlags = nifMessage | nifIcon | nifTip | nifShowTip
	d.UCallbackMessage = wmTray
	d.HIcon = t.small
	copyUTF16(d.SzTip[:], t.app)
	if r, _, _ := pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&d))); r == 0 {
		return errNotifyIconAdded
	}
	d.UVersion = notifyIconVersion4
	_, _, _ = pShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&d)))
	t.added = true
	return nil
}

func (t *trayNotifier) Show(n Note) error {
	if err := t.start(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.added {
		if err := t.addLocked(); err != nil {
			return err
		}
	}
	d := t.data()
	d.UFlags = nifInfo
	d.DwInfoFlags = niifUser | niifLargeIcon | niifRespectQuietTime
	d.HBalloonIcon = t.large
	copyUTF16(d.SzInfoTitle[:], clip(n.Title, len(d.SzInfoTitle)-1))
	copyUTF16(d.SzInfo[:], clip(n.Body, len(d.SzInfo)-1))
	t.lastKey = n.Key
	if r, _, _ := pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&d))); r == 0 {
		// Explorer restarted and dropped the icon: add it again once.
		t.added = false
		if err := t.addLocked(); err != nil {
			return err
		}
		_, _, _ = pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&d)))
	}
	return nil
}

func (t *trayNotifier) Close() {
	t.mu.Lock()
	hwnd := t.hwnd
	if t.added {
		d := t.data()
		_, _, _ = pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&d)))
		t.added = false
	}
	t.mu.Unlock()
	if hwnd != 0 {
		_, _, _ = pPostMessageW.Call(hwnd, wmClose, 0, 0)
		<-t.windowDone
	}
}

// clicked reports a click on the toast (with the note's key) or on the icon.
func (t *trayNotifier) clicked(onNote bool) {
	t.mu.Lock()
	key := ""
	if onNote {
		key = t.lastKey
	}
	t.mu.Unlock()
	if t.onClick != nil {
		go t.onClick(key)
	}
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmTray:
		currentMu.Lock()
		t := current
		currentMu.Unlock()
		if t != nil {
			switch lParam & 0xffff { // NOTIFYICON_VERSION_4: the event is in the low word
			case ninBalloonUserClick:
				t.clicked(true)
			case ninSelect, wmLButtonUp:
				t.clicked(false)
			}
		}
		return 0
	case wmClose:
		_, _, _ = pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		_, _, _ = pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// copyUTF16 writes s into a fixed, NUL-terminated buffer.
func copyUTF16(dst []uint16, s string) {
	u, _ := windows.UTF16FromString(strings.ReplaceAll(s, "\x00", ""))
	if len(u) > len(dst) {
		u = u[:len(dst)]
		u[len(u)-1] = 0
	}
	copy(dst, u)
}
