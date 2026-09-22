//go:build unix

package main

import "syscall"

// openRegularNoFollow opens path for reading without following a trailing
// symlink and without blocking on a FIFO. Only unix defines O_NOFOLLOW.
func openRegularNoFollow(path string) (int, error) {
	return syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
