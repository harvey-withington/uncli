package clipfiles

import (
	"errors"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cfHDrop = 15 // CF_HDROP: a list of files, as Explorer copies them

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	shell32                    = windows.NewLazySystemDLL("shell32.dll")
	procOpenClipboard          = user32.NewProc("OpenClipboard")
	procCloseClipboard         = user32.NewProc("CloseClipboard")
	procIsClipboardFormatAvail = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardData       = user32.NewProc("GetClipboardData")
	procDragQueryFileW         = shell32.NewProc("DragQueryFileW")
)

func paths() ([]string, error) {
	if r, _, _ := procIsClipboardFormatAvail.Call(cfHDrop); r == 0 {
		return nil, nil
	}
	// The clipboard belongs to one thread at a time, and another app may
	// hold it for a moment.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	opened := false
	for range 5 {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return nil, errors.New("the clipboard is busy; try pasting again")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(cfHDrop)
	if h == 0 {
		return nil, nil
	}
	count, _, _ := procDragQueryFileW.Call(h, 0xFFFFFFFF, 0, 0)
	out := make([]string, 0, count)
	for i := uintptr(0); i < count; i++ {
		n, _, _ := procDragQueryFileW.Call(h, i, 0, 0)
		if n == 0 {
			continue
		}
		buf := make([]uint16, n+1)
		procDragQueryFileW.Call(h, i, uintptr(unsafe.Pointer(&buf[0])), n+1)
		out = append(out, windows.UTF16ToString(buf))
	}
	return out, nil
}
