//go:build darwin || linux

package authority

import (
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"os"
	"strings"
	"syscall"
)

// hooks are the OS entry points the primitives call. Tests inside this
// package replace them to inject a durability, locking or publication
// failure; nothing outside the package can.
var (
	syncFile      = fullSync
	syncDirectory = directorySync
	flockNB       = flockExclusiveNB
	flockRelease  = flockUnlock
)

// flockExclusiveNB is `flock(LOCK_EX|LOCK_NB)` (§5.1).
func flockExclusiveNB(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
}

// flockUnlock is `flock(LOCK_UN)`.
func flockUnlock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_UN)
}

// isWouldBlock reports lock contention: another open file description
// holds the lock.
func isWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}

// isLockUnsupported reports the §5.1 refusals: ENOLCK, EOPNOTSUPP, ENOTSUP.
func isLockUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOLCK) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTSUP)
}

// isInterrupted reports EINTR, which is retried.
func isInterrupted(err error) bool {
	return errors.Is(err, syscall.EINTR)
}

// isFullSyncUnsupported reports a sync the filesystem refused rather than
// failed: ENOTSUP/EOPNOTSUPP (F_FULLFSYNC on a volume without it) or EINVAL
// (fsync on a descriptor that does not support synchronisation).
func isFullSyncUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EINVAL)
}

// withFD pins the file for each syscall, including against concurrent Close.
func withFD(f *os.File, fn func(int) error) error {
	return safeopen.Control(f, func(fd uintptr) error { return fn(int(fd)) })
}
func directorySync(f *os.File) error {
	return withFD(f, func(fd int) error {
		for {
			err := syscall.Fsync(fd)
			if err != syscall.EINTR {
				return err
			}
		}
	})
}

// cString converts a NUL-terminated fixed C char array to a Go string.
func cString(b []int8) string {
	// Size the result to the NUL-terminated prefix: one allocation of the
	// name's own length, not a buffer of the whole fixed array per call
	// (fixture session checks call this per retained parent per operation).
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	var out strings.Builder
	out.Grow(n)
	for _, c := range b[:n] {
		out.WriteByte(byte(c))
	}
	return out.String()
}
