//go:build unix

package extevidence

import (
	"os"
	"syscall"
)

// openSource opens a provider file without blocking on a FIFO, so a path
// replaced by a FIFO after any earlier check is refused by the caller's fstat
// instead of hanging the read (EEP-V0-805, V1-0562).
func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
