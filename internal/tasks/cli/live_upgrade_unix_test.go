//go:build darwin || linux

package cli_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// commandGroup is the process group of pid, refused when it is the test's
// own group, which cleanup must never signal.
func commandGroup(pid int) (int, error) {
	group, err := syscall.Getpgid(pid)
	if err != nil {
		return 0, err
	}
	if group == syscall.Getpgrp() {
		return 0, fmt.Errorf("process %d shares the test's process group %d", pid, group)
	}
	return group, nil
}

// retireSurvivor ends a survivor runner from test cleanup without hanging the
// package. A runner already reaped is left alone, so a recycled group ID is
// never signalled. Otherwise the command's process group, every member and
// descendant that stayed in it, is killed, then the runner itself, and the
// reap is waited for at most bound.
func retireSurvivor(runner *os.Process, group int, done chan error, bound time.Duration) error {
	select {
	case err := <-done:
		done <- err
		return nil
	default:
	}
	if group > 0 {
		if err := syscall.Kill(-group, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("kill process group %d: %w", group, err)
		}
	}
	_ = runner.Kill()
	select {
	case err := <-done:
		done <- err
		return nil
	case <-time.After(bound):
		return fmt.Errorf("survivor runner not reaped within %s of cleanup", bound)
	}
}

// CAL-V0-130 test hygiene: the survivor cleanup finishes inside its bound
// even when a descendant outlives the killed command and holds stderr open.
// Killing only the command's leader leaves Wait blocked on the pipe; retiring
// the recorded process group ends it.
func TestCALV0130_SurvivorCleanupRetiresTheCommandGroup(t *testing.T) {
	start := func(t *testing.T) (*exec.Cmd, int, chan error) {
		t.Helper()
		pidFile := t.TempDir() + "/pid"
		cmd := exec.Command("/bin/sh", "-c", `sleep 300 & echo $$ > "$1"; wait`, "survivor", pidFile)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if raw, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(raw), "\n") {
				pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
				group, err := commandGroup(pid)
				if err != nil {
					t.Fatal(err)
				}
				return cmd, group, done
			}
			if time.Now().After(deadline) {
				t.Fatal("command never started")
			}
		}
	}

	// Before: killing only the leader leaves the background sleep holding
	// stderr, so Wait does not return.
	leaderOnly, group, done := start(t)
	t.Cleanup(func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
	_ = leaderOnly.Process.Kill()
	select {
	case <-done:
		t.Fatal("Wait returned with a descendant still holding stderr; the check proves nothing")
	case <-time.After(time.Second):
	}
	// The same cleanup the upgrade test uses retires the group in bound.
	if err := retireSurvivor(leaderOnly.Process, group, done, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	// After: a fresh command, retired directly, finishes inside the bound.
	cmd, group, done := start(t)
	began := time.Now()
	if err := retireSurvivor(cmd.Process, group, done, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("cleanup took %s", took)
	}
}
