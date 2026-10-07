//go:build darwin || linux

package cli_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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
	record := recordSignals

	// A runner that ignores SIGTERM, with a background sleep holding stderr.
	// SIGTERM, then SIGKILL, and WaitDelay closing the held pipe end it well
	// before the sleep would. The runner reports ready only after its trap is
	// installed and the sleep started, and nothing is signalled before that.
	ready := filepath.Join(t.TempDir(), "ready")
	stuck := exec.Command("/bin/sh", "-c", `trap "" TERM; sleep 5 & echo ready > "$1"; wait`, "stuck", ready)
	stuck.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stuck.Stderr = new(sliceWriter)
	stuck.WaitDelay = 500 * time.Millisecond
	if err := stuck.Start(); err != nil {
		t.Fatal(err)
	}
	s := watchSurvivor(stuck)
	sent := record(s)
	waitForLine(t, ready, 10*time.Second)
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

// CAL-V0-130 test hygiene: the upgrade test's fixture command ends by itself
// when its runner was SIGKILLed before retiring it, once its lifetime bound
// passes or once its directory is removed, as failed-test cleanup does.
func TestCALV0130_SurvivorFixtureOutlivesNoKilledRunner(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seconds string
		remove  bool
		within  time.Duration
	}{
		{"lifetime bound", "3", false, 10 * time.Second},
		{"directory removed", "120", true, 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fixtureDir := filepath.Join(dir, "fixture")
			if err := os.Mkdir(fixtureDir, 0o700); err != nil {
				t.Fatal(err)
			}
			started, ready := filepath.Join(fixtureDir, "started"), filepath.Join(dir, "ready")
			// Exit is detected by EOF on a pipe whose write end only the
			// runner and the fixture hold (as fd 3), not by the pid
			// disappearing: an orphaned fixture that exits can stay a zombie
			// on a host whose adopter does not reap.
			exitRead, exitWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			// The stand-in runner ignores SIGTERM, starts the fixture
			// exactly as the upgrade test's runner does, and is SIGKILLed,
			// which closes its copy of fd 3. It leads process group G, and
			// the fixture and every child of either stay in G.
			runnerCmd := exec.Command("/bin/sh", "-c", `trap "" TERM; /bin/sh -c "$1" survivor "$2" "$3" "$4" "$5" & echo ready > "$6"; wait`,
				"runner", survivorFixture, started, filepath.Join(fixtureDir, "finish"), fixtureDir, tc.seconds, ready)
			runnerCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			runnerCmd.ExtraFiles = []*os.File{exitWrite}
			err = runnerCmd.Start()
			exitWrite.Close()
			if err != nil {
				exitRead.Close()
				t.Fatal(err)
			}
			exited := make(chan struct{})
			go func() {
				_, _ = io.Copy(io.Discard, exitRead)
				close(exited)
			}()
			// One cleanup owns G: it kills the whole group, then waits for
			// EOF. G stays pinned while any member lives; only once every
			// member is gone could G be reused, the stated residual.
			group := runnerCmd.Process.Pid
			t.Cleanup(func() {
				defer exitRead.Close()
				if err := syscall.Kill(-group, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					t.Errorf("kill fixture group %d: %v", group, err)
				}
				select {
				case <-exited:
				case <-time.After(5 * time.Second):
					t.Errorf("fixture group %d still holds fd 3 5s after cleanup killed it", group)
				}
			})
			runner := watchSurvivor(runnerCmd)
			sent := recordSignals(runner)
			pid, err := strconv.Atoi(strings.TrimSpace(waitForLine(t, started, 10*time.Second)))
			if err != nil {
				t.Fatal(err)
			}
			waitForLine(t, ready, 10*time.Second)
			// SIGKILL the runner only, never G, so the fixture must outlive it.
			if err := runner.retire(200*time.Millisecond, 5*time.Second); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(*sent, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}) {
				t.Fatalf("signals sent: %v", *sent)
			}
			select {
			case <-exited:
				t.Fatal("fixture already gone before its bound")
			default:
			}
			if tc.remove {
				if err := os.RemoveAll(fixtureDir); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-exited:
			case <-time.After(tc.within):
				t.Fatalf("fixture loop %d still running %s after its runner was killed", pid, tc.within)
			}
		})
	}
}

// waitForLine waits at most bound for path to hold a complete line.
func waitForLine(t *testing.T, path string, bound time.Duration) string {
	t.Helper()
	for deadline := time.Now().Add(bound); ; time.Sleep(10 * time.Millisecond) {
		if raw, err := os.ReadFile(path); err == nil && strings.HasSuffix(string(raw), "\n") {
			return string(raw)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s not written within %s", path, bound)
		}
	}
}

// recordSignals wraps a runner's kill so the signals cleanup sends are kept.
func recordSignals(s *survivorRunner) *[]syscall.Signal {
	var sent []syscall.Signal
	real := s.signal
	s.signal = func(sig syscall.Signal) error { sent = append(sent, sig); return real(sig) }
	return &sent
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
