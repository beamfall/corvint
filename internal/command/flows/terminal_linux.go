// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"os"
	"syscall"
	"unsafe"
)

// Query the terminal driver. A character device such as /dev/null fails this check.
func terminalInput(file *os.File) bool {
	var state syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&state)))
	return err == 0
}
