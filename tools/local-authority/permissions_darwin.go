//go:build darwin

package main

import (
	"encoding/binary"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func ensureRootDirectory(path string) error { return auditRootDirectory(path, true) }
func auditRootDirectory(path string, create bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("directory path")
	}
	// All ancestors are operator-owned; no caller-writable parent or ACL can
	// redirect the one-shot install destination.
	prefix := "/"
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		prefix = filepath.Join(prefix, part)
		info, e := os.Lstat(prefix)
		if os.IsNotExist(e) && create {
			if e = os.Mkdir(prefix, 0755); e != nil {
				return e
			}
			info, e = os.Lstat(prefix)
		}
		if e != nil {
			return e
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || st.Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.New("unsafe root directory")
		}
		if e = emptyACL(prefix); e != nil {
			return e
		}
	}
	return nil
}
func emptyACL(path string) error {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return e
	}
	defer syscall.Close(fd)
	attrs := [6]uint32{5, 0x00400000, 0, 0, 0, 0}
	buffer := make([]byte, 4096)
	_, _, errno := syscall.Syscall6(syscall.SYS_FGETATTRLIST, uintptr(fd), uintptr(unsafe.Pointer(&attrs[0])), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	if errno != 0 {
		return errno
	}
	if binary.LittleEndian.Uint32(buffer[:4]) != 12 || binary.LittleEndian.Uint32(buffer[4:8]) != 8 || binary.LittleEndian.Uint32(buffer[8:12]) != 0 {
		return errors.New("ACL present or unsupported")
	}
	return nil
}

func openAuthorityDirectory(relative string, owner uint32) (int, error) {
	if relative != "private" && relative != "public" && relative != "private/enrollments" {
		return -1, errors.New("unowned directory")
	}
	if e := ensureRootDirectory("/Library/CorvintAuthority"); e != nil {
		return -1, e
	}
	fd, e := unix.Open("/Library/CorvintAuthority", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, e
	}
	for _, part := range strings.Split(relative, "/") {
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return -1, e
		}
		fd = next
		var st unix.Stat_t
		if e = unix.Fstat(fd, &st); e != nil {
			unix.Close(fd)
			return -1, e
		}
		if st.Uid != owner || st.Mode&0022 != 0 {
			unix.Close(fd)
			return -1, errors.New("directory ownership")
		}
		if e = emptyACLFD(fd); e != nil {
			unix.Close(fd)
			return -1, e
		}
	}
	return fd, nil
}
func renameExclusive(from int, name string, to int, target string) error {
	return unix.RenameatxNp(from, name, to, target, unix.RENAME_EXCL)
}
func emptyACLFD(fd int) error {
	attrs := [6]uint32{5, 0x00400000, 0, 0, 0, 0}
	buffer := make([]byte, 4096)
	_, _, errno := syscall.Syscall6(syscall.SYS_FGETATTRLIST, uintptr(fd), uintptr(unsafe.Pointer(&attrs[0])), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	if errno != 0 {
		return errno
	}
	if binary.LittleEndian.Uint32(buffer[:4]) != 12 || binary.LittleEndian.Uint32(buffer[4:8]) != 8 || binary.LittleEndian.Uint32(buffer[8:12]) != 0 {
		return errors.New("ACL present or unsupported")
	}
	return nil
}

func readRootFile(path string, limit int) ([]byte, error) {
	if e := auditRootDirectory(filepath.Dir(path), false); e != nil {
		return nil, e
	}
	fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var before unix.Stat_t
	if e = unix.Fstat(fd, &before); e != nil {
		return nil, e
	}
	if before.Uid != 0 || before.Mode&0022 != 0 || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Nlink != 1 {
		return nil, errors.New("unprotected root file")
	}
	if e = emptyACLFD(fd); e != nil {
		return nil, e
	}
	raw, e := readBound(f, limit)
	if e != nil {
		return nil, e
	}
	var after unix.Stat_t
	if e = unix.Fstat(fd, &after); e != nil || !sameRootStat(before, after) {
		return nil, errors.New("root file changed")
	}
	return raw, nil
}

func sameRootStat(a, b unix.Stat_t) bool {
	a.Atim = unix.Timespec{}
	b.Atim = unix.Timespec{}
	return a == b
}
