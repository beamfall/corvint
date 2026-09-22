//go:build unix

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestObserverInterruptLeavesNoProcessGroup(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestNativeObserverHelperProcess$")
	command.Env = append(os.Environ(), "CORVINT_NATIVE_OBSERVER_HELPER=1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	defer func() { _ = syscall.Kill(-pid, syscall.SIGKILL); _ = command.Wait() }()
	if _, err = stdin.Write([]byte{'{'}); err != nil {
		t.Fatal(err)
	}
	if err = syscall.Kill(pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	deadline := time.Now().Add(time.Second)
	for syscall.Kill(-pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if err := syscall.Kill(-pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("process group survived: %v", err)
	}
}

// mustCreateFIFO creates a named pipe at path so
// TestInventoryProjectionAndExpectedFileProtection can prove loadRegularJSON
// refuses to block reading one. Only unix has FIFOs.
func mustCreateFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
}

// TestObserverBlockingPipeInputHonorsLifetime covers NPO-V0-009: a blocking
// pipe fd is not pollable, so a read deadline cannot end the read; observe
// mode must still return by its lifetime. The 10x bound is a hang detector.
func TestObserverBlockingPipeInputHonorsLifetime(t *testing.T) {
	fds := make([]int, 2)
	if err := syscall.Pipe(fds); err != nil {
		t.Fatal(err)
	}
	read, write := os.NewFile(uintptr(fds[0]), "blocking-read"), os.NewFile(uintptr(fds[1]), "blocking-write")
	defer read.Close()
	defer write.Close()
	if _, err := write.Write([]byte{'{'}); err != nil {
		t.Fatal(err)
	}
	lifetime := 50 * time.Millisecond
	done := make(chan int, 1)
	go func() { done <- runObserver(context.Background(), read, io.Discard, lifetime) }()
	select {
	case status := <-done:
		if status != 1 {
			t.Fatalf("status=%d, want 1", status)
		}
	case <-time.After(10 * lifetime):
		t.Fatal("blocking pipe input outlived the observer lifetime")
	}
}
