//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package lspstdio

import (
	"os"
	"syscall"
)

func openWorkspaceFile(path string) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), path), nil
}
