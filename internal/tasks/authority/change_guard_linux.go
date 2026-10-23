//go:build linux

package authority

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// notifyWatch registers every directory with inotify, and regular files up to
// a ceiling. The kernel counts watches per user (fs.inotify.max_user_watches),
// across every process the user runs, and a watch per receipt and request of
// a large store, in each of a few concurrent writers, spends the whole
// allowance (CAL-V0-364). Directories register as they are visited and regular
// files last (finish), the first inotifyFileCeiling of them with inotify. Any
// other regular file, and one whose watch the kernel refuses with ENOSPC, is
// tracked by the stat identity taken when it was visited and re-read by
// sweep, as Darwin tracks a file beyond its descriptor budget (CAL-V0-026,
// CAL-V0-219). A directory the kernel refuses still fails the registration:
// its events are what report entries created, removed or renamed at any time.
type notifyWatch struct {
	fd      int
	dirty   bool
	pending []pendingFile
	stats   []statWatch
}

// pendingFile is a regular file visited and not yet registered.
type pendingFile struct {
	path   string
	before os.FileInfo
	mask   uint32
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

// inotifyAddWatch, inotifyFileCeiling and sweepLstat are the seams of the tests
// that exhaust the watch allowance and count the stat reads of a sweep.
var (
	inotifyAddWatch     = syscall.InotifyAddWatch
	inotifyFileCeiling  = defaultInotifyFileCeiling
	sweepLstat          = os.Lstat
	inotifyWatchesLimit = "/proc/sys/fs/inotify/max_user_watches"
)

// defaultInotifyFileCeiling is the number of regular files one guard watches
// with inotify: a 32nd of the user's watch allowance, at most 4,096. It leaves
// the rest of the allowance to the directories of this guard and to the other
// processes that hold one, whatever the store's receipt count. An allowance
// that cannot be read counts as the 8,192 of older kernels.
func defaultInotifyFileCeiling() int {
	allowance := 8192
	if raw, err := os.ReadFile(inotifyWatchesLimit); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && n > 0 {
			allowance = n
		}
	}
	return min(4096, allowance/32)
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
	if before.Mode().IsRegular() {
		w.pending = append(w.pending, pendingFile{path: path, before: before, mask: mask})
		return before, nil
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
	return after, nil
}

// finish registers the regular files visited since the last call, after every
// directory. A file replaced since it was visited fails the registration as
// any other watch identity change does; a replacement in a watched directory
// is also reported by that directory's events.
func (w *notifyWatch) finish() error {
	pending := w.pending
	w.pending = nil
	ceiling := inotifyFileCeiling()
	for i, p := range pending {
		if i >= ceiling {
			if _, err := w.addStat(p.path, p.before); err != nil {
				return err
			}
			continue
		}
		if _, err := inotifyAddWatch(w.fd, p.path, p.mask); err != nil {
			if !errors.Is(err, syscall.ENOSPC) {
				return err
			}
			if _, err := w.addStat(p.path, p.before); err != nil {
				return err
			}
			continue
		}
		after, err := os.Lstat(p.path)
		if err != nil || !os.SameFile(p.before, after) {
			return watchMoved(p.path)
		}
	}
	return nil
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

// sweep re-reads the stat identity of every file the kernel refused to watch.
// A difference or a failed read marks the watch dirty for good, so a later
// poll reports it. With every file watched it has nothing to read.
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
	w.pending, w.stats = nil, nil
	return syscall.Close(w.fd)
}
