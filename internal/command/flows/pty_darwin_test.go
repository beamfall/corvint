// SPDX-License-Identifier: Apache-2.0

package flows

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// openTestTerminal returns the slave side of a new pseudo-terminal. On Darwin the
// /dev/ptmx master does not answer TIOCGETA, so terminalInput needs the slave.
func openTestTerminal() (*os.File, func(), error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	ioctl := func(request uintptr, arg uintptr) error {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), request, arg); errno != 0 {
			return errno
		}
		return nil
	}
	name := new([128]byte) // heap: the address is passed as a uintptr below
	for _, call := range [][2]uintptr{{syscall.TIOCPTYGRANT, 0}, {syscall.TIOCPTYUNLK, 0}, {syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(name))}} {
		if err := ioctl(call[0], call[1]); err != nil {
			master.Close()
			return nil, nil, err
		}
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	slave, err := os.OpenFile(string(name[:n]), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, err
	}
	return slave, func() { slave.Close(); master.Close() }, nil
}
