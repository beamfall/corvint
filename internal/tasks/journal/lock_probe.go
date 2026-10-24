package journal

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// writerLockFileName is authority.LockFileName, the writer lock every native
// writer session holds (§5.2). journal cannot import authority (authority's
// tests import journal); TestCTSV0026_ProbeNamesTheWriterLock pins the two.
const writerLockFileName = "taskman.lock"

// writerLockFree reports whether this instant proves that no native writer
// holds `<commonDir>/taskman.lock` (CTS-V0-026). Every writer session holds
// that lock while it owns a staging slot, so a free lock proves a
// descriptor-less slot a killed writer's orphan.
//
// The probe never mutates: it opens the existing lock file read-only, without
// O_CREAT and without following a symlink, takes `flock(LOCK_SH|LOCK_NB)`, and
// releases it at once. Only a granted shared lock on the same regular file the
// name resolves to before and after the open answers true. An absent lock
// file, a symlink, a replaced file, contention, an unsupported filesystem or
// platform, or any error answers false, which proves nothing.
func writerLockFree(commonDir string) bool {
	if !safeopen.Supported || !filepath.IsAbs(commonDir) || filepath.Clean(commonDir) != commonDir {
		return false
	}
	root, err := safeopen.Root(commonDir)
	if err != nil {
		return false
	}
	defer root.Close()
	pre, err := root.Lstat(writerLockFileName)
	if err != nil || !pre.Mode().IsRegular() {
		return false
	}
	f, err := safeopen.InRoot(root, writerLockFileName, os.O_RDONLY, 0, false)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || !os.SameFile(pre, st) {
		return false
	}
	if !sharedLockFree(f) {
		return false
	}
	post, err := root.Lstat(writerLockFileName)
	return err == nil && os.SameFile(post, st)
}
