//go:build darwin || linux

package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/Beamfall/corvint/internal/execfile"
)

// V1-1047: the terminal harness sends every group signal (and SIGWINCH)
// while its child is still unreaped. The host exits at once, so the old
// order reaped it first and then sent the deferred group SIGTERM and the
// final group SIGKILL to a group ID that could have been reused.
func TestV11047TerminalSignalsOnlyAnUnreapedChild(t *testing.T) {
	sent, afterReap := runRecordedTerminal(t)
	if len(afterReap) != 0 {
		t.Fatalf("signals %v were sent after the child was reaped (all: %v)", afterReap, sent)
	}
	if len(sent) < 2 {
		t.Fatalf("cleanup sent %v, want the group SIGTERM and SIGKILL", sent)
	}
}

// V1-1047 review P1: an exit observation that fails with ECHILD (another
// reaper collected the child) unpins the PID, so the harness sends no group
// signal at all.
func TestV11047TerminalFailedObservationSendsNoSignal(t *testing.T) {
	previous := observeTerminalExit
	t.Cleanup(func() { observeTerminalExit = previous })
	observeTerminalExit = func(pid int) error {
		if err := previous(pid); err != nil {
			return err
		}
		return syscall.ECHILD
	}
	if sent, _ := runRecordedTerminal(t); len(sent) != 0 {
		t.Fatalf("signals %v sent after a failed exit observation", sent)
	}
}

// runRecordedTerminal runs a host that exits at once and returns every
// signal sent and those sent after the child was reaped.
func runRecordedTerminal(t *testing.T) (sent, afterReap []syscall.Signal) {
	t.Helper()
	dir := t.TempDir()
	host := filepath.Join(dir, "host")
	if err := execfile.WriteFile(host, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "terminal.json")
	if err := writeJSON(config, Object{"host": host, "directory": dir, "env": []any{}}); err != nil {
		t.Fatal(err)
	}
	stdin, hold, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousStdin, previousSignal := os.Stdin, terminalSignal
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin, terminalSignal = previousStdin, previousSignal
		_ = hold.Close()
	})
	var mu sync.Mutex
	terminalSignal = func(pid int, signal syscall.Signal) error {
		mu.Lock()
		sent = append(sent, signal)
		if reaped(abs(pid)) {
			afterReap = append(afterReap, signal)
		}
		mu.Unlock()
		return previousSignal(pid, signal)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := runTerminal(ctx, config); err != nil {
		t.Fatalf("runTerminal: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return sent, afterReap
}

// reaped reports that pid is no longer this process's unreaped child:
// waitid without reaping answers ECHILD.
func reaped(pid int) bool {
	var info [128]byte
	_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(pid),
		uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT|syscall.WNOHANG), 0, 0)
	return errno == syscall.ECHILD
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
