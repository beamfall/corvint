//go:build darwin

package authority

import (
	"errors"
	"os"
	"syscall"
)

type vnodeWatch struct {
	queue int
	files []*os.File
	dirty bool
}

func newChangeWatch() (changeWatch, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	return &vnodeWatch{queue: fd}, nil
}

func (w *vnodeWatch) add(path string, contents bool) (os.FileInfo, error) {
	before, err := watchable(path)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_EVTONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !os.SameFile(before, info) {
		f.Close()
		return nil, watchMoved(path)
	}
	flags := uint32(syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_REVOKE | syscall.NOTE_ATTRIB)
	if contents {
		flags |= syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_LINK
	}
	ev := syscall.Kevent_t{Ident: uint64(fd), Filter: syscall.EVFILT_VNODE, Flags: syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_CLEAR, Fflags: flags}
	if _, err = syscall.Kevent(w.queue, []syscall.Kevent_t{ev}, nil, nil); err != nil {
		f.Close()
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) {
		f.Close()
		return nil, watchMoved(path)
	}
	w.files = append(w.files, f)
	return info, nil
}

func (w *vnodeWatch) changed() (bool, error) {
	if w.dirty {
		return true, nil
	}
	var events [1]syscall.Kevent_t
	var n int
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		n, err = syscall.Kevent(w.queue, nil, events[:], &syscall.Timespec{})
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		return true, err
	}
	w.dirty = n != 0
	return w.dirty, nil
}

func (w *vnodeWatch) close() error {
	err := syscall.Close(w.queue)
	for _, f := range w.files {
		err = errors.Join(err, f.Close())
	}
	w.files = nil
	return err
}
