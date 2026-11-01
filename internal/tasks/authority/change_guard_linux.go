//go:build linux

package authority

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/wire"
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
//
// An ancestor is watched for itself only: the kernel reports IN_ATTRIB for
// every entry of a watched directory, so a mode or time change on an entry
// unrelated to the store, including any direct entry of the system temporary
// directory that another process makes, would otherwise invalidate the
// observation (PSR-V0-274). poll keeps an ancestor's nameless events and
// those naming its entry on the path to an observed root, and drops the rest.
type notifyWatch struct {
	fd        int
	dirty     bool
	dirs      map[string]bool          // directories watched with the content mask
	ancestors map[int32]*ancestorWatch // ancestor watch descriptors and their entries on the path
	files     []statWatch              // every regular file, re-read by sweep
	uncovered []int                    // indexes into files that poll re-reads too
	byDir     []sweepDir               // files grouped by directory, built by the first sweep
}

// ancestorWatch names the entries of an ancestor directory on the path to an
// observed root; an event naming any other entry is not the store's.
type ancestorWatch struct {
	path  string
	names map[string]bool
}

// pollReads bounds the event buffers one poll reads. A poll that reaches the
// bound without draining the queue reports a change, because an event it has
// not decoded may name the store; only a drained queue is clean (PSR-V0-274).
const pollReads = 64

type statWatch struct {
	path string
	id   statIdentity
}

// sweepDir is one directory's tracked files, which sweep re-reads with
// fstatat relative to one descriptor of the directory instead of resolving
// each full path (V1-2530): names are NUL-terminated for the raw call.
type sweepDir struct {
	path  string
	files []int    // indexes into notifyWatch.files
	names [][]byte // their base names, NUL-terminated
}

type statIdentity struct {
	dev, ino     uint64
	mode         uint32
	nlink        uint64
	size         int64
	mtime, ctime int64
}

// inotifyAddWatch, sweepLstat and sweepStatAt are the seams of the tests that
// exhaust the watch allowance, count the watches of a guard and the stat reads
// of a sweep. sweepLstat re-reads one uncovered file in poll; sweepStatAt
// re-reads a file relative to its directory's descriptor in sweep.
var (
	inotifyAddWatch = syscall.InotifyAddWatch
	sweepLstat      = syscall.Lstat
	sweepStatAt     = fstatat
)

// atSymlinkNoFollow is AT_SYMLINK_NOFOLLOW: fstatat reads a final symlink
// itself, as lstat does.
const atSymlinkNoFollow = 0x100

func newChangeWatch() (changeWatch, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &notifyWatch{fd: fd, dirs: map[string]bool{}, ancestors: map[int32]*ancestorWatch{}}, nil
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
	wd, err := inotifyAddWatch(w.fd, path, mask)
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			return nil, wire.Errorf(wire.CodeResourceExhausted, path, "inotify watch limit reached (fs.inotify.max_user_watches): %v", err)
		}
		return nil, err
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		return nil, watchMoved(path)
	}
	if contents {
		// A directory first added as an ancestor now carries the content
		// mask under the same descriptor: every event of it counts.
		delete(w.ancestors, int32(wd))
		w.dirs[path] = true
	} else {
		w.ancestors[int32(wd)] = &ancestorWatch{path: path, names: map[string]bool{}}
	}
	return after, nil
}

