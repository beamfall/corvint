//go:build darwin || linux

package cli_test

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// survivorRunner owns one started runner process for test cleanup. It never
// signals a process group: the runner is the owner of its command's group
// and retires it on SIGTERM while that group's leader is unreaped. The runner
// itself is signalled only while it is known unreaped: the watcher marks it
// exited, under mu, after waitid reports the exit without reaping and before
// Wait reaps it, and signals are sent under mu only while it is unmarked.
type survivorRunner struct {
	mu      sync.Mutex
	exited  bool
	signal  func(syscall.Signal) error
	done    chan struct{}
	waitErr error
}

// watchSurvivor starts the reap watcher for a started command. The command
// should set WaitDelay, so a descendant that outlives it and keeps a pipe
// open cannot hold Wait.
func watchSurvivor(cmd *exec.Cmd) *survivorRunner {
	pid := cmd.Process.Pid
	s := &survivorRunner{signal: func(sig syscall.Signal) error { return syscall.Kill(pid, sig) }, done: make(chan struct{})}
	go func() {
		_ = exitedUnreaped(pid)
		s.mu.Lock()
		s.exited = true
		s.mu.Unlock()
		s.waitErr = cmd.Wait()
		close(s.done)
	}()
	return s
}

// send signals the runner if it has not exited, and reports whether it did.
func (s *survivorRunner) send(sig syscall.Signal) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exited {
		return false, nil
	}
	return true, s.signal(sig)
}

// retire asks the runner to stop with SIGTERM, which retires its command's
// group, waits grace, then sends SIGKILL and waits bound. A runner already
// exited is never signalled.
func (s *survivorRunner) retire(grace, bound time.Duration) error {
	for _, step := range []struct {
		sig  syscall.Signal
		wait time.Duration
	}{{syscall.SIGTERM, grace}, {syscall.SIGKILL, bound}} {
		if sent, err := s.send(step.sig); !sent {
			break
		} else if err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("signal survivor runner: %w", err)
		}
		select {
		case <-s.done:
			return nil
		case <-time.After(step.wait):
		}
	}
	select {
	case <-s.done:
		return nil
	case <-time.After(bound):
		return fmt.Errorf("survivor runner not reaped within %s of cleanup", bound)
	}
}

// exitedUnreaped blocks until pid exits and leaves it unreaped (waitid
// WEXITED|WNOWAIT), so the pid cannot be reused before Wait reaps it.
func exitedUnreaped(pid int) error {
	var info [128]byte
	for {
		_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(pid),
			uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT), 0, 0)
		if errno == 0 {
			return nil
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

// CAL-V0-130 test hygiene: the survivor cleanup finishes inside its bound
// even when the runner ignores SIGTERM and a descendant keeps its stderr
// open, and a runner already reaped is never signalled again.
func TestCALV0130_SurvivorCleanupSignalsOnlyAnUnreapedRunner(t *testing.T) {
	record := func(s *survivorRunner) *[]syscall.Signal {
		var sent []syscall.Signal
		real := s.signal
		s.signal = func(sig syscall.Signal) error { sent = append(sent, sig); return real(sig) }
		return &sent
	}

	// A runner that ignores SIGTERM, with a background sleep holding stderr.
	// SIGTERM, then SIGKILL, and WaitDelay closing the held pipe end it well
	// before the sleep would.
	stuck := exec.Command("/bin/sh", "-c", `trap "" TERM; sleep 5 & wait`)
	stuck.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stuck.Stderr = new(sliceWriter)
	stuck.WaitDelay = 500 * time.Millisecond
	if err := stuck.Start(); err != nil {
		t.Fatal(err)
	}
	s := watchSurvivor(stuck)
	sent := record(s)
	time.Sleep(100 * time.Millisecond)
	began := time.Now()
	if err := s.retire(500*time.Millisecond, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > 4*time.Second {
		t.Fatalf("cleanup took %s, so it waited on the descendant", took)
	}
	if !slices.Equal(*sent, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}) {
		t.Fatalf("signals sent: %v", *sent)
	}
	// After retirement and reaping, cleanup again sends nothing.
	*sent = nil
	if err := s.retire(time.Second, time.Second); err != nil || len(*sent) != 0 {
		t.Fatalf("cleanup after reaping: %v, signals %v", err, *sent)
	}

	// A runner that finished on its own is never signalled.
	clean := exec.Command("/bin/sh", "-c", "exit 0")
	if err := clean.Start(); err != nil {
		t.Fatal(err)
	}
	c := watchSurvivor(clean)
	sent = record(c)
	<-c.done
	if err := c.retire(time.Second, time.Second); err != nil || len(*sent) != 0 || c.waitErr != nil {
		t.Fatalf("cleanup after a clean exit: %v, signals %v, wait %v", err, *sent, c.waitErr)
	}
}

// sliceWriter is a non-file writer, so exec copies the command's output
// through a pipe that a descendant can hold open.
type sliceWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	return len(p), nil
}
