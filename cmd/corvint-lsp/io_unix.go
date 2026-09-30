//go:build unix

// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"errors"
	"os"
	"syscall"
)

func pollable(f *os.File) (*os.File, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeSocket) == 0 {
		return nil, errors.New("stdio requires owned pipes or sockets")
	}
	fd := f.Fd()
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		return nil, err
	}
	return os.NewFile(fd, f.Name()), nil
}
