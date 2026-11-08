// SPDX-License-Identifier: Apache-2.0
//go:build linux && amd64

// RSP-V0-020, RSP-V0-021 and RSP-V0-022: unprivileged filtered process-tree tracing.
package proctrace

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type platformBackend struct{}

const (
	ptraceOptions = syscall.PTRACE_O_TRACESYSGOOD | syscall.PTRACE_O_TRACEFORK | syscall.PTRACE_O_TRACEVFORK | syscall.PTRACE_O_TRACECLONE | syscall.PTRACE_O_TRACEEXEC | 0x80 | 0x100000 // TRACESECCOMP, EXITKILL
	waitThread    = 0x20000000                                                                                                                                                            // __WNOTHREAD: never reap another caller's children
	seccompEvent  = 7
)

func (platformBackend) Trace(ctx context.Context, q Request) (r Result) {
	start := time.Now()
	r = emptyResult("linux-amd64-ptrace-seccomp/1")
	defer func() { r.Duration = time.Since(start) }()
	if reason := validate(q); reason != "" {
		r.unavailable(Incomplete, reason)
		return
	}
	if ctx == nil {
		r.unavailable(Incomplete, "missing-context")
		return
	}
	if ctx.Err() != nil {
		r.unavailable(Incomplete, "cancelled")
		return
	}
	// Canonical roots are required: silently interpreting a symlinked root would
	// assign an observation to the wrong repository.
	for _, repo := range q.Repositories {
		resolved, err := filepathRoot(repo.Root)
		info, statErr := os.Stat(repo.Root)
		if err != nil || resolved != repo.Root || statErr != nil || !info.IsDir() {
			r.unavailable(Incomplete, "noncanonical-repository")
			return
		}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cmd := exec.Command(q.Executable, q.Args...)
	cmd.Dir = q.Dir
	cmd.Env = q.Env
	cmd.Stdin = q.Stdin
	cmd.Stdout = q.Stdout
	cmd.Stderr = q.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Ptrace: true, Setpgid: true}
	if err := cmd.Start(); err != nil {
		r.Outcome.Error = err.Error()
		r.unavailable(Incomplete, "trace-start-refused")
		return
	}
	r.Outcome.Started = true
	root := cmd.Process.Pid
	var mu sync.Mutex
	live := map[int]bool{root: true}
	kill := func() {
		mu.Lock()
		defer mu.Unlock()
		for pid := range live {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		_ = syscall.Kill(-root, syscall.SIGKILL)
	}
	stop := context.AfterFunc(ctx, kill)
	defer stop()
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(root, &status, syscall.WALL, nil); err != nil || !status.Stopped() {
		kill()
		_ = cmd.Wait()
		r.unavailable(Incomplete, "initial-stop-missing")
		return
	}
	c := collector{q: q, r: &r, paths: map[Path]bool{}, created: map[string]bool{}}
	c.inheritedFiles(root)
	// Startup exec/chdir happen before syscall observation. Apply the same
	// closed path walker and matching checks as later pathname accesses.
	for _, name := range []string{q.Executable, q.Dir} {
		c.linkDependencies(name)
		c.checkMatching(name)
		if target, err := resolve(name); err == nil {
			c.checkMatching(target)
		}
	}
	rootExecutable, resolveErr := resolve(q.Executable)
	if resolveErr != nil {
		r.unavailable(Incomplete, "executable-resolution-failed")
	} else {
		c.recordExec(q.Executable, rootExecutable)
	}
	if err := syscall.PtraceSetOptions(root, ptraceOptions); err != nil {
		r.unavailable(Incomplete, "ptrace-options-refused")
		kill()
	} else if err := installFilter(root); err != nil {
		r.unavailable(Incomplete, "seccomp-install-refused")
		kill()
	} else if err := syscall.PtraceCont(root, 0); err != nil {
		r.unavailable(Incomplete, "ptrace-resume-failed")
		kill()
	}
	pending := map[int]*call{}
	newborn := map[int]bool{}
	for {
		mu.Lock()
		remaining := len(live)
		mu.Unlock()
		if remaining == 0 {
			break
		}
		pid, err := syscall.Wait4(-1, &status, syscall.WALL|waitThread, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			r.unavailable(Incomplete, "wait-failed")
			kill()
			break
		}
		if status.Exited() || status.Signaled() {
			if pid == root {
				r.Outcome.ExitCode = status.ExitStatus()
				if status.Signaled() {
					r.Outcome.Signal = int(status.Signal())
				}
			}
			mu.Lock()
			delete(live, pid)
			mu.Unlock()
			delete(pending, pid)
			delete(newborn, pid)
			continue
		}
		if !status.Stopped() {
			r.unavailable(Incomplete, "unexpected-wait-status")
			continue
		}
		signal := 0
		event := status.TrapCause()
		switch {
		case event == syscall.PTRACE_EVENT_FORK || event == syscall.PTRACE_EVENT_VFORK || event == syscall.PTRACE_EVENT_CLONE:
			child, e := syscall.PtraceGetEventMsg(pid)
			if e != nil {
				r.unavailable(Incomplete, "child-identity-missing")
				kill()
			} else {
				mu.Lock()
				live[int(child)] = true
				mu.Unlock()
				newborn[int(child)] = true
				mu.Lock()
				overCap := len(live) > q.EventCap
				mu.Unlock()
				if overCap {
					r.unavailable(Incomplete, "process-cap")
					kill()
				}
				// Concurrent descendants can mutate path arguments or filesystem names.
				r.unavailable(Incomplete, "concurrent-filesystem-path-race")
				// Shared address spaces can mutate path arguments while a sibling is stopped.
				// Keep their observations, but never claim an atomic path snapshot.
				if event == syscall.PTRACE_EVENT_CLONE {
					r.unavailable(Incomplete, "shared-memory-path-race")
				}
			}
		case event == syscall.PTRACE_EVENT_EXEC:
			old, e := syscall.PtraceGetEventMsg(pid)
			if e != nil {
				r.unavailable(Incomplete, "exec-identity-missing")
			} else if int(old) != pid {
				mu.Lock()
				delete(live, int(old))
				live[pid] = true
				mu.Unlock()
				delete(pending, int(old))
				r.unavailable(Incomplete, "thread-exec-identity-change")
			}
			if p := pending[pid]; p != nil {
				c.finish(p, 0)
				delete(pending, pid)
			}
		case event == seccompEvent:
			r.Events++
			if r.Events > q.EventCap {
				r.unavailable(Incomplete, "event-cap")
				delete(pending, pid)
				break
			}
			var regs syscall.PtraceRegs
			if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
				r.unavailable(Incomplete, "register-read-failed")
				break
			}
			p := c.enter(pid, regs)
			pending[pid] = p
		case status.StopSignal() == syscall.Signal(int(syscall.SIGTRAP)|0x80):
			if p := pending[pid]; p != nil {
				var regs syscall.PtraceRegs
				if e := syscall.PtraceGetRegs(pid, &regs); e != nil {
					r.unavailable(Incomplete, "register-read-failed")
				} else {
					c.finish(p, int64(regs.Rax))
				}
				delete(pending, pid)
			} else {
				r.unavailable(Incomplete, "unexpected-syscall-stop")
			}
		case status.StopSignal() == syscall.SIGSTOP && newborn[pid]:
			delete(newborn, pid)
		default:
			// Deliver the tracee's actual signals, including SIGTRAP. Swallowing signals
			// changes outcomes. Group stops cannot be represented by this backend.
			signal = int(status.StopSignal())
			if signal == int(syscall.SIGSTOP) || signal == int(syscall.SIGTSTP) || signal == int(syscall.SIGTTIN) || signal == int(syscall.SIGTTOU) {
				r.unavailable(Incomplete, "group-stop")
			}
		}
		var resume error
		if pending[pid] != nil {
			resume = syscall.PtraceSyscall(pid, signal)
		} else {
			resume = syscall.PtraceCont(pid, signal)
		}
		if resume != nil && resume != syscall.ESRCH {
			r.unavailable(Incomplete, "ptrace-resume-failed")
			kill()
		}
	}
	// Wait closes exec.Cmd's copy pipes. Wait4 above retained the actual outcome.
	_ = cmd.Wait()
	if ctx.Err() != nil {
		r.unavailable(Incomplete, "cancelled")
	}
	return
}

