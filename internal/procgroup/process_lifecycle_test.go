//go:build darwin || linux

package procgroup

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/Beamfall/corvint/internal/liveverify/processidentity"
)

type lifecycleProcessIdentity struct {
	PID   int
	PGID  int
	Start string
}

type lifecycleProcessReady struct {
	Leader lifecycleProcessIdentity
	Child  lifecycleProcessIdentity
}

// Readiness waits are hang detectors, not performance budgets (decision 0082).
const lifecycleReadinessHangTimeout = 30 * time.Second

func lifecycleIdentity(pid int) (lifecycleProcessIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start, err := processidentity.Start(ctx, pid)
	if err != nil {
		return lifecycleProcessIdentity{}, err
	}
	group, err := syscall.Getpgid(pid)
	return lifecycleProcessIdentity{PID: pid, PGID: group, Start: start}, err
}

// PID reuse and zombies are not surviving original processes. A pre-signal
// live probe is the sensitivity control for every asserted disappearance.
func lifecycleProcessLive(identity lifecycleProcessIdentity) (bool, error) {
	if identity.PID <= 0 {
		return false, errors.New("missing process identity")
	}
	if err := syscall.Kill(identity.PID, 0); errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	current, err := lifecycleIdentity(identity.PID)
	if err != nil {
		if errors.Is(syscall.Kill(identity.PID, 0), syscall.ESRCH) {
			return false, nil
		}
		return false, err
	}
	if current.Start != identity.Start || current.PGID != identity.PGID {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(identity.PID), "-o", "stat=")
	command.WaitDelay = time.Second
	state, err := command.Output()
	if err != nil {
		if errors.Is(syscall.Kill(identity.PID, 0), syscall.ESRCH) {
			return false, nil
		}
		return false, err
	}
	return !strings.HasPrefix(strings.TrimSpace(string(state)), "Z"), nil
}

func lifecycleWriteJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", raw, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func lifecycleReadJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func lifecycleWaitReady(t *testing.T, directory string) lifecycleProcessReady {
	t.Helper()
	deadline := time.Now().Add(lifecycleReadinessHangTimeout)
	for time.Now().Before(deadline) {
		var ready lifecycleProcessReady
		if lifecycleReadJSON(filepath.Join(directory, "ready"), &ready) == nil {
			for _, identity := range []lifecycleProcessIdentity{ready.Leader, ready.Child} {
				live, err := lifecycleProcessLive(identity)
				if err != nil || !live {
					t.Fatalf("ready process is not live: %+v, %v", identity, err)
				}
			}
			return ready
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("agent did not publish its atomic readiness record")
	return lifecycleProcessReady{}
}

// Register this before starting anything: the leader record precedes the child
// spawn, and the child publishes its own identity before the ready rendezvous.
func lifecycleCleanup(t *testing.T, directory string) {
	t.Helper()
	t.Cleanup(func() {
		deadline := time.Now().Add(2 * time.Second)
		var observationErr error
		for time.Now().Before(deadline) {
			found := false
			for _, name := range []string{"leader", "child"} {
				var identity lifecycleProcessIdentity
				if lifecycleReadJSON(filepath.Join(directory, name), &identity) != nil {
					continue
				}
				live, err := lifecycleProcessLive(identity)
				if err != nil {
					found = true
					observationErr = err
					continue
				}
				if live {
					found = true
					if identity.PGID == identity.PID && identity.PGID != syscall.Getpgrp() {
						_ = syscall.Kill(-identity.PGID, syscall.SIGKILL)
					}
					_ = syscall.Kill(identity.PID, syscall.SIGKILL)
				}
			}
			if !found {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("test cleanup did not quiesce its recorded processes (last observation error: %v)", observationErr)
	})
}

func lifecycleAssertGone(t *testing.T, identity lifecycleProcessIdentity) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		live, err := lifecycleProcessLive(identity)
		if err == nil && !live {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("original process remains live: %+v", identity)
}

// CWT-V0-014 / CRT-V0-011: the supervisor owns the full descriptor/exit matrix.
func TestRunProcessLifecycleMatrix(t *testing.T) {
	for _, streams := range []string{"inherited", "closed"} {
		for _, outcome := range []string{"0", "23", "cancel", "timeout"} {
			t.Run("CRR-V0-003 "+streams+"/"+outcome, func(t *testing.T) {
				directory := t.TempDir()
				lifecycleCleanup(t, directory)
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				executable, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				spec := Spec{
					Argv: []string{executable, "-test.run=^TestLifecycleProcessHelper$"},
					Dir:  directory,
					Env: []string{"LIFECYCLE_MODE=parent", "LIFECYCLE_ROOT=" + directory,
						"LIFECYCLE_STREAMS=" + streams, "LIFECYCLE_OUTCOME=" + outcome},
					Timeout: 30 * time.Second, ShutdownTimeout: 2 * time.Second,
					OutputLimit: 128,
				}
				if outcome == "timeout" {
					spec.Timeout = 2 * time.Second
				}
				done := make(chan Observation, 1)
				go func() { done <- Run(ctx, spec) }()
				finished := false
				t.Cleanup(func() {
					cancel()
					if !finished {
						select {
						case <-done:
						case <-time.After(4 * time.Second):
							t.Error("supervisor did not return during test cleanup")
						}
					}
				})
				ready := lifecycleWaitReady(t, directory)
				if ready.Leader.PGID != ready.Leader.PID || ready.Child.PGID != ready.Leader.PGID {
					t.Fatalf("child is outside the owned group: %+v", ready)
				}
				if outcome == "cancel" {
					cancel()
				}
				if outcome == "0" || outcome == "23" {
					if err := os.WriteFile(filepath.Join(directory, "release"), []byte("exit"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				var result Observation
				select {
				case result = <-done:
					finished = true
				case <-time.After(5 * time.Second):
					t.Fatal("supervisor exceeded its shutdown bound")
				}
				switch outcome {
				case "cancel":
					if !result.Cancelled || result.TimedOut {
						t.Errorf("cancellation flags: %+v", result)
					}
					assertProcessErrorCode(t, result.Err, "process-cancelled")
				case "timeout":
					if !result.TimedOut || result.Cancelled {
						t.Errorf("deadline flags: %+v", result)
					}
					assertProcessErrorCode(t, result.Err, "process-timeout")
				default:
					code, _ := strconv.Atoi(outcome)
					if result.Err != nil || result.ExitStatus != code {
						t.Errorf("normal exit lost: %+v", result)
					}
				}
				if !result.ExitObserved || !result.WaitCompleted || !result.PipesDrained || !result.OwnedProcessGroupCleanup {
					t.Errorf("incomplete cleanup: %+v", result)
				}
				if string(result.Stdout) != "normal output" || string(result.Stderr) != "normal error" {
					t.Errorf("pipe drain changed output: stdout=%q stderr=%q", result.Stdout, result.Stderr)
				}
				lifecycleAssertGone(t, ready.Leader)
				lifecycleAssertGone(t, ready.Child)
				if _, err := os.Stat(filepath.Join(directory, "late")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("descendant produced a delayed side effect: %v", err)
				}
			})
		}
	}
}

func TestLifecycleProcessHelper(t *testing.T) {
	mode := os.Getenv("LIFECYCLE_MODE")
	if mode == "" {
		return
	}
	directory := os.Getenv("LIFECYCLE_ROOT")
	if mode == "child" {
		signal.Ignore(syscall.SIGTERM)
		child, err := lifecycleIdentity(os.Getpid())
		if err != nil || lifecycleWriteJSON(filepath.Join(directory, "child"), child) != nil {
			os.Exit(125)
		}
		time.Sleep(6 * time.Second)
		_ = os.WriteFile(filepath.Join(directory, "late"), []byte("survived"), 0o600)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	leader, err := lifecycleIdentity(os.Getpid())
	if err != nil || lifecycleWriteJSON(filepath.Join(directory, "leader"), leader) != nil {
		os.Exit(125)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(125)
	}
	command := exec.Command(executable, "-test.run=^TestLifecycleProcessHelper$")
	command.Env = append(os.Environ(), "LIFECYCLE_MODE=child")
	if os.Getenv("LIFECYCLE_STREAMS") == "inherited" {
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
	}
	if err := command.Start(); err != nil {
		os.Exit(125)
	}
	var child lifecycleProcessIdentity
	deadline := time.Now().Add(lifecycleReadinessHangTimeout)
	for lifecycleReadJSON(filepath.Join(directory, "child"), &child) != nil {
		if time.Now().After(deadline) {
			os.Exit(125)
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _ = fmt.Fprint(os.Stdout, "normal output")
	_, _ = fmt.Fprint(os.Stderr, "normal error")
	if lifecycleWriteJSON(filepath.Join(directory, "ready"), lifecycleProcessReady{Leader: leader, Child: child}) != nil {
		os.Exit(125)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "release")); err == nil {
			code, err := strconv.Atoi(os.Getenv("LIFECYCLE_OUTCOME"))
			if err != nil {
				os.Exit(125)
			}
			os.Exit(code)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
