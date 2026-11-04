//go:build !windows

package datadir

import (
	"errors"
	"os"
	"syscall"
)

// lock holds an exclusive advisory lock on the lock file; the system drops
// it when the process ends.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrInUse
		}
		return nil, err
	}
	return func() { f.Close() }, nil
}
