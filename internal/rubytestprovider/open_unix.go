//go:build darwin || linux

package rubytestprovider

import (
	"os"
	"syscall"
)

// openNoFollow opens name read-only without blocking on a FIFO or device that
// replaced the admitted file. os.Root still resolves a symlink that stays
// inside the root, so readBounded's identity check refuses a swapped file.
func openNoFollow(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
