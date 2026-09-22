//go:build !darwin && !linux

package main

import "os"

func openRegularNoFollow(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return nil, os.ErrInvalid
	}
	return os.Open(path)
}
