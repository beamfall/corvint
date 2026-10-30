//go:build darwin || linux

package update

import (
	"fmt"
	"os"
	"syscall"
)

// Lock the destination directory inode, so different state roots serialize and
// kernel ownership is released on process death without stale lock recovery.
func lockDirectory(path string) (func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("destination mutation locked: %w", err)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