func filepathRoot(root string) (string, error) { return resolve(root) }

// safeCalls is a closed allowlist of process-local memory, descriptor lifetime,
// signal setup, identity, time and exit operations. No pathname, peer selection,
// descriptor readiness/control, or extensible operation selector bypasses enter.
// New syscalls and new forms of multiplexed calls must be proved separately.
var safeCalls = []uint32{
	syscall.SYS_CLOSE, syscall.SYS_DUP, syscall.SYS_DUP2,
	syscall.SYS_MPROTECT, syscall.SYS_MUNMAP, syscall.SYS_BRK, syscall.SYS_MREMAP,
	syscall.SYS_RT_SIGACTION, syscall.SYS_RT_SIGPROCMASK, syscall.SYS_RT_SIGRETURN,
	syscall.SYS_SIGALTSTACK, syscall.SYS_PAUSE, syscall.SYS_NANOSLEEP,
	syscall.SYS_SCHED_YIELD, syscall.SYS_GETPID, syscall.SYS_GETTID,
	syscall.SYS_GETUID, syscall.SYS_GETGID, syscall.SYS_GETEUID, syscall.SYS_GETEGID,
	syscall.SYS_GETPPID, syscall.SYS_GETPGRP, syscall.SYS_GETGROUPS,
	syscall.SYS_GETRESUID, syscall.SYS_GETRESGID,
	syscall.SYS_GETTIMEOFDAY, syscall.SYS_TIME, syscall.SYS_GETRLIMIT,
	syscall.SYS_EXIT, syscall.SYS_EXIT_GROUP, syscall.SYS_WAIT4,
}

