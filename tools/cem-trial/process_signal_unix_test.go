//go:build darwin || linux

package main

import (
	"bytes"
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

// These subprocesses enter the real main via TestMain, so this also covers
// production signal.NotifyContext wiring rather than a copied signal handler.
func TestTrialMainSignalsCleanOwnedAgentGroup(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run("CRT-V0-011 "+sig.String(), func(t *testing.T) {
			directory := t.TempDir()
			trialCleanup(t, directory)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			script := filepath.Join(directory, "agent.sh")
			quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
			if err := os.WriteFile(script, []byte("#!/bin/sh\nexport TRIAL_COMMAND_MODE=hold\nexec "+quoted+" -test.run='^TestTrialProcessHelper$'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(executable, trialSignalArguments(t, directory, script)...)
			command.Env = append(os.Environ(), "TRIAL_LIFECYCLE_MAIN=1", "TRIAL_PROCESS_DIR="+directory)
			command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			finished := false
			started := false
			var outer trialProcessIdentity
			done := make(chan error, 1)
			// Register before Start. Process.Signal addresses the original handle;
			// group cleanup additionally requires a fresh matching identity.
			t.Cleanup(func() {
				if !started {
					return
				}
				if !finished {
					_ = command.Process.Signal(syscall.SIGTERM)
					select {
					case <-done:
						finished = true
					case <-time.After(2 * time.Second):
						if live, err := trialProcessLive(outer); err == nil && live {
							_ = syscall.Kill(-outer.PGID, syscall.SIGKILL)
						}
						_ = command.Process.Kill()
						select {
						case <-done:
							finished = true
						case <-time.After(2 * time.Second):
							t.Error("trial process did not reap during cleanup")
						}
					}
				}
			})
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			started = true
			go func() { done <- command.Wait() }()
			outer, err = trialIdentity(command.Process.Pid)
			if err != nil {
				t.Fatal(err)
			}
			ready := trialWaitReady(t, directory)
			if ready.Leader.PID != ready.Leader.PGID || ready.Child.PGID != ready.Leader.PGID || ready.Leader.PGID == outer.PGID {
				t.Fatalf("agent does not own its separate group: outer=%+v agent=%+v", outer, ready)
			}
			// Ready checked both processes as live; signal only the trial leader.
			if err := command.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
				finished = true // A cancelled lane may still yield CLI exit 0.
			case <-time.After(7 * time.Second):
				t.Fatalf("production main did not exit after %s", sig)
			}
			trialAssertGone(t, outer)
			trialAssertGone(t, ready.Leader)
			trialAssertGone(t, ready.Child)
			trialAssertGroupQuiescent(t, ready.Leader.PGID)
			if _, err := os.Stat(filepath.Join(directory, "late")); !os.IsNotExist(err) {
				t.Fatalf("agent descendant produced a delayed side effect: %v", err)
			}
		})
	}
}

func trialAssertGroupQuiescent(t *testing.T, group int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		command := exec.CommandContext(ctx, "/bin/ps", "-axo", "pgid=,stat=")
		command.WaitDelay = time.Second
		raw, err := command.Output()
		cancel()
		if err != nil {
			t.Fatalf("observe owned group: %v", err)
		}
		live := false
		for _, row := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(row)
			if len(fields) >= 2 && fields[0] == strconv.Itoa(group) && !strings.HasPrefix(fields[1], "Z") {
				live = true
			}
		}
		if !live {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error(fmt.Sprintf("owned group %d remains live", group))
}

func trialSignalArguments(t *testing.T, directory, agent string) []string {
	repository := fixtureRepo(t)
	revision, err := git(context.Background(), repository, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base, err := git(context.Background(), repository, "rev-parse", "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	patch := "diff --git a/internal/play/play.go b/internal/play/play.go\n--- a/internal/play/play.go\n+++ b/internal/play/play.go\n@@ -1 +1 @@\n-old\n+new\n"
	if err := os.WriteFile(filepath.Join(directory, "change.patch"), []byte(patch), 0o600); err != nil {
		t.Fatal(err)
	}
	set := manifest{Partition: "pilot", Seed: "lifecycle", SelectionRule: selectionRule,
		Population: 1, Repository: "synthetic-lifecycle", Pairs: 1,
		Tasks: []task{{ID: "c01", Commit: revision, BaseCommit: base, PatchPath: "change.patch",
			PatchSHA256: digestOf(patch), HunkCount: 1, SourceFiles: []string{"internal/play/play.go"}}}}
	path := filepath.Join(directory, "tasks.json")
	if err := trialWriteJSON(path, set); err != nil {
		t.Fatal(err)
	}
	return []string{"run", "--tasks", path, "--history", repository, "--arms", "control",
		"--agent", "script", "--agent-command", agent, "--timeout", "30s",
		"--output", filepath.Join(directory, "report.json")}
}
