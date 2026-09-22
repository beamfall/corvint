//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package companionrelease

import (
	"fmt"
	"os"
	"syscall"
)

func openCopyRegular(path string, before os.FileInfo) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = f.Close()
		return nil, fmt.Errorf("cache member changed or is not regular: %s", path)
	}
	return f, nil
}
