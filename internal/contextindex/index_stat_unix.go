//go:build darwin || linux

package contextindex

import (
	"os"
	"syscall"
)

func openGitIndex(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
}
