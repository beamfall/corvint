// SPDX-License-Identifier: Apache-2.0
package main

import (
	"syscall"
	"testing"
)

// DTL-V0-018: pin process identity before killing the parent.
func watchScratchExit(t *testing.T, pid int) func() {
	t.Helper()
	fd, _, errno := syscall.Syscall(434, uintptr(pid), 0, 0) // pidfd_open, all Linux Go architectures
	if errno != 0 {
		t.Fatalf("pidfd_open: %v", errno)
	}
	t.Cleanup(func() { _ = syscall.Close(int(fd)) })
	return func() {
		ep, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
		if err != nil {
			t.Fatal(err)
		}
		err = syscall.EpollCtl(ep, syscall.EPOLL_CTL_ADD, int(fd), &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)})
		if err != nil {
			_ = syscall.Close(ep)
			t.Fatal(err)
		}
		events := make([]syscall.EpollEvent, 1)
		n, err := syscall.EpollWait(ep, events, 5000)
		_ = syscall.Close(ep)
		if err != nil || n != 1 {
			t.Fatalf("bash child survived SIGKILL: events=%d err=%v", n, err)
		}
	}
}
