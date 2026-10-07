//go:build darwin || linux

package appmap

import (
	"os"
	"syscall"
)

// openInput opens a caller-named input without following a final symlink or blocking on a FIFO.
func openInput(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
