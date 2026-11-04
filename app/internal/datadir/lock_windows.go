package datadir

import (
	"errors"

	"golang.org/x/sys/windows"
)

// lock opens the lock file with no sharing: while it's open, no other
// process (or open in this one) can open it. Windows closes it when the
// process ends, however it ends.
func lock(path string) (func(), error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return nil, ErrInUse
	}
	if err != nil {
		return nil, err
	}
	return func() { _ = windows.CloseHandle(h) }, nil
}
