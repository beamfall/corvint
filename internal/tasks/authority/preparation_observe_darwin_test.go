//go:build darwin

package authority

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

// Darwin's lock query sees every process; nothing to assume.
func assumeCompleteLockTable(*testing.T) {}

// recordLockKinds are the non-flock locks a foreign process can hold here.
var recordLockKinds = []string{"posix"}

func holdRecordLock(f *os.File, kind string) error {
	return syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &syscall.Flock_t{Type: syscall.F_RDLCK, Start: 100, Len: 1})
}

// A Darwin record lock hides whether a flock coexists, so the read abstains.
func checkRecordLockObservation(t *testing.T, kind string, q PreparationQueue) {
	t.Helper()
	if !strings.Contains(q.NotObserved, "record lock") || q.Registered != 0 {
		t.Fatalf("%s: %+v", kind, q)
	}
}
