//go:build darwin

package authority

import (
	"errors"
	"os"
	"syscall"
)

// Darwin reports a flock(2) owner through fcntl(F_GETLK) (l_pid -1). The
// query tests for a conflicting lock and acquires nothing.
type getlkView struct{}

func loadPreparationLockView() (preparationLockView, error) { return getlkView{}, nil }

func (getlkView) method() string { return "fcntl-getlk" }

func (getlkView) held(f *os.File, _ os.FileInfo) (bool, error) {
	lk := syscall.Flock_t{Type: syscall.F_WRLCK, Whence: 0, Start: 0, Len: 0}
	err := withFD(f, func(fd int) error {
		for {
			e := syscall.FcntlFlock(uintptr(fd), syscall.F_GETLK, &lk)
			if e != syscall.EINTR {
				return e
			}
		}
	})
	if err != nil {
		return false, errors.New("lock query failed")
	}
	return lk.Type != syscall.F_UNLCK, nil
}
