// Package intent reads the Git-tracked intent store of SPEC §3.1
// (`.taskman/`), resolves the primary worktree (§3.1, §3.4) and computes the
// intent tree digest and publication facts. Everything here is read-only:
// no file is opened for writing, no directory is created, no lock is taken
// and no subprocess (including git) is run.
package intent

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Repository is the resolved repository authority of a working directory.
type Repository struct {
	// CommonDir is the Git common directory (`<primary>/.git`).
	CommonDir string
	// PrimaryWorktree is the worktree whose `.git` is the common directory
	// itself (§3.1). Linked worktrees are never the primary.
	PrimaryWorktree string
	// StateDir is `<git-common-dir>/taskman` (§3.4); it may not exist.
	StateDir string
	// LockPath is `<git-common-dir>/taskman.lock`; reads never open it.
	LockPath string
	// FromLinkedWorktree is true when cwd was inside a linked worktree.
	FromLinkedWorktree bool
}

// PrimaryWorktreeSha256 is the SHA-256 of the primary worktree's exact
// absolute path bytes (the WQO ExecutableIdentity.pathSha256 convention;
// SPEC §3.3 names the field but not the preimage, see docs/ROADMAP.md).
func (r *Repository) PrimaryWorktreeSha256() wire.Digest {
	return wire.Sum([]byte(r.PrimaryWorktree))
}

// CheckNoSymlink refuses any symlink component in an absolute path
// (§3.4: symlinks in the resolved path fail UNSUPPORTED_FILESYSTEM). A
// not-yet-existing tail is not a symlink.
func CheckNoSymlink(path string) error {
	return checkNoSymlink(path)
}

func checkNoSymlink(path string) error {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	cur := string(filepath.Separator)
	for _, p := range parts {
		if p == "" {
			continue
		}
		cur = filepath.Join(cur, p)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // a not-yet-existing tail is not a symlink
			}
			return wire.Errorf(wire.CodeUnsupportedFilesystem, cur, "cannot stat: %v", err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return wire.Errorf(wire.CodeUnsupportedFilesystem, cur, "symlink in the resolved path")
		}
	}
	return nil
}

// Resolve walks up from cwd to the first `.git` and resolves the common
// directory exactly as §3.4 prescribes: a `.git` directory is the common
// dir; a `.git` file names a `gitdir:` whose `commondir` file, if present,
// names the common dir. Windows is unsupported; no environment variable or
// flag relocates the state dir.
func Resolve(cwd string) (*Repository, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, cwd, "cannot resolve cwd: %v", err)
	}
	dir := filepath.Clean(abs)
	for {
		dotGit := filepath.Join(dir, ".git")
		fi, err := os.Lstat(dotGit)
		if err == nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dotGit, ".git is a symlink")
			}
			if fi.IsDir() {
				return finish(dotGit, false)
			}
			if fi.Mode().IsRegular() {
				return resolveGitFile(dotGit)
			}
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dotGit, ".git is neither a directory nor a file")
		} else if !os.IsNotExist(err) {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dotGit, "cannot stat: %v", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, wire.Errorf(wire.CodeUninitialized, abs, "no .git found walking up from cwd")
		}
		dir = parent
	}
}

func resolveGitFile(dotGit string) (*Repository, error) {
	raw, err := readBounded(dotGit, 4*wire.KiB)
	if err != nil {
		return nil, err
	}
	line := strings.TrimRight(string(raw), "\r\n")
	if !strings.HasPrefix(line, "gitdir: ") || strings.Contains(line, "\n") {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, dotGit, ".git file must contain a single gitdir: line")
	}
	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(filepath.Dir(dotGit), gitdir)
	}
	gitdir = filepath.Clean(gitdir)
	fi, err := os.Lstat(gitdir)
	if err != nil || !fi.IsDir() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, gitdir, "gitdir is not a directory")
	}
	common := gitdir
	cdPath := filepath.Join(gitdir, "commondir")
	if craw, err := readBounded(cdPath, 4*wire.KiB); err == nil {
		rel := strings.TrimRight(string(craw), "\r\n")
		if rel == "" || strings.Contains(rel, "\n") {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, cdPath, "commondir must be a single path")
		}
		if filepath.IsAbs(rel) {
			common = filepath.Clean(rel)
		} else {
			common = filepath.Clean(filepath.Join(gitdir, rel))
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return finish(common, common != gitdir)
}

func finish(common string, linked bool) (*Repository, error) {
	if err := checkNoSymlink(common); err != nil {
		return nil, err
	}
	if filepath.Base(common) != ".git" {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, common, "common dir is not a `.git` directory of a primary worktree (bare repositories are unsupported)")
	}
	fi, err := os.Lstat(common)
	if err != nil || !fi.IsDir() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, common, "common dir is not a directory")
	}
	primary := filepath.Dir(common)
	return &Repository{
		CommonDir:          common,
		PrimaryWorktree:    primary,
		StateDir:           filepath.Join(common, "taskman"),
		LockPath:           filepath.Join(common, "taskman.lock"),
		FromLinkedWorktree: linked,
	}, nil
}

// readBounded reads a whole file refusing anything larger than max. It opens
// the file read-only, refuses symlinks and non-regular files, re-checks the
// identity of the opened descriptor against the Lstat result (so a file
// swapped between stat and open is refused SNAPSHOT_MOVED rather than read),
// and propagates every read error: a short read is never returned as
// success.
func readBounded(path string, max int) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot stat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "symlink")
	}
	if !fi.Mode().IsRegular() {
		return nil, wire.Errorf(wire.CodeMalformed, path, "not a regular file")
	}
	if fi.Size() > int64(max) {
		return nil, wire.Errorf(wire.CodeLimitExceeded, path, "file larger than %d bytes (%d)", max, fi.Size())
	}
	f, err := openReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot open: %v", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "cannot stat the opened file: %v", err)
	}
	if !st.Mode().IsRegular() || !os.SameFile(fi, st) {
		return nil, wire.Errorf(wire.CodeSnapshotMoved, path, "file replaced between stat and open")
	}
	return readAll(f, path, max, int(fi.Size()))
}

// readAll drains r into memory with a hard byte bound. Any error other than
// io.EOF is propagated; truncated bytes are never returned as success.
func readAll(r io.Reader, path string, max int, hint int) ([]byte, error) {
	if hint < 0 || hint > max {
		hint = max
	}
	buf := make([]byte, 0, hint+1)
	tmp := make([]byte, 32*wire.KiB)
	for {
		n, rerr := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > max {
				return nil, wire.Errorf(wire.CodeLimitExceeded, path, "file larger than %d bytes", max)
			}
		}
		if rerr == io.EOF {
			return buf, nil
		}
		if rerr != nil {
			return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, path, "read failed after %d bytes: %v", len(buf), rerr)
		}
	}
}

// ReadFile is the exported bounded read-only file reader used by every read
// verb: it never creates, truncates or locks.
func ReadFile(path string, max int) ([]byte, error) {
	return readBounded(path, max)
}

// Replaced only by deterministic stat/open race tests.
var openReadFile = safeopen.File
