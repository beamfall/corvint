//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
package contextindex

import (
	"context"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestWorkerGitProcessGroup(t *testing.T) {
	mode := os.Getenv("CORVINT_TEST_WORKER_GROUP")
	if mode == "" {
		for _, m := range []string{"default", "inherited"} {
			command := exec.Command(os.Args[0], "-test.run=^TestWorkerGitProcessGroup$", "-test.v")
			command.Env = append(os.Environ(), "CORVINT_TEST_WORKER_GROUP="+m)
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			if out, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%s: %s %v", m, out, err)
			} else {
				t.Logf("%s: %s", m, out)
			}
		}
		return
	}
	if mode == "inherited" {
		if err := gitstatus.EnableOwnedWorker(); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, gitstatus.Executable(), "hash-object", "--stdin")
	input, writer := io.Pipe()
	defer input.Close()
	defer writer.Close()
	command.Stdin = input
	configureProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group, err := syscall.Getpgid(command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		t.Fatal(err)
	}
	wanted := command.Process.Pid
	if mode == "inherited" {
		wanted = syscall.Getpgrp()
	}
	t.Logf("mode=%s parent=%d git=%d group=%d expected=%d", mode, os.Getpid(), command.Process.Pid, group, wanted)
	if group != wanted {
		command.Process.Kill()
		command.Wait()
		t.Fatalf("group mismatch")
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	cancel()
	writer.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Git cancellation did not join")
	}
	if err := syscall.Kill(command.Process.Pid, 0); err != syscall.ESRCH {
		t.Fatalf("Git leader remains: %v", err)
	}
}
