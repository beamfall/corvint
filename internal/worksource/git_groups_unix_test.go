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

// TestGitGroupsProducerHelper is the producer the killed-producer tests use: it
// acquires scratch and blocks in a fixture Git read, or with DIE_AT_START kills
// itself between the Git child's start and its wait.
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
	if os.Getenv("CORVINT_WORKSOURCE_TEST_DIE_AT_START") != "" {
		afterGitStart = func() { _ = syscall.Kill(os.Getpid(), syscall.SIGKILL) }
	}
	_, _ = source.Git(context.Background(), 100, "cat-file")
}

// GitGroupFixture writes a Git fixture under root that records its own and its
// sleeping child's PIDs at pidPath, then waits.
func gitGroupFixture(t *testing.T, root, pidPath string) {
	t.Helper()
	write(t, root, "git-fixture", []byte("#!/bin/sh\nsleep 600 &\nprintf '%s %s\\n' \"$$\" \"$!\" > "+pidPath+".tmp\nmv "+pidPath+".tmp "+pidPath+"\nwait\n"), 0755)
}

// killedProducer runs the helper producer, SIGKILLs its group once the Git
// fixture is running (unless it already died at start) and returns its scratch
// and the fixture's PIDs, which are confirmed to have outlived the producer.
func killedProducer(t *testing.T, dieAtStart bool) (string, []int) {
	t.Helper()
	scratch, root := privateDirectory(t), t.TempDir()
	pidPath := filepath.Join(root, "pids")
	gitGroupFixture(t, root, pidPath)
	producer := exec.Command(os.Args[0], "-test.run=^TestGitGroupsProducerHelper$", "-test.count=1")
	producer.Env = append(os.Environ(), gitGroupsProducerScratch+"="+scratch, "CORVINT_WORKSOURCE_TEST_ROOT="+root)
	if dieAtStart {
		producer.Env = append(producer.Env, "CORVINT_WORKSOURCE_TEST_DIE_AT_START=1")
	}
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
	pids = waitFixturePIDs(t, pidPath, func() bool {
		select {
		case <-done:
			return !dieAtStart
		default:
			return false
		}
	})
	_ = syscall.Kill(-producer.Process.Pid, syscall.SIGKILL)
	<-done
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("Git process %d did not survive the producer kill: %v", pid, err)
		}
	}
	return scratch, pids
}

// waitFixturePIDs reads the fixture's PID marker. Readiness ends on the marker
// or on exited; the deadline is a hang detector.
func waitFixturePIDs(t *testing.T, pidPath string, exited func() bool) []int {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		if raw, err := os.ReadFile(pidPath); err == nil {
			var pids []int
			for _, field := range strings.Fields(string(raw)) {
				pid, err := strconv.Atoi(field)
				if err != nil || pid <= 1 {
					t.Fatalf("fixture pids %q", raw)
				}
				pids = append(pids, pid)
			}
			return pids
		}
		if exited() {
			t.Fatal("producer exited before its Git fixture was ready")
		}
		if time.Now().After(deadline) {
			t.Fatal("Git fixture readiness exceeded the hang bound")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// assertGone waits, bounded by a hang detector, until every PID is gone; the
// reparented zombies are reaped by the system.
func assertGone(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for _, pid := range pids {
		for !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			if time.Now().After(deadline) {
				t.Fatalf("Git process %d outlived RetireGitGroups", pid)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// WQO-V0-052 (V1-2291): Git children lead their own groups, so a SIGKILL of the
// producer's group leaves them running inside its acquisition scratch. The
// scratch owner retires them before removing the scratch.
func TestRetireGitGroupsAfterProducerKill(t *testing.T) {
	scratch, pids := killedProducer(t, false)
	if err := RetireGitGroups(scratch, time.Minute); err != nil {
		t.Fatal(err)
	}
	assertGone(t, pids)
}

// The record is complete before Git exists, so a producer killed between the
// Git child's start and anything it does afterwards leaves a retirable run.
func TestRetireGitGroupsAfterProducerDiesAtGitStart(t *testing.T) {
	scratch, pids := killedProducer(t, true)
	if err := RetireGitGroups(scratch, time.Minute); err != nil {
		t.Fatal(err)
	}
	assertGone(t, pids)
}

// A held record whose holder is no marked Git run is never signalled, even
// when its content names a live process group, and the run stays unconfirmed.
func TestRetireGitGroupsNeverSignalsAnUnpinnedGroup(t *testing.T) {
	scratch := privateDirectory(t)
	unrelated := exec.Command("/bin/sleep", "600")
	unrelated.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = unrelated.Wait(); close(done) }()
	t.Cleanup(func() { _ = unrelated.Process.Kill(); <-done })
	directory := filepath.Join(scratch, "corvint-work-source-x", gitGroupDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("ab", 16)
	path := filepath.Join(directory, "group-"+token)
	write(t, directory, "group-"+token, []byte(strconv.Itoa(unrelated.Process.Pid)+"\n"), 0600)
	holder, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := syscall.Flock(int(holder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := RetireGitGroups(scratch, 200*time.Millisecond); err == nil {
		t.Fatal("a record held by an unretired holder was reported retired")
	}
	select {
	case <-done:
		t.Fatal("RetireGitGroups signalled a process group it could not pin")
	default:
	}
	if err := syscall.Kill(unrelated.Process.Pid, 0); err != nil {
		t.Fatalf("unrelated process group was signalled: %v", err)
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

// Every scratch-owning Git run carries the marker its record names, and
// removes its record after the reap.
func TestGitRunRemovesItsGroupRecord(t *testing.T) {
	source, err := newSourceWithScratch(privateDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = t.TempDir()
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nprintf '%s %s\\n' \"$1\" \"$2\"\nls \"$HOME/../git-groups\"\n"), 0755)
	out, err := source.Git(context.Background(), 1000, "ignored")
	marker, record, _ := strings.Cut(string(out), "\n")
	token, found := strings.CutPrefix(marker, "-c corvint.worksourcerun=")
	if err != nil || !found || len(token) != 32 || record != "group-"+token+"\n" {
		t.Fatalf("Git run did not carry its recorded marker: %q %v", out, err)
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
