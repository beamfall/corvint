//go:build darwin || linux

package session

import (
	"os"
	"syscall"
)

// openWatched opens a watched file without blocking on a FIFO or device.
func openWatched(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
