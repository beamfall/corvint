//go:build darwin || linux

package testsupport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// starvedHold is how long the output reader stays blocked after the child's
// exit is observed: several times the old one-second pipe-drain bound.
const starvedHold = 3 * time.Second

// CheckPipeDrainBound exercises a production pipe-drain configuration
// (V1-0391). configure must set the bound the production call site uses.
// It proves that the bound is finite and above a reader stall, that a child
// exiting 0 while its reader is stalled after exit keeps its output, that
// the old one-second bound fails the same run (negative control), and that a
// held pipe still fails the call once a finite bound expires.
func CheckPipeDrainBound(t *testing.T, configure func(*exec.Cmd)) {
	t.Helper()
	probe := exec.CommandContext(context.Background(), "/bin/true")
	configure(probe)
	if probe.WaitDelay <= starvedHold {
		t.Fatalf("pipe-drain bound %s is unbounded or below a %s reader stall", probe.WaitDelay, starvedHold)
	}
	if out, err := runStarved(t, configure, 0); err != nil || out != "ok" {
		t.Fatalf("stalled reader failed a successful child: out=%q err=%v", out, err)
	}
	if _, err := runStarved(t, configure, time.Second); !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("negative control: one-second bound returned %v, want exec.ErrWaitDelay", err)
	}
	checkHeldPipe(t, configure)
}

// gatedWriter blocks every Write until release is closed.
type gatedWriter struct {
	release chan struct{}
	buf     bytes.Buffer
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	<-w.release
	return w.buf.Write(p)
}

// runStarved starts the child, observes its exit without reaping it, and
// only then calls Wait, so the drain timer starts no earlier than the
// stall. The reader stays blocked for starvedHold after Wait is entered.
func runStarved(t *testing.T, configure func(*exec.Cmd), override time.Duration) (string, error) {
	t.Helper()
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "printf ok")
	configure(command)
	if override != 0 {
		command.WaitDelay = override
	}
	out := &gatedWriter{release: make(chan struct{})}
	command.Stdout = out
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if err := exitedUnreaped(command.Process.Pid); err != nil {
		close(out.release)
		_ = command.Wait()
		t.Fatalf("observe child exit: %v", err)
	}
	timer := time.AfterFunc(starvedHold, func() { close(out.release) })
	defer timer.Stop()
	err := command.Wait()
	return out.buf.String(), err
}

// checkHeldPipe: a descendant keeps stdout open after the leader exits; with
// a finite (injected, short) bound the call still fails with ErrWaitDelay.
func checkHeldPipe(t *testing.T, configure func(*exec.Cmd)) {
	t.Helper()
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "sleep 30 & echo $! > '"+pidPath+"'; printf ok")
	configure(command)
	command.WaitDelay = 200 * time.Millisecond
	var out bytes.Buffer
	command.Stdout = &out
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 1 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		if command.SysProcAttr != nil && command.SysProcAttr.Setpgid {
			_ = syscall.Kill(-leader, syscall.SIGKILL)
		}
	})
	began := time.Now()
	err := command.Wait()
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("held pipe returned %v, want exec.ErrWaitDelay", err)
	}
	if elapsed := time.Since(began); elapsed > 20*time.Second {
		t.Fatalf("held pipe failed only after %s", elapsed)
	}
}

// exitedUnreaped blocks until the process exits and leaves it unreaped
// (waitid WEXITED|WNOWAIT), so a later Wait still reaps it.
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
