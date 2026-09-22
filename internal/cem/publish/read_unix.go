//go:build darwin || linux

package publish

import (
	"os"
	"syscall"
)

func openBoundedInput(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
