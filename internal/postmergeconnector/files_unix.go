//go:build darwin || linux

package postmergeconnector

import (
	"fmt"
	"os"
	"syscall"
)

func openRegular(path string, before os.FileInfo) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	after, e := f.Stat()
	if e != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close()
		return nil, fmt.Errorf("file-changed")
	}
	return f, nil
}
func singleLink(i os.FileInfo) bool { s, ok := i.Sys().(*syscall.Stat_t); return ok && s.Nlink == 1 }
