// SPDX-License-Identifier: Apache-2.0
package main

import (
	"syscall"
	"testing"
)

// DTL-V0-018: register both live processes before killing the invoking parent.
// NOTE_EXIT observes exit independently of when launchd reaps orphaned children.
func watchScratchExit(t *testing.T, pid int) func() {
	t.Helper()
	kq, err := syscall.Kqueue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Close(kq) })
	change := syscall.Kevent_t{Ident: uint64(pid), Filter: syscall.EVFILT_PROC,
		Flags: syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_ONESHOT, Fflags: syscall.NOTE_EXIT}
	if _, err := syscall.Kevent(kq, []syscall.Kevent_t{change}, nil, nil); err != nil {
		t.Fatal(err)
	}
	return func() {
		events := make([]syscall.Kevent_t, 1)
		timeout := syscall.Timespec{Sec: 5}
		n, err := syscall.Kevent(kq, nil, events, &timeout)
		if err != nil || n != 1 {
			t.Fatalf("bash child survived SIGKILL: events=%d err=%v", n, err)
		}
		event := events[0]
		if event.Flags&syscall.EV_ERROR != 0 || event.Ident != uint64(pid) || event.Filter != syscall.EVFILT_PROC || event.Fflags&syscall.NOTE_EXIT == 0 {
			t.Fatalf("want process exit for %d, got %+v", pid, event)
		}
	}
}
