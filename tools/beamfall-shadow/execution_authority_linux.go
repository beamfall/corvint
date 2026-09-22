//go:build linux

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var linuxPrctl = syscall.Syscall6

const (
	// Linux UAPI prctl operations; syscall omits these constants on amd64.
	prSetChildSubreaper = 36
	prGetChildSubreaper = 37
)

func establishExecutionAuthority() error {
	if _, _, errno := linuxPrctl(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0, 0, 0, 0); errno != 0 {
		return fmt.Errorf("set child subreaper: %w", errno)
	}
	enabled, err := linuxSubreaperEnabled()
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("child subreaper is not enabled")
	}
	return nil
}

func linuxSubreaperEnabled() (bool, error) {
	var enabled int32
	if _, _, errno := linuxPrctl(syscall.SYS_PRCTL, prGetChildSubreaper, uintptr(unsafe.Pointer(&enabled)), 0, 0, 0, 0); errno != 0 {
		return false, fmt.Errorf("get child subreaper: %w", errno)
	}
	return enabled == 1, nil
}

func processContainmentLabel() string { return "PROCESS_GROUP_AND_LINUX_SUBREAPER" }
