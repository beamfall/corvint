// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package corpusbridge

import (
	"os"
	"syscall"
)

// openStampFile opens path read-only without following a final symlink and
// without blocking on a FIFO or device; the caller checks the opened file's
// type before reading.
func openStampFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
