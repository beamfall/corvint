//go:build unix

package receiptbundle

import (
	"os"
	"syscall"
)

// openStableEntry opens a bundle entry without following a final symlink;
// O_NONBLOCK keeps an entry replaced by a FIFO from blocking before the caller's
// identity check refuses it.
func openStableEntry(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}
