//go:build linux

package main

import (
	"errors"
	"syscall"
)

func reapWrapperChildren() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if pid > 0 {
			continue
		}
		if errors.Is(err, syscall.ECHILD) || err == nil {
			return
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		return
	}
}
