//go:build linux && (amd64 || arm64)

package main

// Linux keeper watches and the Linux step-1 quiet observation for the
// owner/keeper containment in stable_process_unix.go.
//
// The keeper watches its owner and its Git child through pidfds (Linux 5.3)
// in one epoll set, the Linux counterpart of the Darwin kqueue watch.
//
// Linux answers signal 0 successfully for a group that holds only zombies,
// including the owner's own unreaped keeper, so signal 0 cannot end step 1.
// Step 1 instead reads /proc while the keeper is still unreaped and so pins
// its PID as the group ID: the keeper must be a zombie child of this owner
// leading the group, and every process listed with that group must be a
// zombie with no remaining thread. After the group SIGKILL every member is
// dying and Linux refuses fork to a task with a pending fatal signal, so a
// single listing sees every member that outlives it. Any read failure or
// exceeded bound is "not quiet", which ends in HOLD at the caller's bound.
// Step 2 is unchanged: reap, then only signal-0 ESRCH proves absence.

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"syscall"
)

const (
	// stableSysPidfdOpen is pidfd_open(2), number 434 on linux/amd64 and
	// linux/arm64; the build constraint keeps other architectures refused.
	stableSysPidfdOpen  = 434
	stableProcStatMax   = 4096
	stableProcStatusMax = 16384
	stableProcScanMax   = 1 << 22
)

func stablePidfdOpen(pid int) (int, error) {
	fd, _, e := syscall.Syscall(stableSysPidfdOpen, uintptr(pid), 0, 0)
	if e != 0 {
		return -1, e
	}
	return int(fd), nil
}

// stableOwnerSupported probes the primitives the Linux route depends on, so a
// kernel without pidfds, a host without a readable /proc, or an owner running
// as PID 1 (whose keeper could not watch it) refuses before admission or any
// spawn with unsupported-process-containment.
func stableOwnerSupported() bool {
	me := os.Getpid()
	if me <= 1 {
		return false
	}
	fd, err := stablePidfdOpen(me)
	if err != nil {
		return false
	}
	_ = syscall.Close(fd)
	ep, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return false
	}
	_ = syscall.Close(ep)
	st, found, ok := stableProcStat(me)
	return ok && found && st.ppid == os.Getppid() && st.pgrp == syscall.Getpgrp()
}

type stableProcEntry struct {
	state      byte
	ppid, pgrp int
}

// stableProcRead reads at most limit bytes of /proc/<pid>/<name>. found is
// false when the process is gone; ok is false on any other failure or when
// the file exceeds limit.
func stableProcRead(pid int, name string, limit int) (b []byte, found, ok bool) {
	f, err := os.Open("/proc/" + strconv.Itoa(pid) + "/" + name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			return nil, false, true
		}
		return nil, false, false
	}
	defer f.Close()
	b, err = io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if errors.Is(err, syscall.ESRCH) {
		return nil, false, true
	}
	if err != nil || len(b) > limit {
		return nil, false, false
	}
	return b, true, true
}

// stableProcStat parses state, ppid and pgrp from /proc/<pid>/stat. The
// command name may hold any byte, so fields start after its last ')'.
func stableProcStat(pid int) (stableProcEntry, bool, bool) {
	b, found, ok := stableProcRead(pid, "stat", stableProcStatMax)
	if !ok || !found {
		return stableProcEntry{}, found, ok
	}
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return stableProcEntry{}, true, false
	}
	f := bytes.Fields(b[i+1:])
	if len(f) < 3 || len(f[0]) != 1 {
		return stableProcEntry{}, true, false
	}
	ppid, e1 := strconv.Atoi(string(f[1]))
	pgrp, e2 := strconv.Atoi(string(f[2]))
	if e1 != nil || e2 != nil {
		return stableProcEntry{}, true, false
	}
	return stableProcEntry{state: f[0][0], ppid: ppid, pgrp: pgrp}, true, true
}

