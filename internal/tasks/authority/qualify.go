package authority

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// Qualification is the result of one §5.1 probe of a directory. A field is
// true only when the OS actually performed the operation; a false field on
// an error result says which step did not happen.
type Qualification struct {
	// Root is the directory that was probed.
	Root string

	// Filesystem is what fstatfs reported for Root.
	Filesystem Filesystem

	// Allowed is Classify(Filesystem.Platform, Filesystem.Type).
	Allowed bool

	// FullSync: the probe file was written and fully synced.
	FullSync bool

	// Flock: `flock(LOCK_EX|LOCK_NB)` then unlock succeeded on the probe.
	Flock bool

	// DirSync: the directory itself was synced after the probe was removed.
	DirSync bool

	// ProbeName is the unique temporary file the probe created under Root
	// and removed; empty when no file was created.
	ProbeName string
}

// Qualify performs the §5.1 filesystem qualification (TM-V0-010) of one
// directory by observation: no symlink in the path, a local filesystem type
// in the allowlist as reported by fstatfs, a full sync of a freshly created
// file, a working exclusive flock on it, and a directory sync. The
// allowlist is checked before anything is written, so nothing is ever
// created on a refused (network, FUSE, unknown) mount.
//
// The only write is one unique, O_EXCL-created temporary file directly
// under root named `.taskman-qualify-<pid>-<random>.tmp`; it never
// overwrites anything and is removed on every exit. A removal failure is
// reported, never hidden.
//
// Root is chosen by the caller: the Git common dir of the repository
// authority for a mutating command, or a fixture root under test. Reads
// never call Qualify. A successful qualification proves properties of the
// mount, not of the caller (see Prerequisites).
func Qualify(root string) (*Qualification, error) {
	q := &Qualification{Root: root}
	if !supportedPlatform {
		return q, fsErr(root, "unsupported platform")
	}
	if !filepath.IsAbs(root) {
		return q, fsErr(root, "qualification root must be an absolute path")
	}
	if err := intent.CheckNoSymlink(root); err != nil {
		return q, err
	}
	fi, err := os.Lstat(root)
	if err != nil {
		return q, fsErr(root, "cannot stat: %v", err)
	}
	if !fi.IsDir() {
		return q, fsErr(root, "qualification root is not a directory")
	}
	r, err := openRoot(root)
	if err != nil {
		return q, fsErr(root, "cannot open the root: %v", err)
	}
	defer r.Close()
	dir, err := r.Open(".")
	if err != nil {
		return q, fsErr(root, "cannot open the directory: %v", err)
	}
	defer dir.Close()
	st, err := dir.Stat()
	if err != nil {
		return q, fsErr(root, "cannot stat the opened directory: %v", err)
	}
	if !st.IsDir() || !os.SameFile(fi, st) {
		return q, fsErr(root, "directory replaced between stat and open (identity drift)")
	}

	fs, err := observeFilesystem(dir)
	q.Filesystem = fs
	if err != nil {
		return q, fsErr(root, "cannot observe the filesystem type: %v", err)
	}
	q.Allowed = Classify(fs.Platform, fs.Type)
	if !q.Allowed {
		return q, fsErr(root, "filesystem %q on %s is not in the §5.1 allowlist (Darwin apfs, hfs; Linux ext4, xfs, btrfs, tmpfs); nothing was written", fs.Type, fs.Platform)
	}
	if !fs.Local {
		return q, fsErr(root, "filesystem %q on %s was not reported local by the OS; nothing was written", fs.Type, fs.Platform)
	}

	name, err := uniqueName(probePrefix, ".tmp")
	if err != nil {
		return q, err
	}
	f, err := r.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return q, fsErr(filepath.Join(root, name), "cannot create the probe file exclusively: %v", err)
	}
	q.ProbeName = name
	probeErr := probe(q, f, filepath.Join(root, name))
	closeErr := f.Close()
	var cleanup error
	if rmErr := r.Remove(name); rmErr != nil {
		cleanup = fsErr(filepath.Join(root, name), "probe file was not removed: %v", rmErr)
	}
	if probeErr != nil {
		return q, withCleanup(probeErr, cleanup)
	}
	if closeErr != nil {
		return q, withCleanup(fsErr(filepath.Join(root, name), "closing the probe file failed: %v", closeErr), cleanup)
	}
	if cleanup != nil {
		return q, cleanup
	}
	if err := syncDirectory(dir); err != nil {
		if isFullSyncUnsupported(err) {
			return q, fsErr(root, "directory sync is unsupported here: %v", err)
		}
		return q, fsErr(root, "directory sync failed: %v", err)
	}
	q.DirSync = true
	return q, nil
}

// probe writes, fully syncs, locks and unlocks the open probe file.
func probe(q *Qualification, f *os.File, where string) error {
	data := []byte("taskman-qualification-probe\n")
	n, err := f.Write(data)
	if err != nil {
		return fsErr(where, "probe write failed after %d of %d bytes: %v", n, len(data), err)
	}
	if n != len(data) {
		return fsErr(where, "short probe write: %d of %d bytes", n, len(data))
	}
	if err := syncFile(f); err != nil {
		if isFullSyncUnsupported(err) {
			return fsErr(where, "full sync is unsupported on this filesystem: %v", err)
		}
		return fsErr(where, "full sync failed: %v", err)
	}
	q.FullSync = true
	if err := withFD(f, flockNB); err != nil {
		if isLockUnsupported(err) {
			return fsErr(where, "flock is unsupported on this filesystem: %v", err)
		}
		return fsErr(where, "flock(LOCK_EX|LOCK_NB) on a fresh private file failed: %v", err)
	}
	if err := withFD(f, flockRelease); err != nil {
		return fsErr(where, "flock(LOCK_UN) failed: %v", err)
	}
	q.Flock = true
	return nil
}
