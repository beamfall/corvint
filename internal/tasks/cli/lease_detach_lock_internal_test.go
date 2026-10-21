//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV11721_LaunchLockNeverTakenAfterTheBound: a holder that releases after
// the bound but before the next poll does not let the waiter take the lock
// late: the waiter sleeps at most the remaining bound and returns
// LOCK_TIMEOUT once it has passed (ATR-V0-024, V1-1721).
func TestV11721_LaunchLockNeverTakenAfterTheBound(t *testing.T) {
	p := filepath.Join(t.TempDir(), leaseDetachLockName)
	holder, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	bound := leaseDetachPoll / 4
	released := make(chan struct{})
	time.AfterFunc(leaseDetachPoll*3/5, func() {
		defer close(released)
		_ = syscall.Flock(int(holder.Fd()), syscall.LOCK_UN)
	})
	defer func() { <-released }()
	started := time.Now()
	unlock, err := lockLeaseDetach(p, bound)
	elapsed := time.Since(started)
	if err == nil {
		unlock()
		t.Fatalf("lock taken after %v, past the %v bound", elapsed, bound)
	}
	var we *wire.Error
	if !errors.As(err, &we) || we.Code != wire.CodeLockTimeout {
		t.Fatalf("err %v, want LOCK_TIMEOUT", err)
	}
	if elapsed >= leaseDetachPoll*3/5 {
		t.Fatalf("returned after %v, past the holder's release", elapsed)
	}
}
