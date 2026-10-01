//go:build windows

package local

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// configure hides the console window: UNCLI never shows a terminal.
func configure(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

var (
	jobOnce sync.Once
	job     windows.Handle
)

// appJob is one job object for the whole app with kill-on-close, so every
// CLI process (and anything it spawns) dies with UNCLI, even on a crash.
func appJob() windows.Handle {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		job = h
	})
	return job
}

// attach puts the process in the app job and returns a function that kills
// its whole tree.
func attach(c *exec.Cmd) func() {
	pid := uint32(c.Process.Pid)
	if j := appJob(); j != 0 {
		if h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid); err == nil {
			_ = windows.AssignProcessToJobObject(j, h)
			windows.CloseHandle(h)
		}
	}
	return func() {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", itoa(pid))
		configure(kill)
		_ = kill.Run()
	}
}

func itoa(n uint32) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
