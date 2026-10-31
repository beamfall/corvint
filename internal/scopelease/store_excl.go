package scopelease

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// openLockFile is the create seam of exclusiveLock; tests replace it.
var openLockFile = os.OpenFile

// exclusiveLock is the deferred-platform lock (decision 0061): one O_EXCL
// creation with a bounded wait and age-only stale reclamation. Its
// check-then-remove reclaim is not race-free; darwin and linux use a kernel
// flock instead. A create refused for permission is contention: on Windows a
// lock another writer removed but has not closed is delete-pending and
// refuses re-creation with ERROR_ACCESS_DENIED (SCL-V0-900).
func exclusiveLock(directory string) (func(), error) {
	path := filepath.Join(directory, lockName)
	deadline := time.Now().Add(LockWait)
	for {
		file, err := openLockFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("scopelease: close lease lock: %w", closeErr)
			}
			return func() { releaseLock(path) }, nil
		}
		denied := errors.Is(err, fs.ErrPermission)
		if !denied && !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("scopelease: create lease lock: %w", err)
		}
		if info, statErr := os.Stat(path); !denied && statErr == nil && time.Since(info.ModTime()) > StaleLockAge {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			if denied {
				return nil, fmt.Errorf("%w (last create refused: %v)", ErrLockBusy, err)
			}
			return nil, ErrLockBusy
		}
		time.Sleep(lockPoll)
	}
}

// releaseLock removes the lock only while it still records this process.
func releaseLock(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if strings.TrimSpace(string(content)) != strconv.Itoa(os.Getpid()) {
		return
	}
	_ = os.Remove(path)
}