// stableProcThreads reads the Threads: count of /proc/<pid>/status. A zombie
// thread-group leader can still have running threads; only a count of one
// (the zombie leader itself) is quiet.
func stableProcThreads(pid int) (int, bool, bool) {
	b, found, ok := stableProcRead(pid, "status", stableProcStatusMax)
	if !ok || !found {
		return 0, found, ok
	}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if v, cut := bytes.CutPrefix(line, []byte("Threads:")); cut {
			n, err := strconv.Atoi(string(bytes.TrimSpace(v)))
			return n, true, err == nil && n >= 0
		}
	}
	return 0, true, false
}

func stableProcDead(state byte) bool { return state == 'Z' || state == 'X' }

// stableGroupQuiet is the Linux step-1 observation for the group led by the
// owner's unreaped child pgid. It sends no signal.
func stableGroupQuiet(pgid int) bool {
	me := os.Getpid()
	leader := func() bool {
		e, found, ok := stableProcStat(pgid)
		return ok && found && stableProcDead(e.state) && e.ppid == me && e.pgrp == pgid
	}
	if !leader() {
		return false
	}
	d, err := os.Open("/proc")
	if err != nil {
		return false
	}
	defer d.Close()
	seen, count := false, 0
	for {
		names, err := d.Readdirnames(512)
		for _, n := range names {
			if count++; count > stableProcScanMax {
				return false
			}
			pid, perr := strconv.Atoi(n)
			if perr != nil || pid <= 0 || strconv.Itoa(pid) != n {
				continue
			}
			e, found, ok := stableProcStat(pid)
			if !ok {
				return false
			}
			if !found || e.pgrp != pgid {
				continue
			}
			if !stableProcDead(e.state) {
				return false
			}
			threads, found, ok := stableProcThreads(pid)
			if !ok || found && threads > 1 {
				return false
			}
			if pid == pgid {
				seen = true
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
	}
	return seen && leader()
}

// stableKeeperOwner watches the keeper's owner, the pid named at spawn, so the
// keeper never outlives it. Control EOF alone is not enough: a copy of the
// owner's control end leaked into any longer-lived process keeps fd 3 open
// and would park the keeper forever.
type stableKeeperOwner struct{ ep, pidfd, pid int }

// stableKeeperWatchOwner registers the owner's exit; false means the owner is
// already gone, is not the keeper's parent, or cannot be watched, and the
// keeper must not proceed. The pidfd and the epoll set are close-on-exec, so
// Git inherits neither.
func stableKeeperWatchOwner(want int) (stableKeeperOwner, bool) {
	o := stableKeeperOwner{ep: -1, pidfd: -1, pid: want}
	if want <= 1 || os.Getppid() != want {
		return o, false
	}
	fd, err := stablePidfdOpen(o.pid)
	if err != nil {
		return o, false
	}
	o.pidfd = fd
	ep, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return o, false
	}
	o.ep = ep
	if syscall.EpollCtl(ep, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}) != nil {
		return o, false
	}
	// The pidfd names whichever process held the pid when it was opened. The
	// owner was the parent before and after the open, and its exit reparents
	// the keeper before the pid can be reused, so the pidfd names the owner.
	return o, os.Getppid() == want
}

// await blocks until fd is readable, returning true, or the owner exits,
// returning false. Any epoll failure is treated as owner loss so the keeper
// fails closed.
func (o stableKeeperOwner) await(fd int) bool {
	if syscall.EpollCtl(o.ep, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(fd)}) != nil {
		return false
	}
	defer syscall.EpollCtl(o.ep, syscall.EPOLL_CTL_DEL, fd, nil)
	ev := make([]syscall.EpollEvent, 2)
	for {
		n, err := syscall.EpollWait(o.ep, ev, -1)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return false
		}
		for _, e := range ev[:n] {
			if int(e.Fd) == o.pidfd {
				return false
			}
		}
		if n > 0 {
			return true
		}
	}
}

// awaitControl waits until fd 3 is readable; false means the owner exited.
func (o stableKeeperOwner) awaitControl() bool { return o.await(3) }

// awaitChild waits until the Git child exits; false means the owner exited.
// The child is unreaped, so its pid still names it when the pidfd is opened.
func (o stableKeeperOwner) awaitChild(pid int) bool {
	fd, err := stablePidfdOpen(pid)
	if err != nil {
		return false
	}
	defer syscall.Close(fd)
	return o.await(fd)
}
