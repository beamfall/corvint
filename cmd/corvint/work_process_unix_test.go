//go:build unix

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Helper processes are selected by argv, never by an inherited environment.
func TestWorkProcessHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--work-process-helper" {
			index = i
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) != 2 {
		os.Exit(91)
	}
	mode, root := args[0], args[1]
	switch mode {
	case "escape":
		// Register before setsid, allowing the independent guardian to clean up
		// even if the coordinator dies at any point after this write.
		if os.WriteFile(filepath.Join(root, "pid"), []byte(strconv.Itoa(os.Getpid())), 0600) != nil {
			os.Exit(92)
		}
		if _, err := syscall.Setsid(); err != nil {
			os.Exit(93)
		}
		if os.WriteFile(filepath.Join(root, "escaped"), []byte("ready"), 0600) != nil {
			os.Exit(94)
		}
		fmt.Print("held\n")
		time.Sleep(8 * time.Second)
		os.Exit(0)
	case "guardian":
		// This session is established and acknowledged before the coordinator
		// can launch. It has its own deadline and survives coordinator SIGKILL.
		if os.WriteFile(filepath.Join(root, "guardian-ready"), []byte(strconv.Itoa(os.Getpid())), 0600) != nil {
			os.Exit(95)
		}
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(root, "stop")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		raw, err := os.ReadFile(filepath.Join(root, "pid"))
		if err != nil {
			os.Exit(0)
		}
		pid, err := strconv.Atoi(string(raw))
		if err != nil || pid <= 1 {
			os.Exit(96)
		}
		_ = syscall.Kill(pid, syscall.SIGKILL)
		if leaderRaw, err := os.ReadFile(filepath.Join(root, "leader")); err == nil {
			if leader, err := strconv.Atoi(strings.TrimSpace(string(leaderRaw))); err == nil && leader > 1 {
				_ = syscall.Kill(-leader, syscall.SIGKILL)
			}
		}
		os.Exit(0)
	case "coordinator":
		binary, err := os.Executable()
		if err != nil {
			os.Exit(97)
		}
		script := workEscapedScript(binary, root, true)
		runner := workTestRunner(t, context.Background(), script)
		_, _, _ = runner.run("snapshot", []string{}, 100)
		os.Exit(0)
	}
	os.Exit(98)
}

func workEscapedScript(binary, root string, wait bool) string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	script := "#!/bin/sh\nprintf '%s' $$ > " + quote(filepath.Join(root, "leader")) + "\n" + quote(binary) + " -test.run=^TestWorkProcessHelper$ -- --work-process-helper escape " + quote(root) + " &\n"
	script += "i=0\nwhile [ ! -f " + quote(filepath.Join(root, "escaped")) + " ]; do i=$((i + 1)); [ \"$i\" -lt 100 ] || exit 9; sleep 0.01; done\n"
	if wait {
		return script + "wait\n"
	}
	return script + "exit 0\n"
}

func workAwaitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper registration missing: %s", path)
}

func workStartGuardian(t *testing.T, root string) (*exec.Cmd, string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	guardian := exec.Command(binary, "-test.run=^TestWorkProcessHelper$", "--", "--work-process-helper", "guardian", root)
	guardian.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := guardian.Start(); err != nil {
		t.Fatal(err)
	}
	// Install cleanup before waiting for readiness; the guardian's own deadline
	// remains active if this entire test process is interrupted.
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(root, "stop"), []byte("stop"), 0600)
		done := make(chan error, 1)
		go func() { done <- guardian.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("guardian: %v", err)
			}
		case <-time.After(5 * time.Second):
			_ = guardian.Process.Kill()
			<-done
			t.Error("guardian deadline exceeded")
		}
		for _, name := range []string{"pid", "leader"} {
			workAssertHelperGone(t, filepath.Join(root, name))
		}
	})
	workAwaitFile(t, filepath.Join(root, "guardian-ready"))
	return guardian, binary
}

func workAssertHelperGone(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Error(err)
		return
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("owned helper %d remains after cleanup", pid)
}

// WQO-V0-034: normal group residue must be killed and disappear before the
// fixture completes; PID evidence is recorded before the leader exits.
func TestWorkRunnerGroupDescendantCleanup(t *testing.T) {
	runner := workTestRunner(t, context.Background(), "#!/bin/sh\nsleep 30 &\nprintf '%s' $! > child-pid\nexit 0\n")
	_, receipt, err := runner.run("snapshot", []string{}, 100)
	if err == nil || receipt.State != "INCOMPLETE" {
		t.Fatalf("residue: %+v %v", receipt, err)
	}
	workAssertHelperGone(t, filepath.Join(runner.root, "child-pid"))
}

// WQO-V0-034: escaped pipe holders cannot make production capture unbounded.
// This guardian is a fixture safety mechanism, not a containment claim.
func TestWorkRunnerEscapedPipeHolder(t *testing.T) {
	root := t.TempDir()
	_, binary := workStartGuardian(t, root)
	runner := workTestRunner(t, context.Background(), workEscapedScript(binary, root, false))
	start := time.Now()
	_, receipt, err := runner.run("snapshot", []string{}, 100)
	if time.Since(start) > 2*time.Second || err == nil || receipt.State != "INCOMPLETE" || !strings.Contains(err.Error(), "pipe") {
		t.Fatalf("escaped capture: %+v %v", receipt, err)
	}
}

// WQO-V0-034: prove the separately registered fixture guardian outlives a killed
// coordinator and removes the escaped helper; no production crash claim follows.
func TestWorkRunnerCoordinatorInterruptionGuardian(t *testing.T) {
	root := t.TempDir()
	guardian, binary := workStartGuardian(t, root)
	coordinator := exec.Command(binary, "-test.run=^TestWorkProcessHelper$", "--", "--work-process-helper", "coordinator", root)
	workContain(coordinator)
	if err := coordinator.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { workKillGroup(coordinator) })
	workAwaitFile(t, filepath.Join(root, "escaped"))
	workKillGroup(coordinator)
	done := make(chan error, 1)
	go func() { done <- coordinator.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("coordinator not reaped")
	}
	if syscall.Kill(guardian.Process.Pid, 0) != nil {
		t.Fatal("guardian died with coordinator")
	}
}
