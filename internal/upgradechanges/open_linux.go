package upgradechanges

import "syscall"

func openRefAt(parent uintptr, name string, flags int) (int, error) {
	return syscall.Openat(int(parent), name, flags, 0)
}
