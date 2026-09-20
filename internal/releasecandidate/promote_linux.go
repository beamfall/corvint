//go:build linux && (amd64 || arm64)

package releasecandidate

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

func promoteNoReplace(parent, oldName, newName string) error {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	oldPointer, err := syscall.BytePtrFromString(oldName)
	if err != nil {
		return err
	}
	newPointer, err := syscall.BytePtrFromString(newName)
	if err != nil {
		return err
	}
	const renameNoReplace = uintptr(1)
	_, _, errno := syscall.Syscall6(linuxRenameat2Trap, directory.Fd(), uintptr(unsafe.Pointer(oldPointer)), directory.Fd(), uintptr(unsafe.Pointer(newPointer)), renameNoReplace, 0)
	runtime.KeepAlive(directory)
	if errno != 0 {
		return errno
	}
	return nil
}
