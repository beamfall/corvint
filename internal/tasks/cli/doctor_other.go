//go:build !darwin && !linux

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
)

// doctorPluginGroup: elsewhere the timeout kills the plugin process only.
func doctorPluginGroup(c *exec.Cmd) {}

// doctorPluginReap: elsewhere there is no process group to kill.
func doctorPluginReap(c *exec.Cmd) {}

// doctorOwned: elsewhere ownership is not checked.
func doctorOwned(st fs.FileInfo) bool { return true }

// writeDoctorCache replaces <commonDir>/taskman-doctor/summary.json through a
// temporary file, fsync and rename (TQD-V0-011). Off darwin and linux there
// is no descriptor-anchored boundary and no flock: steps are rooted at the
// common directory through os.Root, which can follow a link swapped in
// between the check and the open, and concurrent refreshes are not
// serialized (a documented limit). The rest follows no link: a cache directory or file that is a
// link, not a directory or regular file, or not owned by the caller is
// refused UNSUPPORTED_FILESYSTEM before anything is created, changed or
// written through it.
func writeDoctorCache(commonDir string, raw []byte) (err error) {
	root, err := os.OpenRoot(commonDir)
	if err != nil {
		return err
	}
	defer root.Close()
	refuse := doctorCacheRefusal
	st, err := root.Lstat(doctorCacheDirName)
	if errors.Is(err, fs.ErrNotExist) {
		if err = root.Mkdir(doctorCacheDirName, 0o700); err != nil {
			return err
		}
		st, err = root.Lstat(doctorCacheDirName)
	}
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&fs.ModeSymlink != 0 {
		return refuse("directory is a link or not a directory")
	}
	if !doctorOwned(st) {
		return refuse("directory is not owned by this user")
	}
	if doctorCacheHook != nil {
		doctorCacheHook("open")
	}
	sub, err := root.OpenRoot(doctorCacheDirName)
	if err != nil {
		return err
	}
	defer sub.Close()
	if here, err := sub.Stat("."); err != nil || !os.SameFile(here, st) {
		return refuse("directory changed while it was opened")
	}
	if cur, err := sub.Lstat(doctorCacheFileName); err == nil {
		if !cur.Mode().IsRegular() {
			return refuse("file is a link or not a regular file")
		}
		if !doctorOwned(cur) {
			return refuse("file is not owned by this user")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if st.Mode().Perm() != 0o700 {
		if err := sub.Chmod(".", 0o700); err != nil {
			return err
		}
	}
	if doctorCacheHook != nil {
		doctorCacheHook("lock")
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	tmp := ".summary-" + hex.EncodeToString(nonce[:]) + ".tmp"
	f, err := sub.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = sub.Remove(tmp)
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
	return sub.Rename(tmp, doctorCacheFileName)
}
