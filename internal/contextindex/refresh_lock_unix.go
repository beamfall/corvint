//go:build darwin || linux

package contextindex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	file, err := openSnapshotRefreshLock(store, true)
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
	expected, err := openSnapshotRefreshLock(store, false)
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

// Walk from the store anchor using native openat, not os.Root.OpenFile: os.Root
// resolves in-root symlinks even when O_NOFOLLOW is passed. An opened directory
// stays pinned if its pathname is subsequently replaced. No leaf is truncated
// until ValidateSnapshotRefreshLock has independently repeated this same walk.
func openSnapshotRefreshLock(store snapshotStore, create bool) (*os.File, error) {
	relative, err := filepath.Rel(store.base, store.directory)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, errors.New("index refresh store path refused")
	}
	current, err := os.OpenFile(store.base, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if !validComponent(component) {
			current.Close()
			return nil, errors.New("index refresh directory component refused")
		}
		if create {
			err = snapshotRefreshMkdirat(int(current.Fd()), component, 0o755)
			if err != nil && !errors.Is(err, syscall.EEXIST) {
				current.Close()
				return nil, &os.PathError{Op: "mkdirat", Path: component, Err: err}
			}
		}
		fd, err := snapshotRefreshOpenat(int(current.Fd()), component, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		current.Close()
		if err != nil {
			return nil, &os.PathError{Op: "openat directory", Path: component, Err: err}
		}
		current = os.NewFile(uintptr(fd), component)
	}
	defer current.Close()
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	var fd int
	if create {
		// Separate creation from opening the stable inode: concurrent Darwin
		// O_CREAT opens can return ENOENT while another caller creates it.
		fd, err = snapshotRefreshOpenat(int(current.Fd()), "refresh.lock", flags|os.O_CREATE|os.O_EXCL, 0o600)
	}
	if !create || errors.Is(err, syscall.EEXIST) {
		fd, err = snapshotRefreshOpenat(int(current.Fd()), "refresh.lock", flags, 0)
	}
	if err != nil {
		return nil, &os.PathError{Op: "openat lock", Path: "refresh.lock", Err: err}
	}
	return os.NewFile(uintptr(fd), "refresh.lock"), nil
}
