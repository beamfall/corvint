//go:build darwin || linux

package authority

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/Beamfall/corvint/internal/tasks/safeopen"
)

// All directory and source/destination descriptors remain pinned throughout
// the native call. No /dev/fd pathname is used as syscall path authority.
func fixturePair(a, b, source, dest *os.File, fn func(uintptr, uintptr) error) error {
	return safeopen.Control(a, func(afd uintptr) error {
		return safeopen.Control(b, func(bfd uintptr) error {
			return safeopen.Control(source, func(uintptr) error {
				if dest == nil {
					return fn(afd, bfd)
				}
				return safeopen.Control(dest, func(uintptr) error { return fn(afd, bfd) })
			})
		})
	})
}
func fixtureTwoPaths(trap uintptr, a *os.File, from string, b *os.File, to string, source, dest *os.File) error {
	old, err := syscall.BytePtrFromString(from)
	if err != nil {
		return err
	}
	new, err := syscall.BytePtrFromString(to)
	if err != nil {
		return err
	}
	return fixturePair(a, b, source, dest, func(afd, bfd uintptr) error {
		_, _, errno := syscall.Syscall6(trap, afd, uintptr(unsafe.Pointer(old)), bfd, uintptr(unsafe.Pointer(new)), 0, 0)
		if errno != 0 {
			return errno
		}
		return nil
	})
}
func fixtureLink(a *os.File, from string, b *os.File, to string, source *os.File) error {
	return fixtureTwoPaths(fixtureLinkat, a, from, b, to, source, nil)
}
func fixtureUnlink(parent *os.File, name string, source *os.File) error {
	return safeopen.Control(source, func(uintptr) error { return fixtureOnePath(fixtureUnlinkat, parent, name, 0) })
}
func fixtureMkdir(parent *os.File, name string) error {
	return fixtureOnePath(fixtureMkdirat, parent, name, 0700)
}
func fixtureOnePath(trap uintptr, parent *os.File, name string, arg uintptr) error {
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	return safeopen.Control(parent, func(fd uintptr) error {
		_, _, errno := syscall.Syscall(trap, fd, uintptr(unsafe.Pointer(p)), arg)
		if errno != 0 {
			return errno
		}
		return nil
	})
}
