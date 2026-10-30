//go:build darwin || linux

package main

import (
	"errors"
	"io"
	"os"
	"syscall"
)

func replaceNativeHook(path string, args, environment []string) error {
	return syscall.Exec(path, args, environment)
}
func readNativeHookFile(path string, limit int64) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("not a regular enrollment file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("enrollment exceeds bound")
	}
	return raw, nil
}
