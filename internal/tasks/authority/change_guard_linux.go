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
// ancestors, whatever its receipt count.
//
// Events reach the directory only through its own entry, so every regular
// file is also tracked by the stat identity taken when it was visited, link
// count included (PSR-V0-223). A link made to it after registration, or a
// write or mode change through such a link or through a descriptor opened
// through another link before registration, changes its link count, size or
// change time, which sweep re-reads for every file. A file with another link
// at registration, through which a write never passes the watched directory,
// and a file whose parent is not watched for contents, is uncovered: poll
// re-reads it as well, so a check under the writer lock sees a change made to
// it after the sweep. A directory the kernel refuses still fails the
// registration.
type notifyWatch struct {
	fd        int
	dirty     bool
	dirs      map[string]bool // directories watched with the content mask
	files     []statWatch     // every regular file, re-read by sweep
	uncovered []int           // indexes into files that poll re-reads too
}

type statWatch struct {
	path string
	id   statIdentity
}

type statIdentity struct {
	dev, ino     uint64
	mode         uint32
	nlink        uint64
	size         int64
	mtime, ctime int64
}

// inotifyAddWatch and sweepLstat are the seams of the tests that exhaust the
// watch allowance, count the watches of a guard and the stat reads of a sweep.
var (
	inotifyAddWatch = syscall.InotifyAddWatch
	sweepLstat      = syscall.Lstat
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
		// in the identity read here, and one through another link in the
		// identity sweep re-reads.
		st, ok := before.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fsErr(path, "no stat identity")
		}
		if st.Nlink == 1 && w.dirs[filepath.Dir(path)] {
			w.files = append(w.files, statWatch{path: path, id: identityOf(st)})
			return before, nil
		}
		return w.addStat(path, before)
	}
	if w.dirs[path] {
		// Already watched with the content mask; a second add without it
		// would replace the mask and silence the events of its entries.
		return before, nil
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
	st, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fsErr(path, "no stat identity")
	}
	id := identityOf(st)
	after, err := os.Lstat(path)
	if err != nil {
		return nil, watchMoved(path)
	}
	if now, ok := after.Sys().(*syscall.Stat_t); !ok || identityOf(now) != id {
		return nil, watchMoved(path)
	}
	w.uncovered = append(w.uncovered, len(w.files))
	w.files = append(w.files, statWatch{path: path, id: id})
	return after, nil
}

func identityOf(st *syscall.Stat_t) statIdentity {
	return statIdentity{
		dev: uint64(st.Dev), ino: st.Ino, mode: st.Mode, nlink: uint64(st.Nlink), size: st.Size,
		mtime: st.Mtim.Nano(), ctime: st.Ctim.Nano(),
	}
}

// changed re-reads the stat identity of one tracked file; a difference or a
// failed read marks the watch dirty for good, so a later poll reports it.
func (w *notifyWatch) changed(f *statWatch) bool {
	var st syscall.Stat_t
	w.dirty = sweepLstat(f.path, &st) != nil || identityOf(&st) != f.id
	return w.dirty
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
	if w.dirty = n != 0; w.dirty {
		return true, nil
	}
	// A write through another link of an uncovered file reaches no watched
	// directory at any time, so it is re-read here as well as by sweep.
	for _, i := range w.uncovered {
		if w.changed(&w.files[i]) {
			break
		}
	}
	return w.dirty, nil
}

// sweep re-reads the stat identity of every regular file, which reports a
// link made to it, or a write or mode change through a link or descriptor
// that bypasses its watched directory, made before the read (PSR-V0-223).
func (w *notifyWatch) sweep() bool {
	for i := 0; !w.dirty && i < len(w.files); i++ {
		w.changed(&w.files[i])
	}
	return w.dirty
}

func (w *notifyWatch) close() error {
	w.dirs, w.files, w.uncovered = nil, nil, nil
	return syscall.Close(w.fd)
}
