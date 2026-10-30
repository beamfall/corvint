//go:build unix

package playwrightruns

import (
	"os"
	"syscall"
)

// openNonblocking opens without blocking on a FIFO; ReadFile then refuses any non-regular file.
func openNonblocking(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
