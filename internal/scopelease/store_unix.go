//go:build darwin || linux

package scopelease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// lock is the single-writer boundary: an exclusive flock on .lock with a
// bounded wait. The kernel drops the lock when its holder exits, so a crashed
// writer needs no reclamation step that could misjudge a live holder, and the
// file is never removed, so every writer contends on the same inode.
func lock(directory string) (func(), error) {
	file, err := os.OpenFile(filepath.Join(directory, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("scopelease: open lease lock: %w", err)
	}
	deadline := time.Now().Add(LockWait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = file.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = file.Close()
			return nil, fmt.Errorf("scopelease: lock lease lock: %w", err)
		}
		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, ErrLockBusy
		}
		time.Sleep(lockPoll)
	}
}

// syncDirectory makes a rename or removal inside directory durable.
func syncDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("scopelease: open lease directory: %w", err)
	}
	syncErr := handle.Sync()
	closeErr := handle.Close()
	if syncErr != nil {
		return fmt.Errorf("scopelease: sync lease directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("scopelease: close lease directory: %w", closeErr)
	}
	return nil
}
