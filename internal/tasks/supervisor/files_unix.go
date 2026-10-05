//go:build darwin || linux

package supervisor

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"os"
	"path/filepath"
	"syscall"
)

// openExecutable opens path for reading without following a final symlink
// and without blocking on a FIFO; the caller checks the descriptor's type.
func openExecutable(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// protectedRuntime reports whether the runtime at path, whose descriptor
// status is st, can be executed by its path without a private copy: path is
// canonical (no symlink component), names the same file st describes, and it
// and every ancestor directory are owned by root and writable by neither group
// nor other. Substituting such a file needs root, so executing the path runs
// the verified object. Platform binaries, which macOS launch constraints keep
// from running as a copy, take this branch (CAL-V0-074).
func protectedRuntime(path string, st os.FileInfo) bool {
	if resolved, e := filepath.EvalSymlinks(path); e != nil || resolved != path {
		return false
	}
	native, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	for p := path; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil {
			return false
		}
		now, ok := info.Sys().(*syscall.Stat_t)
		if !ok || now.Uid != 0 || info.Mode().Perm()&0o022 != 0 || info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if p == path && (now.Dev != native.Dev || now.Ino != native.Ino) {
			return false
		}
		if p == "/" {
			return true
		}
	}
}

// writeRuntime writes the verified runtime bytes to a new private file at
// path (exclusive create, never through an existing name), syncs it and
// leaves it read and execute only.
func writeRuntime(path string, b []byte) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o700)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Chmod(0o500)
	}
	if e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		_ = os.Remove(path)
	}
	return e
}

func DirectoryIdentity(path string) (string, error) {
	r, e := safeopen.Root(path)
	if e != nil {
		return "", e
	}
	defer r.Close()
	st, e := r.Stat(".")
	if e != nil {
		return "", e
	}
	native, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("directory identity unavailable")
	}
	return fmt.Sprintf("%d:%d", native.Dev, native.Ino), nil
}
