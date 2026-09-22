//go:build unix

package docmaintain

import (
	"os"
	"syscall"
)

const watchPlatformSupported = true

func openWatchPage(path string) (*os.File, error) {
	// A replacement FIFO must not defeat the foreground session's deadline.
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
