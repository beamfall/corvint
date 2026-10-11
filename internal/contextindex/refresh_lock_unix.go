//go:build darwin || linux

package contextindex

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// AcquireSnapshotRefreshLock admits one background index writer per shared store
// (proposed IDX-SNAP-V0-032). A nil file and nil error means another writer owns
// it. Close releases the flock, including after a crash; never unlink the inode,
// because another opener could then lock a different inode at the same path.
func AcquireSnapshotRefreshLock(root string) (*os.File, error) {
	store := locateSnapshotStore(root)
	if err := refuseLinkedSnapshotDirectory(root, store.directory); err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(store.base, store.directory)
	if err != nil {
		return nil, err
	}
	directory, err := openBlobShardDirectory(store.base, relative, true)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	// EXCL prevents even os.Root's in-root symlink resolution from creating a
	// dangling link's target. Existing entries are opened without CREATE.
	file, err := directory.OpenFile("refresh.lock", os.O_CREATE|os.O_EXCL|flags, 0o600)
	if errors.Is(err, os.ErrExist) {
		file, err = directory.OpenFile("refresh.lock", flags, 0)
	}
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("index refresh lock must be a regular file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 {
		file.Close()
		return nil, errors.New("hardlinked index refresh lock refused")
	}
	if !sameBlobShardEntry(directory, "refresh.lock", file) {
		file.Close()
		return nil, errors.New("linked index refresh lock refused")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, nil
		}
		return nil, err
	}
	return file, nil
}

// ValidateSnapshotRefreshLock verifies the inherited descriptor before the child
// writes its PID. An internal environment marker alone cannot authorize an
// arbitrary inherited file to be truncated.
func ValidateSnapshotRefreshLock(root string, file *os.File) error {
	store := locateSnapshotStore(root)
	expected, err := openBlobShard(store.base, filepath.Join(store.directory, "refresh.lock"))
	if err != nil {
		return err
	}
	defer expected.Close()
	a, err := expected.Stat()
	if err != nil {
		return err
	}
	b, err := file.Stat()
	if err != nil {
		return err
	}
	if !a.Mode().IsRegular() || !os.SameFile(a, b) {
		return errors.New("index refresh descriptor mismatch")
	}
	if stat, ok := a.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 {
		return errors.New("hardlinked index refresh lock refused")
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
