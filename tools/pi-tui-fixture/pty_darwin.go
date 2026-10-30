package main

import (
	"bytes"
	"syscall"
	"unsafe"
)

// unlockPTY grants and unlocks the master's replica and returns its device path.
func unlockPTY(master uintptr) (string, error) {
	if err := ioctl(master, syscall.TIOCPTYGRANT, nil); err != nil {
		return "", err
	}
	if err := ioctl(master, syscall.TIOCPTYUNLK, nil); err != nil {
		return "", err
	}
	var name [128]byte
	if err := ioctl(master, syscall.TIOCPTYGNAME, unsafe.Pointer(&name[0])); err != nil {
		return "", err
	}
	return string(name[:bytes.IndexByte(name[:], 0)]), nil
}
