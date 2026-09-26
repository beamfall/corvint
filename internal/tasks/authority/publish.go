package authority

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Dir is a root-confined handle on one directory of the repository
// authority's state dir (§3.4), opened for exclusive publication. It is
// the only place a link-in can happen: there is no arbitrary-path
// publication API in this package.
type Dir struct {
	root *os.Root
	path string
}

// OpenDir opens `<StateDir>/<rel>` of a resolved repository for
// publication. rel is a slash-separated local path ("receipts", "effects",
// "requests/ab") or "" for the state dir itself. The state dir must already
// exist: this package never creates it (that is `corvint-tasks init`, the journal
// slice). Symlinks anywhere in the path and non-directories are refused;
// every later open goes through the os.Root so no later swap can escape.
func OpenDir(repo *intent.Repository, rel string) (*Dir, error) {
	if !supportedPlatform {
		return nil, fsErr("", "unsupported platform")
	}
	if repo == nil {
		return nil, fsErr("", "no repository authority was resolved")
	}
	wantState := filepath.Join(repo.CommonDir, "taskman")
	if repo.StateDir != wantState {
		return nil, fsErr(repo.StateDir, "state dir is not %s (repository identity drift)", wantState)
	}
	if err := intent.CheckNoSymlink(repo.StateDir); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(repo.StateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, wire.Errorf(wire.CodeUninitialized, repo.StateDir, "state dir does not exist; `corvint-tasks init` creates it")
		}
		return nil, fsErr(repo.StateDir, "cannot stat: %v", err)
	}
	if !fi.IsDir() {
		return nil, fsErr(repo.StateDir, "state dir is not a directory")
	}
	state, err := openRoot(repo.StateDir)
	if err != nil {
		return nil, fsErr(repo.StateDir, "cannot open the state dir: %v", err)
	}
	opened, err := state.Stat(".")
	if err != nil || !os.SameFile(fi, opened) {
		state.Close()
		return nil, fsErr(repo.StateDir, "state directory identity drift")
	}
	if rel == "" {
		return &Dir{root: state, path: repo.StateDir}, nil
	}
	defer state.Close()
	if !filepath.IsLocal(rel) || strings.Contains(rel, "\\") || filepath.Clean(rel) != rel {
		return nil, fsErr(rel, "publication directory must be a clean local path under the state dir")
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return nil, fsErr(rel, "publication directory components may not start with a dot")
		}
	}
	// Record the intended directory identity; the safe open below separately
	// enforces no-follow traversal atomically on every component.
	acc := ""
	var intended os.FileInfo
	for _, part := range strings.Split(rel, "/") {
		acc = filepath.Join(acc, part)
		info, err := state.Lstat(acc)
		if err != nil {
			return nil, fsErr(filepath.Join(repo.StateDir, acc), "cannot stat: %v", err)
		}
		intended = info
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fsErr(filepath.Join(repo.StateDir, acc), "symlink inside the state dir")
		}
		if !info.IsDir() {
			return nil, fsErr(filepath.Join(repo.StateDir, acc), "not a directory")
		}
	}
	sub, err := openSubRoot(state, filepath.FromSlash(rel))
	if err != nil {
		return nil, fsErr(filepath.Join(repo.StateDir, rel), "cannot open: %v", err)
	}
	opened, err = sub.Stat(".")
	if err != nil || !os.SameFile(intended, opened) {
		sub.Close()
		return nil, fsErr(rel, "publication directory identity drift")
	}
	return &Dir{root: sub, path: filepath.Join(repo.StateDir, filepath.FromSlash(rel))}, nil
}

// Path is the directory's absolute path.
func (d *Dir) Path() string { return d.path }

// Close releases the directory handle. Files already published stay.
func (d *Dir) Close() error { return d.root.Close() }

// Sync fsyncs the directory itself (§5.1: directory fsync after every
// publication). An unsupported or failed sync is reported, never skipped.
func (d *Dir) Sync() error {
	f, err := d.root.Open(".")
	if err != nil {
		return fsErr(d.path, "cannot open the directory for sync: %v", err)
	}
	defer f.Close()
	if err := syncDirectory(f); err != nil {
		if isFullSyncUnsupported(err) {
			return fsErr(d.path, "directory sync is unsupported here: %v", err)
		}
		return fsErr(d.path, "directory sync failed: %v", err)
	}
	return nil
}

// Fsync makes an open file's bytes durable: F_FULLFSYNC on Darwin, fsync on
// Linux. It reports an unsupported sync as UNSUPPORTED_FILESYSTEM and never
// substitutes a weaker call.
func Fsync(f *os.File) error {
	if f == nil {
		return fsErr("", "Fsync of a nil file")
	}
	if err := syncFile(f); err != nil {
		if isFullSyncUnsupported(err) {
			return fsErr(f.Name(), "full sync is unsupported on this filesystem: %v", err)
		}
		return fsErr(f.Name(), "full sync failed: %v", err)
	}
	return nil
}

