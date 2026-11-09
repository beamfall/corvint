//go:build darwin || linux

package cli

import (
	"os"
	"syscall"
)

func openStableMap(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
