//go:build unix

package stepnegation

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive flock on name; the kernel drops
// it when the holder exits, so a crash never leaves a stale lock.
func lockFile(directory *os.Root, name string) (func(), error) {
	if info, err := directory.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("step-negation lock is not a regular file")
	}
	file, err := directory.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("step-negation lock cannot be opened: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("step-negation lock cannot be taken: %w", err)
	}
	return func() { file.Close() }, nil
}
