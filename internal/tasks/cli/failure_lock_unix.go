//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// lockFailureLedger takes the ledger's exclusive lock, waiting up to bound
// for another run's read-check-write (TOL-V0-054). The kernel drops the lock
// when its holder dies, and the descriptor is close-on-exec.
func lockFailureLedger(p string, bound time.Duration) (func(), error) {
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
				return nil, errors.New("another run held the ledger lock past " + bound.String())
			}
			return nil, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}
