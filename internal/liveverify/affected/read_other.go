//go:build !darwin && !linux

package affected

import (
	"fmt"
	"os"
)

// readSourceFile stats the path before opening it where O_NOFOLLOW and
// O_NONBLOCK are not both available: the same refusals as read_unix.go, one
// path resolution more.
func readSourceFile(full, relative string) ([]byte, error) {
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %q is not a regular file", ErrInvalidUnit, relative)
	}
	if info.Size() > MaxSourceBytes {
		return nil, fmt.Errorf("%w: %q is %d bytes", ErrWalkLimit, relative, info.Size())
	}
	return os.ReadFile(full)
}
