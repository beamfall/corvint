//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func helperCommand(mode, root string) ([]string, []string) {
	exe := must(os.Executable())
	return []string{exe, "-test.run=^TestHDCProcessHelper$"}, []string{"HDC_HELPER_MODE=" + mode, "HDC_HELPER_ROOT=" + root, "PATH=/usr/bin:/bin"}
}
func waitHeartbeat(t *testing.T, root string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(filepath.Join(root, "heartbeat")); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child did not publish readiness")
}
func registerChildCleanup(t *testing.T, root string) {
	t.Helper()
	t.Cleanup(func() {
		raw, err := os.ReadFile(filepath.Join(root, "child.pid"))
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
}
func assertHeartbeatStopped(t *testing.T, root string) {
	t.Helper()
	info, err := os.Stat(filepath.Join(root, "heartbeat"))
	if err != nil {
		t.Fatal(err)
	}
	size := info.Size()
	time.Sleep(120 * time.Millisecond)
	info, err = os.Stat(filepath.Join(root, "heartbeat"))
	if err != nil || info.Size() != size {
		t.Fatalf("descendant survived: size=%d info=%v err=%v", size, info, err)
	}
}
func TestHDCProcessOwnership(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		root := t.TempDir()
		registerChildCleanup(t, root)
		argv, env := helperCommand("parent", root)
		expectFailure(t, "command:timeout", func() { runProcess(context.Background(), argv, root, env, 700*time.Millisecond, maxStreamBytes) })
		waitHeartbeat(t, root)
		assertHeartbeatStopped(t, root)
	})
	t.Run("overflow", func(t *testing.T) {
		root := t.TempDir()
		argv, env := helperCommand("flood", root)
		expectFailure(t, "output-bound-exceeded", func() { runProcess(context.Background(), argv, root, env, time.Second, 65536) })
	})
	t.Run("sigterm", func(t *testing.T) {
		root := t.TempDir()
		registerChildCleanup(t, root)
		argv, env := helperCommand("runner", root)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = env
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		finished := false
		t.Cleanup(func() {
			if !finished {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("runner did not reap")
				}
			}
		})
		waitHeartbeat(t, root)
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
			finished = true
		case <-time.After(4 * time.Second):
			t.Fatal("interrupted runner did not return")
		}
		assertHeartbeatStopped(t, root)
	})
}
func TestHDCProcessHelper(t *testing.T) {
	mode := os.Getenv("HDC_HELPER_MODE")
	if mode == "" {
		return
	}
	root := os.Getenv("HDC_HELPER_ROOT")
	switch mode {
	case "flood":
		for {
			_, err := os.Stdout.Write(make([]byte, 8192))
			if err != nil {
				os.Exit(0)
			}
		}
	case "child":
		signal.Ignore(syscall.SIGTERM)
		must(true, os.WriteFile(filepath.Join(root, "child.pid"), []byte(strconv.Itoa(os.Getpid())), 0600))
		f := must(os.OpenFile(filepath.Join(root, "heartbeat"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600))
		defer f.Close()
		for {
			must(f.WriteString("x"))
			time.Sleep(10 * time.Millisecond)
		}
	case "parent":
		argv, env := helperCommand("child", root)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		must(true, cmd.Start())
		defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
		time.Sleep(30 * time.Second)
	case "runner":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		defer func() {
			if e := recover(); e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(2)
			}
		}()
		argv, env := helperCommand("parent", root)
		runProcess(ctx, argv, root, env, 10*time.Second, maxStreamBytes)
		os.Exit(0)
	}
	os.Exit(0)
}
