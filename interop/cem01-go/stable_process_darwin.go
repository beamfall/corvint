//go:build darwin

package main

// Darwin keeper watches and the Darwin step-1 quiet observation for the
// owner/keeper containment in stable_process_unix.go.

import (
	"os"
	"syscall"
)

func stableOwnerSupported() bool { return true }

// stableGroupQuiet is the Darwin step-1 observation: with the leader unreaped,
// the group is quiet once signal 0 no longer succeeds (OQ-12).
func stableGroupQuiet(pgid int) bool { return syscall.Kill(-pgid, 0) != nil }

// stableKeeperOwner watches the keeper's owner, the pid named at spawn, so the
// keeper never outlives it. Control EOF alone is not enough: a copy of the
// owner's control end leaked into any longer-lived process keeps fd 3 open
// and would park the keeper forever.
type stableKeeperOwner struct{ kq, pid int }

// stableKeeperWatchOwner registers the owner's exit; false means the owner is
// already gone, is not the keeper's parent, or cannot be watched, and the
// keeper must not proceed.
func stableKeeperWatchOwner(want int) (stableKeeperOwner, bool) {
	o := stableKeeperOwner{kq: -1, pid: want}
	if want <= 1 || os.Getppid() != want {
		return o, false
	}
	kq, err := syscall.Kqueue()
	if err != nil {
		return o, false
	}
	o.kq = kq
	ev := []syscall.Kevent_t{{Ident: uint64(o.pid), Filter: syscall.EVFILT_PROC, Flags: syscall.EV_ADD, Fflags: syscall.NOTE_EXIT}}
	if _, err := syscall.Kevent(kq, ev, nil, nil); err != nil {
		return o, false
	}
	// The owner was the parent before and after registration, so the watched
	// pid named it throughout: an owner exit reparents the keeper at once.
	return o, os.Getppid() == want
}

// await blocks until the one-shot event (ident, filter) fires, returning
// true, or the owner exits, returning false. Any kqueue failure is treated
// as owner loss so the keeper fails closed.
func (o stableKeeperOwner) await(ident uint64, filter int16, fflags uint32) bool {
	ch := []syscall.Kevent_t{{Ident: ident, Filter: filter, Flags: syscall.EV_ADD | syscall.EV_ONESHOT, Fflags: fflags}}
	if _, err := syscall.Kevent(o.kq, ch, nil, nil); err != nil {
		// A child that has already exited cannot be registered; Wait reaps it.
		return filter == syscall.EVFILT_PROC && err == syscall.ESRCH
	}
	ev := make([]syscall.Kevent_t, 2)
	for {
		n, err := syscall.Kevent(o.kq, nil, ev, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return false
		}
		for _, e := range ev[:n] {
			if e.Filter == syscall.EVFILT_PROC && e.Ident == uint64(o.pid) {
				return false
			}
		}
		if n > 0 {
			return true
		}
	}
}

// awaitControl waits until fd 3 is readable; false means the owner exited.
func (o stableKeeperOwner) awaitControl() bool { return o.await(3, syscall.EVFILT_READ, 0) }

// awaitChild waits until the Git child exits; false means the owner exited.
func (o stableKeeperOwner) awaitChild(pid int) bool {
	return o.await(uint64(pid), syscall.EVFILT_PROC, syscall.NOTE_EXIT)
}
