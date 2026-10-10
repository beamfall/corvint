//go:build darwin || linux

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// doctorPluginGroup runs a plugin in its own process group and kills the
// whole group when its timeout cancels it (TQD-V0-010).
func doctorPluginGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
}

// doctorPluginReap kills whatever is left of the plugin's process group
// after the plugin exits or times out, so no descendant outlives the
// doctor (TQD-V0-010). The group id cannot be reused while any member
// lives, and an empty group answers ESRCH.
func doctorPluginReap(c *exec.Cmd) {
	if c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}

// doctorOwned reports whether the effective user owns the file.
func doctorOwned(st fs.FileInfo) bool {
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && int(sys.Uid) == os.Geteuid()
}

// writeDoctorCache replaces <commonDir>/taskman-doctor/summary.json through a
// temporary file, fsync and rename (TQD-V0-011). The common directory is
// pinned without following links and the cache directory is opened once,
// O_NOFOLLOW, beneath it; every later step (fchmod, lock, destination
// check, temporary create, rename) is relative to that descriptor, so the
// cache path is never resolved by name again. A descriptor pins the
// directory, not its place, so before the chmod, after locking, before the
// temporary create, before the rename and before success is reported the
// entry taskman-doctor beneath the pinned common directory must still be
// that same directory; a directory moved away (for example into the state
// directory) is refused UNSUPPORTED_FILESYSTEM and the temporary file is
// removed; a move landing between the last pre-rename check and the rename
// is reported by the final check, not undone. A link, a non-directory or non-regular file, or a foreign owner
// is refused UNSUPPORTED_FILESYSTEM before anything is changed or written
// through it. Under an exclusive
// flock on taskman-doctor/.lock the destination is re-read and a cache
// written by a later build is refused UNSUPPORTED_VERSION, unchanged.
func writeDoctorCache(commonDir string, raw []byte) (err error) {
	root, err := safeopen.Root(commonDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir(doctorCacheDirName, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if doctorCacheHook != nil {
		doctorCacheHook("open")
	}
	dir, err := safeopen.InRoot(root, doctorCacheDirName, os.O_RDONLY, 0, true)
	if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
		return doctorCacheRefusal("directory is a link or not a directory")
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	st, err := dir.Stat()
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return doctorCacheRefusal("directory is not a directory")
	}
	if !doctorOwned(st) {
		return doctorCacheRefusal("directory is not owned by this user")
	}
	inPlace := func() error {
		here, err := root.Lstat(doctorCacheDirName)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err != nil || !here.IsDir() || !os.SameFile(here, st) {
			return doctorCacheRefusal("directory moved after it was opened")
		}
		return nil
	}
	if err := inPlace(); err != nil {
		return err
	}
	if st.Mode().Perm() != 0o700 {
		if err := dir.Chmod(0o700); err != nil {
			return err
		}
	}
	lock, err := doctorCacheFile(dir, doctorCacheLockName, os.O_RDWR|os.O_CREATE, "lock file")
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := doctorLock(lock); err != nil {
		return err
	}
	if doctorCacheHook != nil {
		doctorCacheHook("lock")
	}
	if err := inPlace(); err != nil {
		return err
	}
	cur, err := doctorCacheFile(dir, doctorCacheFileName, os.O_RDONLY, "file")
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		old, rerr := io.ReadAll(io.LimitReader(cur, doctorCacheBytes+1))
		cur.Close()
		if rerr != nil {
			return rerr
		}
		if err := doctorNewerCache(old); err != nil {
			return err
		}
	}
	at, err := safeopen.RootOf(dir)
	if err != nil {
		return err
	}
	defer at.Close()
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	tmp := ".summary-" + hex.EncodeToString(nonce[:]) + ".tmp"
	if err := inPlace(); err != nil {
		return err
	}
	f, err := safeopen.InDir(dir, tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = at.Remove(tmp)
		}
	}()
	if _, err = f.Write(raw); err != nil {
		return err
	}
	if err = f.Chmod(0o600); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = inPlace(); err != nil {
		return err
	}
	if err = at.Rename(tmp, doctorCacheFileName); err != nil {
		return err
	}
	return inPlace()
}

// doctorLockWait bounds how long a refresh waits for another holder of the
// cache lock, polling a non-blocking flock every doctorLockPoll; past it the
// refresh refuses LOCK_TIMEOUT having written nothing (TQD-V0-011).
const doctorLockWait, doctorLockPoll = 3 * time.Second, 50 * time.Millisecond

func doctorLock(lock *os.File) error {
	deadline := time.Now().Add(doctorLockWait)
	for {
		err := safeopen.Control(lock, func(fd uintptr) error {
			for {
				if err := syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB); err != syscall.EINTR {
					return err
				}
			}
		})
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		if !time.Now().Before(deadline) {
			return wire.Errorf(wire.CodeLockTimeout, doctorCacheDirName+"/"+doctorCacheLockName, "doctor cache lock held by another refresh for more than %v; nothing was written", doctorLockWait)
		}
		time.Sleep(doctorLockPoll)
	}
}

// doctorCacheFile opens name beneath the pinned cache directory without
// following a link and refuses anything but a regular file the user owns.
func doctorCacheFile(dir *os.File, name string, flags int, what string) (*os.File, error) {
	f, err := safeopen.InDir(dir, name, flags, 0o600)
	if errors.Is(err, syscall.ELOOP) {
		return nil, doctorCacheRefusal(what + " is a link")
	}
	// The open itself fails on a directory (EISDIR) or a socket or device
	// (ENXIO, EOPNOTSUPP) before the descriptor's type can be checked.
	if errors.Is(err, syscall.EISDIR) || errors.Is(err, syscall.ENXIO) || errors.Is(err, syscall.EOPNOTSUPP) {
		return nil, doctorCacheRefusal(what + " is not a regular file")
	}
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	switch {
	case err != nil:
	case !st.Mode().IsRegular():
		err = doctorCacheRefusal(what + " is not a regular file")
	case !doctorOwned(st):
		err = doctorCacheRefusal(what + " is not owned by this user")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
