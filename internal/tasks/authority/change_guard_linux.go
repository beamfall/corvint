//go:build linux

package authority

import (
	"errors"
	"os"
	"syscall"
)

type notifyWatch struct {
	fd    int
	dirty bool
}

func newChangeWatch() (changeWatch, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &notifyWatch{fd: fd}, nil
}

func (w *notifyWatch) add(path string, contents bool) (os.FileInfo, error) {
	before, err := watchable(path)
	if err != nil {
		return nil, err
	}
	mask := uint32(syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_ATTRIB | syscall.IN_DONT_FOLLOW)
	if contents {
		mask |= syscall.IN_MODIFY | syscall.IN_CLOSE_WRITE | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO
	}
	if _, err = syscall.InotifyAddWatch(w.fd, path, mask); err != nil {
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return nil, watchMoved(path)
	}
	return after, nil
}

func (w *notifyWatch) poll() (bool, error) {
	if w.dirty {
		return true, nil
	}
	var buf [4096]byte
	var n int
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		n, err = syscall.Read(w.fd, buf[:])
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	// Any event, including overflow or an ignored/revoked watch, invalidates.
	w.dirty = n != 0
	return w.dirty, nil
}

// sweep has nothing to re-read: inotify watches every path without a budget.
func (w *notifyWatch) sweep() bool { return w.dirty }

func (w *notifyWatch) close() error { return syscall.Close(w.fd) }
