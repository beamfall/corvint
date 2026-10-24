//go:build linux

package authority

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// notifyWatch registers directories with inotify and no regular file
// (PSR-V0-222). The kernel counts watches per user
// (fs.inotify.max_user_watches), across every process the user runs, and a
// watch per receipt of a large store, in each of a few concurrent guards,
// spent the whole allowance (CAL-V0-364, V1-2482). A directory watched with
// the content mask reports IN_MODIFY, IN_ATTRIB and IN_CLOSE_WRITE for every
// entry in it as well as the entries created, removed or renamed, so a
// regular file whose only link is in a watched directory needs no watch of
// its own: the guard's watch count is the store's directories and their
// ancestors, whatever its receipt count. A file with another link, through
// which an actor could write without the directory seeing it, and a file
// whose parent is not watched for contents, is tracked by the stat identity
// taken when it was visited and re-read by sweep, as Darwin tracks a file
// beyond its descriptor budget (CAL-V0-026, CAL-V0-219). A directory the
// kernel refuses still fails the registration.
type notifyWatch struct {
	fd    int
	dirty bool
	dirs  map[string]bool // directories watched with the content mask
	stats []statWatch
}

type statWatch struct {
	path string
	id   statIdentity
}

type statIdentity struct {
	dev, ino     uint64
	mode         uint32
	size         int64
	mtime, ctime int64
}

// inotifyAddWatch and sweepLstat are the seams of the tests that exhaust the
// watch allowance, count the watches of a guard and the stat reads of a sweep.
var (
	inotifyAddWatch = syscall.InotifyAddWatch
	sweepLstat      = os.Lstat
)

func newChangeWatch() (changeWatch, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &notifyWatch{fd: fd, dirs: map[string]bool{}}, nil
}

func (w *notifyWatch) add(path string, contents bool) (os.FileInfo, error) {
	before, err := watchable(path)
	if err != nil {
		return nil, err
	}
	if before.Mode().IsRegular() {
		// The parent was watched before it was listed, so a change to this
		// entry after the listing is already reported; one before it shows
		// in the identity read here.
		if st, ok := before.Sys().(*syscall.Stat_t); ok && st.Nlink == 1 && w.dirs[filepath.Dir(path)] {
			return before, nil
		}
		return w.addStat(path, before)
	}
	mask := uint32(syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF | syscall.IN_ATTRIB | syscall.IN_DONT_FOLLOW)
	if contents {
		mask |= syscall.IN_MODIFY | syscall.IN_CLOSE_WRITE | syscall.IN_CREATE | syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO
	}
	if _, err = inotifyAddWatch(w.fd, path, mask); err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return nil, fsErr(path, "inotify watch limit reached (fs.inotify.max_user_watches): %v", err)
		}
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return nil, watchMoved(path)
	}
	if contents {
		w.dirs[path] = true
	}
	return after, nil
}

func (w *notifyWatch) addStat(path string, before os.FileInfo) (os.FileInfo, error) {
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
		dev: uint64(st.Dev), ino: st.Ino, mode: st.Mode, size: st.Size,
		mtime: st.Mtim.Nano(), ctime: st.Ctim.Nano(),
	}, true
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

// sweep re-reads the stat identity of every file no directory event covers.
// A difference or a failed read marks the watch dirty for good, so a later
// poll reports it. With every file singly linked in a watched directory it
// has nothing to read.
func (w *notifyWatch) sweep() bool {
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

func (w *notifyWatch) close() error {
	w.dirs, w.stats = nil, nil
	return syscall.Close(w.fd)
}
