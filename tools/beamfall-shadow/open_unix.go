//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"errors"
	"os"
	"syscall"
)

func openRegularNoFollow(name string, limit int64) (*os.File, os.FileInfo, error) {
	f, err := os.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > limit {
		return nil, nil, errors.Join(err, f.Close(), errors.New("not a bounded regular file"))
	}
	return f, info, nil
}
