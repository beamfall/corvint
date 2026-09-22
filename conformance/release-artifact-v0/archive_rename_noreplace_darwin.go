//go:build darwin

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	darwinSysRenameatxNP = uintptr(0x2000000 + 488)
	darwinRenameExcl     = uintptr(0x4)
)

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
	_, _, errno := syscall.Syscall6(darwinSysRenameatxNP, directory.Fd(), uintptr(unsafe.Pointer(oldPointer)), directory.Fd(), uintptr(unsafe.Pointer(newPointer)), darwinRenameExcl, 0)
	runtime.KeepAlive(directory)
	if errno != 0 {
		return errno
	}
	return nil
}
