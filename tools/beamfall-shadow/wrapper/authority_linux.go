//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

const (
	// Linux UAPI prctl operations; syscall omits these constants on amd64.
	prSetChildSubreaper = 36
	prGetChildSubreaper = 37
)

func establishWrapperAuthority() bool {
	if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0, 0, 0, 0); errno != 0 {
		return false
	}
	var enabled int32
	if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prGetChildSubreaper, uintptr(unsafe.Pointer(&enabled)), 0, 0, 0, 0); errno != 0 {
		return false
	}
	return enabled == 1
}