// PostLinkError reports a failure after the destination was linked in: the
// file named is complete and visible, but the temp unlink or the directory
// sync failed. The caller must not treat the publication as clean and must
// not retry it as if nothing happened (a retry would see ErrExists).
type PostLinkError struct {
	Name string
	Err  error
}

func (e *PostLinkError) Error() string {
	return "authority: " + e.Name + " was linked in but not cleanly finished: " + e.Err.Error()
}

// Unwrap exposes the underlying failure.
func (e *PostLinkError) Unwrap() error { return e.Err }

// validName admits one directory entry name: 1..255 bytes, no separator,
// no NUL, not "." or "..", and not of the reserved temp form `*.tmp-*`
// that §5.2 recovery deletes.
func validName(name string) error {
	if name == "" || len(name) > 255 {
		return fsErr(name, "link-in name must be 1..255 bytes")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return fsErr(name, "link-in name must be a single directory entry")
	}
	if strings.Contains(name, ".tmp-") {
		return fsErr(name, "link-in name may not use the reserved temp form *.tmp-*")
	}
	return nil
}

// LinkIn is the §5.2 exclusive creation primitive: write `<name>.tmp-<pid>-<random>`
// exclusively, full-sync it, `link(2)` it to `<name>`, unlink the temp,
// sync the directory. The destination is therefore either absent or
// complete, and it is never replaced: an existing destination returns
// ErrExists with nothing changed and the temp removed. Data must be
// non-empty and at most MaxLinkInBytes; a short write is a failure.
//
// Failure reporting: before the link, every error removes the temp and
// reports both the failure and any cleanup failure. After the link, a temp
// unlink or directory sync failure is returned as *PostLinkError because
// the destination now exists.
func (d *Dir) LinkIn(name string, data []byte) error {
	if !supportedPlatform {
		return fsErr("", "unsupported platform")
	}
	if err := validName(name); err != nil {
		return err
	}
	if len(data) == 0 {
		return wire.Errorf(wire.CodeMalformed, filepath.Join(d.path, name), "refusing to publish an empty file")
	}
	if len(data) > MaxLinkInBytes {
		return wire.Errorf(wire.CodeLimitExceeded, filepath.Join(d.path, name), "publication larger than %d bytes (%d)", MaxLinkInBytes, len(data))
	}
	dest := filepath.Join(d.path, name)
	// Refuse early when the destination is present, before creating a temp;
	// the link(2) below is still the authoritative exclusive check.
	if _, err := d.root.Lstat(name); err == nil {
		return ErrExists
	} else if !os.IsNotExist(err) {
		return fsErr(dest, "cannot stat the destination: %v", err)
	}
	tmp, err := uniqueName(name+".tmp-", "")
	if err != nil {
		return err
	}
	tmpPath := filepath.Join(d.path, tmp)
	f, err := d.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fsErr(tmpPath, "cannot create the temp file exclusively: %v", err)
	}
	remove := func() error {
		if err := d.root.Remove(tmp); err != nil && !os.IsNotExist(err) {
			return fsErr(tmpPath, "temp file was not removed: %v", err)
		}
		return nil
	}
	fail := func(primary error) error {
		closeErr := f.Close()
		var cleanup error
		if closeErr != nil {
			cleanup = fsErr(tmpPath, "close failed: %v", closeErr)
		}
		cleanup = errors.Join(cleanup, remove())
		return withCleanup(primary, cleanup)
	}
	n, err := f.Write(data)
	if err != nil {
		return fail(fsErr(tmpPath, "write failed after %d of %d bytes: %v", n, len(data), err))
	}
	if n != len(data) {
		return fail(fsErr(tmpPath, "short write: %d of %d bytes", n, len(data)))
	}
	if err := syncFile(f); err != nil {
		if isFullSyncUnsupported(err) {
			return fail(fsErr(tmpPath, "full sync is unsupported on this filesystem: %v", err))
		}
		return fail(fsErr(tmpPath, "full sync failed: %v", err))
	}
	if err := f.Close(); err != nil {
		return withCleanup(fsErr(tmpPath, "close failed: %v", err), remove())
	}
	if err := d.root.Link(tmp, name); err != nil {
		if errors.Is(err, os.ErrExist) {
			if cleanup := remove(); cleanup != nil {
				return errors.Join(ErrExists, cleanup)
			}
			return ErrExists
		}
		// R3: a non-EEXIST link failure is not a race outcome and authorizes
		// nothing; the temp is removed and the record counts as unwritten.
		return withCleanup(fsErr(dest, "link(2) failed: %v", err), remove())
	}
	if err := remove(); err != nil {
		return &PostLinkError{Name: name, Err: err}
	}
	if err := d.Sync(); err != nil {
		return &PostLinkError{Name: name, Err: err}
	}
	return nil
}
