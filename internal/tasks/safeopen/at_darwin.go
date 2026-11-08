//go:build darwin

package safeopen

import (
	"os"
	"syscall"
	"unsafe"
)

const traversalFlags = syscall.O_RDONLY | syscall.O_DIRECTORY

// Darwin openat is syscall 463 (XNU syscall ABI). Go 1.27 exposes only
// syscall's private libc openat wrapper. Use the pinned Syscall6 ABI here;
// no linkname into Go internals or third-party dependency is required.
func openat(fd int, name string, flags int, perm uint32) (int, error) {
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	r, _, errno := syscall.Syscall6(463, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(flags), uintptr(perm), 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(r), nil
}

// LstatIn describes name, one component beneath the pinned directory dir,
// without following it. Darwin traversal handles are readable directories,
// so the RootOf bridge serves (ASD-V0-2421).
func LstatIn(dir *os.File, name string) (os.FileInfo, error) {
	if err := oneComponent(name); err != nil {
		return nil, err
	}
	root, err := RootOf(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Lstat(name)
}

// ReadlinkIn returns the target of the symbolic link name, one component
// beneath the pinned directory dir.
func ReadlinkIn(dir *os.File, name string) (string, error) {
	if err := oneComponent(name); err != nil {
		return "", err
	}
	root, err := RootOf(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	return root.Readlink(name)
}
