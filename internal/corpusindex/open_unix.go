//go:build darwin || linux

package corpusindex

import (
	"os"
	"syscall"
)

// openInput opens a path for reading without blocking on a FIFO or device.
func openInput(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
