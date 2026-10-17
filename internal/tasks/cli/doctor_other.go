//go:build !darwin && !linux

package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
)

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
// written through it. render receives the destination's current bytes (nil
// when absent) and returns the bytes to write.
func writeDoctorCache(commonDir string, render func(prev []byte) ([]byte, error)) (err error) {
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
	var old []byte
	if cur, err := sub.Lstat(doctorCacheFileName); err == nil {
		if !cur.Mode().IsRegular() {
			return refuse("file is a link or not a regular file")
		}
		if !doctorOwned(cur) {
			return refuse("file is not owned by this user")
		}
		rf, err := sub.Open(doctorCacheFileName)
		if err != nil {
			return err
		}
		old, err = io.ReadAll(io.LimitReader(rf, doctorCacheBytes+1))
		rf.Close()
		if err != nil {
			return err
		}
		if err := doctorNewerCache(old); err != nil {
			return err
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
	raw, err := render(old)
	if err != nil {
		return err
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
