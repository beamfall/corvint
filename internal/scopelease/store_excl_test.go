package scopelease

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// SCL-V0-900 (V1-2489): on a deferred platform a lock create refused for
// permission, as Windows refuses a delete-pending lock file, is contention:
// it is retried and then granted once the file is gone, or reported
// ErrLockBusy naming the refusal when the wait is spent. Any other create
// failure is still reported at once.
func TestExclusiveLockTreatsDeletePendingAsContention(t *testing.T) {
	saved := openLockFile
	t.Cleanup(func() { openLockFile = saved })
	denied := &fs.PathError{Op: "open", Path: lockName, Err: fs.ErrPermission}

	t.Run("granted after the pending delete", func(t *testing.T) {
		calls := 0
		openLockFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if calls++; calls <= 3 {
				return nil, denied
			}
			return saved(name, flag, perm)
		}
		release, err := exclusiveLock(t.TempDir())
		if err != nil || calls != 4 {
			t.Fatalf("lock after delete-pending refusals: err %v after %d opens", err, calls)
		}
		release()
	})
	t.Run("busy when the wait is spent", func(t *testing.T) {
		openLockFile = func(string, int, os.FileMode) (*os.File, error) { return nil, denied }
		_, err := exclusiveLock(t.TempDir())
		if !errors.Is(err, ErrLockBusy) || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("persistent permission refusal: %v", err)
		}
	})
	t.Run("other failures at once", func(t *testing.T) {
		calls := 0
		openLockFile = func(string, int, os.FileMode) (*os.File, error) {
			calls++
			return nil, &fs.PathError{Op: "open", Path: lockName, Err: errors.New("i/o error")}
		}
		_, err := exclusiveLock(t.TempDir())
		if err == nil || errors.Is(err, ErrLockBusy) || calls != 1 {
			t.Fatalf("other create failure: %v after %d opens", err, calls)
		}
	})
}
