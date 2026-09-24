package main

import (
	"strconv"
	"syscall"
	"unsafe"
)

// unlockPTY unlocks the master's replica and returns its device path.
func unlockPTY(master uintptr) (string, error) {
	var unlock int32
	if err := ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		return "", err
	}
	var number uint32
	if err := ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&number)); err != nil {
		return "", err
	}
	return "/dev/pts/" + strconv.FormatUint(uint64(number), 10), nil
}
