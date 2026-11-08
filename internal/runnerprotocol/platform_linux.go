//go:build linux

// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Walk descriptor-relative with NOFOLLOW at every component, including parents.
func openRegular(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(absolute, "/"), "/")
	for i, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		next, e := syscall.Openat(fd, part, flags, 0)
		syscall.Close(fd)
		if e != nil {
			return nil, &os.PathError{Op: "open", Path: path, Err: e}
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("regular-file-required")
	}
	return f, nil
}
func lockStore(store string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(store, ".install.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("store-busy:%w", err)
	}
	return func() { f.Close() }, nil
}

// Seal a private descriptor before hashing and executing that same descriptor.
// An in-place store replacement cannot change the executable after admission.
func executableFile(r Resolved) (*os.File, error) {
	source, err := openRegular(r.path)
	if err != nil {
		return nil, fmt.Errorf("missing-executable:%w", err)
	}
	defer source.Close()
	if err = checkDigest(source, r.pin.SHA256); err != nil {
		return nil, err
	}
	var number uintptr
	switch runtime.GOARCH {
	case "amd64":
		number = 319
	case "arm64", "riscv64":
		number = 279
	default:
		return nil, fmt.Errorf("unsupported-executable-sealing")
	}
	name := []byte("corvint-runner\x00")
	fd, _, errno := syscall.Syscall(number, uintptr(unsafe.Pointer(&name[0])), 3, 0)
	if errno != 0 {
		return nil, fmt.Errorf("unsupported-executable-sealing:%w", errno)
	}
	f := os.NewFile(fd, "sealed-runner")
	if _, err = io.Copy(f, io.LimitReader(source, MaxExecutableBytes+1)); err == nil {
		err = f.Chmod(0500)
	}
	if err == nil {
		_, _, e := syscall.Syscall(syscall.SYS_FCNTL, fd, 1033, 15)
		if e != 0 {
			err = e
		}
	}
	if err == nil {
		_, err = f.Seek(0, 0)
	}
	if err == nil {
		err = checkDigest(f, r.pin.SHA256)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func executablePath() string { return "/proc/self/fd/3" }
