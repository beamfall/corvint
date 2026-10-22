//go:build darwin || linux

package worksource

import (
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
)

const gitGroupsProducerScratch = "CORVINT_WORKSOURCE_TEST_PRODUCER_SCRATCH"

// TestGitGroupsProducerHelper is the producer TestRetireGitGroupsAfterProducerKill
// kills: it acquires scratch and blocks in a fixture Git read.
func TestGitGroupsProducerHelper(t *testing.T) {
	scratch := os.Getenv(gitGroupsProducerScratch)
	if scratch == "" {
		t.Skip("producer helper")
	}
	source, err := newSourceWithScratch(scratch)
	if err != nil {
		t.Fatal(err)
	}
	source.Root = os.Getenv("CORVINT_WORKSOURCE_TEST_ROOT")
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	_, _ = source.Git(context.Background(), 100, "cat-file")
}

// WQO-V0-052 (V1-2291): Git children lead their own groups, so a SIGKILL of the
// producer's group leaves them running inside its acquisition scratch. The
// scratch owner retires them before removing the scratch.
func TestRetireGitGroupsAfterProducerKill(t *testing.T) {
	scratch, root := privateDirectory(t), t.TempDir()
	pidPath := filepath.Join(root, "pids")
	write(t, root, "git-fixture", []byte("#!/bin/sh\nsleep 600 &\nprintf '%s %s\\n' \"$$\" \"$!\" > "+pidPath+".tmp\nmv "+pidPath+".tmp "+pidPath+"\nwait\n"), 0755)
	producer := exec.Command(os.Args[0], "-test.run=^TestGitGroupsProducerHelper$", "-test.count=1")
	producer.Env = append(os.Environ(), gitGroupsProducerScratch+"="+scratch, "CORVINT_WORKSOURCE_TEST_ROOT="+root)
	producer.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := producer.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = producer.Wait(); close(done) }()
	var pids []int
	t.Cleanup(func() {
		_ = syscall.Kill(-producer.Process.Pid, syscall.SIGKILL)
		<-done
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	// Readiness ends on the marker or producer exit; the deadline is a hang detector.
	deadline := time.Now().Add(time.Minute)
	for pids == nil {
		if raw, err := os.ReadFile(pidPath); err == nil {
			for _, field := range strings.Fields(string(raw)) {
				pid, err := strconv.Atoi(field)
				if err != nil || pid <= 1 {
					t.Fatalf("fixture pids %q", raw)
				}
				pids = append(pids, pid)
			}
		}
		select {
		case <-done:
			if pids == nil {
				t.Fatal("producer exited before its Git fixture was ready")
			}
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Git fixture readiness exceeded the hang bound")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := syscall.Kill(-producer.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	<-done
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("Git process %d did not survive the producer kill: %v", pid, err)
		}
	}

	if err := RetireGitGroups(scratch, time.Minute); err != nil {
		t.Fatal(err)
	}
	// The released lock means every holder has exited; the reparented zombies are
	// reaped by the system, bounded by a hang detector.
	deadline = time.Now().Add(time.Minute)
	for _, pid := range pids {
		for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			if time.Now().After(deadline) {
				t.Fatalf("Git process %d outlived RetireGitGroups", pid)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := os.RemoveAll(scratch); err != nil {
		t.Fatal(err)
	}
}

// A record whose lock is free belongs to a finished group and is never signalled.
func TestRetireGitGroupsLeavesFinishedRecords(t *testing.T) {
	scratch := t.TempDir()
	directory := filepath.Join(scratch, "corvint-work-source-x", gitGroupDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	// Our own group: signalling it would kill this test.
	write(t, directory, "group-1", []byte(strconv.Itoa(syscall.Getpgrp())+"\n"), 0600)
	if err := RetireGitGroups(scratch, time.Second); err != nil {
		t.Fatal(err)
	}
}

// Every scratch-owning Git run removes its record after the reap.
func TestGitRunRemovesItsGroupRecord(t *testing.T) {
	source, err := newSourceWithScratch(privateDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = t.TempDir()
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nprintf ok\n"), 0755)
	if out, err := source.Git(context.Background(), 100, "ignored"); err != nil || string(out) != "ok" {
		t.Fatalf("%q %v", out, err)
	}
	entries, err := os.ReadDir(filepath.Join(source.temporary, gitGroupDirectory))
	if err != nil || len(entries) != 0 {
		t.Fatalf("group records after the reap: %v %v", entries, err)
	}
}

func privateDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	return directory
}
