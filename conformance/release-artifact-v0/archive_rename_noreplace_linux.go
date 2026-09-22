//go:build linux && (amd64 || arm64)

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const linuxRenameNoReplace = uintptr(1)

func renameRootNoReplace(root *os.Root, oldName, newName string) error {
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
	_, _, errno := syscall.Syscall6(linuxRenameat2Trap, directory.Fd(), uintptr(unsafe.Pointer(oldPointer)), directory.Fd(), uintptr(unsafe.Pointer(newPointer)), linuxRenameNoReplace, 0)
	runtime.KeepAlive(directory)
	if errno != 0 {
		return errno
	}
	return nil
}
