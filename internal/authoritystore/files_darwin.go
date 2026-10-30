//go:build darwin

package authoritystore

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"unsafe"
)

type darwinFiles struct{}

func openProtectedFiles() (protectedFiles, error) { return darwinFiles{}, nil }

func (darwinFiles) read(relative string, owner uint32, limit int) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative || strings.HasPrefix(relative, "../") {
		return nil, errUnavailable
	}
	file, before, err := openAudited(filepath.Join(RootPath, relative), owner, strings.HasPrefix(relative, "public/"))
	if err != nil {
		return nil, errUnavailable
	}
	defer file.Close()
	if before.Size < 0 || before.Size > int64(limit) {
		return nil, errUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(raw) > limit || int64(len(raw)) != before.Size {
		return nil, errUnavailable
	}
	var after syscall.Stat_t
	if syscall.Fstat(int(file.Fd()), &after) != nil || !sameStat(before, after) {
		return nil, errUnavailable
	}
	if noACL(file.Fd()) != nil {
		return nil, errUnavailable
	}
	return raw, nil
}

// Descriptor-relative nofollow traversal prevents both final symlinks and
// symlink/rename races in ancestors. Public directories may be authority-owned;
// every other ancestor must be root-owned. Group/other writes and ACLs refuse.
func openAudited(path string, owner uint32, publication bool) (*os.File, syscall.Stat_t, error) {
	var stat syscall.Stat_t
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, stat, errUnavailable
	}
	rootFD, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, stat, errUnavailable
	}
	current := os.NewFile(uintptr(rootFD), "/")
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if err = auditDescriptor(current.Fd(), 0, true, false); err != nil {
		current.Close()
		return nil, stat, errUnavailable
	}
	prefix := ""
	for index, part := range components {
		prefix += "/" + part
		last := index == len(components)-1
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC | syscall.O_NONBLOCK
		if !last {
			flags |= syscall.O_DIRECTORY
		}
		next, err := openAt(current.Fd(), part, flags)
		current.Close()
		if err != nil {
			return nil, stat, errUnavailable
		}
		current = os.NewFile(uintptr(next), prefix)
		expected := uint32(0)
		if prefix == RootPath+"/public" || strings.HasPrefix(prefix, RootPath+"/public/") || prefix == RootPath+"/private" || strings.HasPrefix(prefix, RootPath+"/private/") {
			expected = owner
		}
		if last {
			expected = owner
		}
		if err = auditDescriptor(current.Fd(), expected, !last, publication && last); err != nil {
			current.Close()
			return nil, stat, errUnavailable
		}

		if prefix == RootPath+"/private" || strings.HasPrefix(prefix, RootPath+"/private/") {
			var privateStat syscall.Stat_t
			if syscall.Fstat(int(current.Fd()), &privateStat) != nil {
				current.Close()
				return nil, stat, errUnavailable
			}
			expectedMode := uint16(0700)
			if last {
				expectedMode = 0600
			}
			if privateStat.Mode&0777 != expectedMode {
				current.Close()
				return nil, stat, errUnavailable
			}
		}
	}
	if err = syscall.Fstat(int(current.Fd()), &stat); err != nil {
		current.Close()
		return nil, stat, errUnavailable
	}
	return current, stat, nil
}

func openAt(parent uintptr, name string, flags int) (int, error) {
	pointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, errUnavailable
	}
	// SYS_openat=463 in the Darwin SDK; the frozen stdlib syscall table predates it.
	fd, _, errno := syscall.Syscall6(463, parent, uintptr(unsafe.Pointer(pointer)), uintptr(flags), 0, 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

func auditDescriptor(fd uintptr, owner uint32, directory, publicFile bool) error {
	var stat syscall.Stat_t
	if syscall.Fstat(int(fd), &stat) != nil {
		return errUnavailable
	}
	if stat.Uid != owner {
		return errUnavailable
	}
	if stat.Mode&0022 != 0 {
		return errUnavailable
	}
	if directory {
		if stat.Mode&syscall.S_IFMT != syscall.S_IFDIR {
			return errUnavailable
		}
	} else {
		if stat.Mode&syscall.S_IFMT != syscall.S_IFREG || stat.Nlink != 1 {
			return errUnavailable
		}
		if publicFile && stat.Mode&0777 != 0444 {
			return errUnavailable
		}
	}
	return noACL(fd)
}

// fgetattrlist(ATTR_CMN_EXTENDED_SECURITY) exposes ACL presence without invoking
// a shell or parsing ls output. We admit no ACL entries, including inherited
// entries. A nonempty or unsupported security attribute is unavailable.
func noACL(fd uintptr) error {
	attrs := [6]uint32{5, 0x00400000, 0, 0, 0, 0}
	buffer := make([]byte, 4096)
	_, _, errno := syscall.Syscall6(syscall.SYS_FGETATTRLIST, fd, uintptr(unsafe.Pointer(&attrs[0])), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	if errno != 0 {
		return errUnavailable
	}
	size := binary.LittleEndian.Uint32(buffer[:4])
	offset := binary.LittleEndian.Uint32(buffer[4:8])
	length := binary.LittleEndian.Uint32(buffer[8:12])
	if size != 12 || offset != 8 || length != 0 {
		return errUnavailable
	}
	return nil
}

func sameStat(a, b syscall.Stat_t) bool {
	a.Atimespec = syscall.Timespec{}
	b.Atimespec = syscall.Timespec{}
	return reflect.DeepEqual(a, b)
}
