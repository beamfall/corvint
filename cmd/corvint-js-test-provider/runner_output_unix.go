//go:build darwin || linux

package main

import (
	"os"
	"syscall"
)

// openOutputFile opens the --runner-output path without following a final
// symlink and without blocking on a FIFO, refuses anything but a regular
// file, then sets mode 0600 and truncates (PWP-V0-028). O_NOFOLLOW does not
// constrain parent directories; the operator names their own directory chain.
func openOutputFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	var stat syscall.Stat_t
	err = syscall.Fstat(fd, &stat)
	if err == nil && stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		err = errRunnerOutputNotRegular
	}
	if err == nil {
		err = syscall.Fchmod(fd, 0o600)
	}
	if err == nil {
		err = syscall.Ftruncate(fd, 0)
	}
	if err == nil {
		err = syscall.SetNonblock(fd, false)
	}
	if err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
