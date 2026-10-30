//go:build darwin || linux

package jstestprovider

import (
	"os"
	"syscall"
)

// Refuse a final symlink and prevent a FIFO replacement from blocking Open.
// The caller checks the descriptor's type before reading. This does not add
// hostile ancestor replacement containment or deadlines for regular-file I/O.
func freshOpenDependency(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
