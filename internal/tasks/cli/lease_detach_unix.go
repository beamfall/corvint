//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// lockLeaseDetach takes the record's launch lock, waiting up to bound for
// another caller's launch; the kernel drops it when its holder dies, and the
// descriptor is close-on-exec, so the detached child never holds it.
func lockLeaseDetach(p string, bound time.Duration) (func(), error) {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(bound)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			_ = f.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return nil, wire.Errorf(wire.CodeLockTimeout, "detach", "another caller is still starting this detached request")
			}
			return nil, err
		}
		time.Sleep(leaseDetachPoll)
	}
}

// leaseDetachLockHeld reports whether another process holds the lock file p.
func leaseDetachLockHeld(p string) bool {
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK)
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}