// allow names an entry of an ancestor directory on the path to an observed
// root, whose events poll keeps. A directory watched for its contents keeps
// every event and needs no name.
func (w *notifyWatch) allow(path, child string) {
	for _, a := range w.ancestors {
		if a.path == path {
			a.names[child] = true
		}
	}
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
	drained := false
	for reads := 0; reads < pollReads; reads++ {
		var n int
		var err error
		for attempt := 0; attempt < 8; attempt++ {
			n, err = syscall.Read(w.fd, buf[:])
			if !errors.Is(err, syscall.EINTR) {
				break
			}
		}
		if errors.Is(err, syscall.EAGAIN) || err == nil && n == 0 {
			drained = true
			break
		}
		if err != nil {
			return true, err
		}
		if w.dirty = w.invalidated(buf[:n]); w.dirty {
			return true, nil
		}
	}
	if !drained {
		w.dirty = true
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

// invalidated reports whether one buffer of inotify events holds an event of
// the store: any event of a directory watched for its contents, an overflow,
// an ignored or unmounted watch, an ancestor's own event, or an event naming
// an ancestor's entry on the path to an observed root (PSR-V0-274). An event
// naming another entry of an ancestor is not the store's. A buffer that does
// not hold whole events invalidates rather than being guessed at.
func (w *notifyWatch) invalidated(buf []byte) bool {
	for len(buf) > 0 {
		if len(buf) < syscall.SizeofInotifyEvent {
			return true
		}
		wd := int32(binary.NativeEndian.Uint32(buf[0:4]))
		mask := binary.NativeEndian.Uint32(buf[4:8])
		size := syscall.SizeofInotifyEvent + int(binary.NativeEndian.Uint32(buf[12:16]))
		if size > len(buf) {
			return true
		}
		if wd < 0 || mask&(syscall.IN_Q_OVERFLOW|syscall.IN_IGNORED|syscall.IN_UNMOUNT) != 0 {
			return true
		}
		a := w.ancestors[wd]
		if a == nil || size == syscall.SizeofInotifyEvent {
			return true
		}
		name, _, _ := bytes.Cut(buf[syscall.SizeofInotifyEvent:size], []byte{0})
		if a.names[string(name)] {
			return true
		}
		buf = buf[size:]
	}
	return false
}

// sweep re-reads the stat identity of every regular file, which reports a
// link made to it, or a write or mode change through a link or descriptor
// that bypasses its watched directory, made before the read (PSR-V0-223).
// sweep re-reads every tracked file's stat identity (PSR-V0-223). Each
// directory is opened once and its files are read with fstatat relative to
// it (V1-2530), which resolves only the base name: the identity fields and
// the symlink handling are lstat's, and the directory is resolved by its path
// at the sweep as lstat would resolve it, so a replaced directory or a file
// that cannot be read marks the watch dirty exactly as before. A directory
// that cannot be opened is swept by full path.
func (w *notifyWatch) sweep() bool {
	if w.byDir == nil {
		w.groupByDir()
	}
	var st syscall.Stat_t
	for d := 0; !w.dirty && d < len(w.byDir); d++ {
		dir := &w.byDir[d]
		fd, err := openSweepDir(dir.path)
		if err != nil {
			// A directory that cannot be opened (no descriptor left, or
			// no search permission) is read by full path as before, so the
			// result never depends on the descriptor.
			for _, i := range dir.files {
				if w.changed(&w.files[i]) {
					break
				}
			}
			continue
		}
		for k, i := range dir.files {
			if sweepStatAt(fd, dir.names[k], &st) != nil || identityOf(&st) != w.files[i].id {
				w.dirty = true
				break
			}
		}
		_ = syscall.Close(fd)
	}
	return w.dirty
}

// groupByDir groups the tracked files by their directory, in first-seen
// order, once per watch.
func (w *notifyWatch) groupByDir() {
	at := map[string]int{}
	w.byDir = []sweepDir{}
	for i, f := range w.files {
		dir, name := filepath.Split(f.path)
		d, ok := at[dir]
		if !ok {
			d = len(w.byDir)
			at[dir] = d
			w.byDir = append(w.byDir, sweepDir{path: dir})
		}
		w.byDir[d].files = append(w.byDir[d].files, i)
		w.byDir[d].names = append(w.byDir[d].names, append([]byte(name), 0))
	}
}

// openSweepDir opens a directory only to name it to fstatat.
func openSweepDir(path string) (int, error) {
	for {
		fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
		if !errors.Is(err, syscall.EINTR) {
			return fd, err
		}
	}
}

func (w *notifyWatch) close() error {
	w.dirs, w.ancestors, w.files, w.uncovered, w.byDir = nil, nil, nil, nil, nil
	return syscall.Close(w.fd)
}
