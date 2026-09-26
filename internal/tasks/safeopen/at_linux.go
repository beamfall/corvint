//go:build linux

package safeopen

import "syscall"

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
