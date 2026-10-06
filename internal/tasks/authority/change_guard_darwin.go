//go:build darwin

package authority

import (
	"errors"
	"os"
	"sync/atomic"
	"syscall"
)

// vnodeWatch registers every directory with kqueue. A regular file takes a
// kqueue descriptor only while the process-wide budget allows; beyond it the
// file is tracked by the stat tuple taken at registration and re-read by
// sweep. Directory events still report entries created, removed or renamed
// at any time, without a sweep.
type vnodeWatch struct {
	queue    int
	budget   int64
	budgeted int64
	files    []*os.File
	stats    []statWatch
	dirty    bool
}

type statWatch struct {
	path string
	id   statIdentity
}

type statIdentity struct {
	dev, mode    int64
	ino          uint64
	size         int64
	mtime, ctime int64
}

// watchedFileDescriptors counts regular-file kqueue descriptors held by all
// live watches. vnodeFileBudget caps it at registration; tests override it
// and sweepLstat.
var (
	watchedFileDescriptors atomic.Int64
	vnodeFileBudget        = defaultVnodeFileBudget
	sweepLstat             = os.Lstat
)

// defaultVnodeFileBudget leaves half of the soft descriptor limit for
// directories, the reads that follow registration and concurrent work.
func defaultVnodeFileBudget() int64 {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0
	}
	return int64(min(limit.Cur/2, 1<<30))
}

func newChangeWatch() (changeWatch, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	return &vnodeWatch{queue: fd, budget: vnodeFileBudget()}, nil
}

func (w *vnodeWatch) add(path string, contents bool) (os.FileInfo, error) {
	before, err := watchable(path)
	if err != nil {
		return nil, err
	}
	if before.Mode().IsRegular() {
		if watchedFileDescriptors.Add(1) > w.budget {
			watchedFileDescriptors.Add(-1)
			return w.addStat(path, before)
		}
		w.budgeted++
		info, err := w.addVnode(path, before, contents)
		if err != nil {
			w.budgeted--
			watchedFileDescriptors.Add(-1)
		}
		return info, err
	}
	return w.addVnode(path, before, contents)
}

func (w *vnodeWatch) addVnode(path string, before os.FileInfo, contents bool) (os.FileInfo, error) {
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

func (w *vnodeWatch) addStat(path string, before os.FileInfo) (os.FileInfo, error) {
	id, ok := identityOf(before)
	if !ok {
		return nil, fsErr(path, "no stat identity")
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, watchMoved(path)
	}
	if now, ok := identityOf(after); !ok || now != id {
		return nil, watchMoved(path)
	}
	w.stats = append(w.stats, statWatch{path: path, id: id})
	return after, nil
}

func identityOf(info os.FileInfo) (statIdentity, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return statIdentity{}, false
	}
	return statIdentity{
		dev: int64(st.Dev), mode: int64(st.Mode), ino: st.Ino, size: st.Size,
		mtime: st.Mtimespec.Nano(), ctime: st.Ctimespec.Nano(),
	}, true
}

// poll reads one pending kqueue event. Its cost does not depend on the number
// of watched paths, so it is the only part of a check made under the writer
// lock (CAL-V0-026).
func (w *vnodeWatch) poll() (bool, error) {
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

// sweep re-reads every over-budget file's stat tuple. A difference or a failed
// read marks the watch dirty for good, so a later poll reports it.
func (w *vnodeWatch) sweep() bool {
	for i := 0; !w.dirty && i < len(w.stats); i++ {
		info, err := sweepLstat(w.stats[i].path)
		if err != nil {
			w.dirty = true
			break
		}
		id, ok := identityOf(info)
		w.dirty = !ok || id != w.stats[i].id
	}
	return w.dirty
}

func (w *vnodeWatch) close() error {
	err := syscall.Close(w.queue)
	for _, f := range w.files {
		err = errors.Join(err, f.Close())
	}
	w.files = nil
	w.stats = nil
	watchedFileDescriptors.Add(-w.budgeted)
	w.budgeted = 0
	return err
}