// installFilter patches only the initial exec stop, restoring instructions,
// registers and stack bytes before the first user instruction. No helper binary,
// process-global init hook, cgo, daemon or privilege is needed.
func installFilter(pid int) (resultErr error) {
	var original syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(pid, &original); err != nil {
		return err
	}
	code := make([]byte, 8)
	if _, err := syscall.PtracePeekText(pid, uintptr(original.Rip), code); err != nil {
		return err
	}
	defer func() {
		_, e := syscall.PtracePokeText(pid, uintptr(original.Rip), code)
		resultErr = errors.Join(resultErr, e)
	}()
	defer func() { resultErr = errors.Join(resultErr, syscall.PtraceSetRegs(pid, &original)) }()
	// sock_filter: load arch; wrong ABI traces and is rejected by enter;
	// load syscall number; allow explicitly harmless calls; trace everything else.
	var filter []byte
	ins := func(op uint16, jt, jf byte, k uint32) {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint16(b, op)
		b[2] = jt
		b[3] = jf
		binary.LittleEndian.PutUint32(b[4:], k)
		filter = append(filter, b...)
	}
	ins(0x20, 0, 0, 4)
	ins(0x15, 1, 0, 0xc000003e)
	ins(0x06, 0, 0, 0x7ff00000)
	ins(0x20, 0, 0, 0)
	for _, n := range safeCalls {
		ins(0x15, 0, 1, n)
		ins(0x06, 0, 0, 0x7fff0000)
	}
	ins(0x06, 0, 0, 0x7ff00000)
	addr := (original.Rsp - uint64(len(filter)) - 512) &^ 15
	scratch := make([]byte, len(filter)+16)
	if _, err := syscall.PtracePeekData(pid, uintptr(addr), scratch); err != nil {
		return err
	}
	defer func() {
		_, e := syscall.PtracePokeData(pid, uintptr(addr), scratch)
		resultErr = errors.Join(resultErr, e)
	}()
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data, uint16(len(filter)/8))
	binary.LittleEndian.PutUint64(data[8:], addr+16)
	data = append(data, filter...)
	if _, err := syscall.PtracePokeData(pid, uintptr(addr), data); err != nil {
		return err
	}
	injected := append([]byte{0x0f, 0x05, 0xcc}, code[3:]...) // syscall; int3
	if _, err := syscall.PtracePokeText(pid, uintptr(original.Rip), injected); err != nil {
		return err
	}
	invoke := func(n, a, b, c uint64) error {
		regs := original
		regs.Rax = n
		regs.Orig_rax = ^uint64(0)
		regs.Rdi = a
		regs.Rsi = b
		regs.Rdx = c
		if err := syscall.PtraceSetRegs(pid, &regs); err != nil {
			return err
		}
		if err := syscall.PtraceCont(pid, 0); err != nil {
			return err
		}
		var status syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &status, syscall.WALL, nil); err != nil {
			return err
		}
		if !status.Stopped() || status.StopSignal() != syscall.SIGTRAP || status.TrapCause() != 0 {
			return fmt.Errorf("injection stop: %v", status)
		}
		if err := syscall.PtraceGetRegs(pid, &regs); err != nil {
			return err
		}
		if regs.Rax != 0 {
			return fmt.Errorf("injection errno: %d", int64(regs.Rax))
		}
		return nil
	}
	if err := invoke(157, 38, 1, 0); err != nil {
		return err
	} // PR_SET_NO_NEW_PRIVS
	return invoke(317, 1, 0, addr) // SECCOMP_SET_MODE_FILTER
}
func procPath(pid int, suffix string) string { return "/proc/" + strconv.Itoa(pid) + "/" + suffix }
