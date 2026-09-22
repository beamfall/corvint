//go:build darwin || freebsd || linux

package worktreeimpact

import (
	"os"
	"syscall"
)

func openTarget(root *os.Root, value string) (*os.File, error) {
	return root.OpenFile(value, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
