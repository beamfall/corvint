//go:build linux

package safeopen

import (
	"os"
	"strconv"
	"syscall"
)

// Ancestors only need a pinned traversal handle. O_PATH avoids requiring
// directory read access (including Landlock READ_DIR) before the final open.
// Linux's O_PATH value is absent from syscall on some supported architectures,
// including amd64. Keep the ABI value here rather than adding a dependency.
const traversalFlags = 0x200000 | syscall.O_DIRECTORY

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}

// LstatIn describes name, one component beneath the traversal handle dir,
// without following it. The /dev/fd bridge rootFromFile also uses resolves
// to dir's pinned directory, never to a path, and inspecting an entry needs
// no read access to an O_PATH handle's directory (ASD-V0-2421).
func LstatIn(dir *os.File, name string) (os.FileInfo, error) {
	var info os.FileInfo
	err := inPinned(dir, name, func(path string) error {
		var err error
		info, err = os.Lstat(path)
		return err
	})
	return info, err
}

// ReadlinkIn returns the target of the symbolic link name, one component
// beneath the traversal handle dir, through the same bridge as LstatIn.
func ReadlinkIn(dir *os.File, name string) (string, error) {
	var target string
	err := inPinned(dir, name, func(path string) error {
		var err error
		target, err = os.Readlink(path)
		return err
	})
	return target, err
}

// inPinned calls fn with the bridge path of name beneath dir while dir's
// descriptor is held, so the path cannot name a reused descriptor.
func inPinned(dir *os.File, name string, fn func(string) error) error {
	if err := oneComponent(name); err != nil {
		return err
	}
	return Control(dir, func(fd uintptr) error {
		return fn("/dev/fd/" + strconv.FormatUint(uint64(fd), 10) + "/" + name)
	})
}
