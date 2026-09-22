//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// TestMain enters the production signal handler only in an owned test subprocess.
func TestMain(m *testing.M) {
	if os.Getenv("TRIAL_LIFECYCLE_MAIN") == "1" {
		_ = os.Unsetenv("TRIAL_LIFECYCLE_MAIN")
		main()
		return
	}
	os.Exit(m.Run())
}

type trialProcessIdentity struct {
	PID   int
	PGID  int
	Start string
}

type trialProcessReady struct {
	Leader trialProcessIdentity
	Child  trialProcessIdentity
}

func trialIdentity(pid int) (trialProcessIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start, err := processidentity.Start(ctx, pid)
	if err != nil {
		return trialProcessIdentity{}, err
	}
	group, err := syscall.Getpgid(pid)
	return trialProcessIdentity{PID: pid, PGID: group, Start: start}, err
}

// PID reuse and zombies are not surviving original processes. A pre-signal
// live probe is the sensitivity control for every asserted disappearance.
func trialProcessLive(identity trialProcessIdentity) (bool, error) {
	if identity.PID <= 0 {
		return false, errors.New("missing process identity")
	}
	if err := syscall.Kill(identity.PID, 0); errors.Is(err, syscall.ESRCH) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	current, err := trialIdentity(identity.PID)
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

func trialWriteJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", raw, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func trialReadJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func trialWaitReady(t *testing.T, directory string) trialProcessReady {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var ready trialProcessReady
		if trialReadJSON(filepath.Join(directory, "ready"), &ready) == nil {
			for _, identity := range []trialProcessIdentity{ready.Leader, ready.Child} {
				live, err := trialProcessLive(identity)
				if err != nil || !live {
					t.Fatalf("ready process is not live: %+v, %v", identity, err)
				}
			}
			return ready
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("agent did not publish its atomic readiness record")
	return trialProcessReady{}
}

// Register this before starting anything: the leader record precedes the child
// spawn, and the child publishes its own identity before the ready rendezvous.
func trialCleanup(t *testing.T, directory string) {
	t.Helper()
	t.Cleanup(func() {
		deadline := time.Now().Add(2 * time.Second)
		var observationErr error
		for time.Now().Before(deadline) {
			found := false
			for _, name := range []string{"leader", "child"} {
				var identity trialProcessIdentity
				if trialReadJSON(filepath.Join(directory, name), &identity) != nil {
					continue
				}
				live, err := trialProcessLive(identity)
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

func trialAssertGone(t *testing.T, identity trialProcessIdentity) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		live, err := trialProcessLive(identity)
		if err == nil && !live {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("original process remains live: %+v", identity)
}

func TestRunCommandTruncatesOutput(t *testing.T) {
	for _, mode := range []string{"stdout", "stderr"} {
		t.Run("CRT-V0-011 "+mode, func(t *testing.T) {
			t.Setenv("TRIAL_COMMAND_MODE", mode)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			stdout, code, stderr, err := trialRunCommand(context.Background(), t.TempDir(), 10*time.Second, executable, "-test.run=^TestTrialProcessHelper$")
			if err != nil || code != 0 {
				t.Fatalf("bounded output changed successful exit: exit=%d error=%v", code, err)
			}
			if mode == "stdout" && (len(stdout) != maxOutputBytes || string(stdout) != strings.Repeat("x", maxOutputBytes)) {
				t.Fatalf("stdout retained %d bytes, want exact %d-byte prefix", len(stdout), maxOutputBytes)
			}
			if mode == "stderr" && trialCapturesStderr && stderr != strings.Repeat("e", maxErrorBytes) {
				t.Fatalf("stderr retained %d unexpected bytes", len(stderr))
			}
		})
	}
}

func TestRunCommandCancellationKillsDescendant(t *testing.T) {
	for _, trigger := range []string{"cancel", "timeout"} {
		t.Run("CRT-V0-011 "+trigger, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("TRIAL_COMMAND_MODE", "hold")
			t.Setenv("TRIAL_PROCESS_DIR", directory)
			trialCleanup(t, directory)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			timeout := 2 * time.Second
			if trigger == "cancel" {
				timeout = 30 * time.Second
			}
			done := make(chan error, 1)
			go func() {
				_, _, _, err := trialRunCommand(ctx, directory, timeout, executable, "-test.run=^TestTrialProcessHelper$")
				done <- err
			}()
			finished := false
			t.Cleanup(func() {
				cancel()
				if !finished {
					select {
					case <-done:
					case <-time.After(6 * time.Second):
						t.Error("test invocation did not return")
					}
				}
			})
			ready := trialWaitReady(t, directory)
			if trigger == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				finished = true
				want := "context canceled"
				if trigger == "timeout" {
					want = "context deadline exceeded"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("invocation error = %v, want %s", err, want)
				}
			case <-time.After(8 * time.Second):
				t.Fatal("command cancellation exceeded its shutdown bound")
			}
			trialAssertGone(t, ready.Leader)
			trialAssertGone(t, ready.Child)
			if _, err := os.Stat(filepath.Join(directory, "late")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("descendant produced a delayed side effect: %v", err)
			}
		})
	}
}

func TestTrialProcessHelper(t *testing.T) {
	mode := os.Getenv("TRIAL_COMMAND_MODE")
	if mode == "" {
		return
	}
	switch mode {
	case "status":
		_, _ = os.Stdout.Write([]byte("stdout\x00\n"))
		_, _ = os.Stderr.Write([]byte(" stderr\n"))
		code, _ := strconv.Atoi(os.Getenv("TRIAL_COMMAND_STATUS"))
		if code == -1 {
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		}
		os.Exit(code)
	case "environment":
		directory, err := os.Getwd()
		if err != nil {
			os.Exit(125)
		}
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(125)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"directory": directory, "value": os.Getenv("TRIAL_VALUE"), "input": len(input)})
	case "stdout":
		// Leave one byte of capacity before a separate two-byte write. A
		// cap-aligned pipe chunk would miss the writer's short-count defect.
		_, _ = os.Stdout.Write([]byte(strings.Repeat("x", maxOutputBytes-1)))
		time.Sleep(100 * time.Millisecond)
		_, _ = os.Stdout.Write([]byte("xx"))
	case "stderr":
		_, _ = os.Stderr.Write([]byte(strings.Repeat("e", maxErrorBytes+4096)))
	case "hold":
		directory := os.Getenv("TRIAL_PROCESS_DIR")
		leader, err := trialIdentity(os.Getpid())
		if err != nil || trialWriteJSON(filepath.Join(directory, "leader"), leader) != nil {
			os.Exit(125)
		}
		executable, err := os.Executable()
		if err != nil {
			os.Exit(125)
		}
		command := exec.Command(executable, "-test.run=^TestTrialProcessHelper$")
		command.Env = append(os.Environ(), "TRIAL_COMMAND_MODE=child")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Start(); err != nil {
			os.Exit(125)
		}
		var child trialProcessIdentity
		deadline := time.Now().Add(3 * time.Second)
		for trialReadJSON(filepath.Join(directory, "child"), &child) != nil {
			if time.Now().After(deadline) {
				os.Exit(125)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if trialWriteJSON(filepath.Join(directory, "ready"), trialProcessReady{Leader: leader, Child: child}) != nil {
			os.Exit(125)
		}
		time.Sleep(30 * time.Second)
	case "child":
		signal.Ignore(syscall.SIGTERM)
		directory := os.Getenv("TRIAL_PROCESS_DIR")
		child, err := trialIdentity(os.Getpid())
		if err != nil || trialWriteJSON(filepath.Join(directory, "child"), child) != nil {
			os.Exit(125)
		}
		time.Sleep(6 * time.Second)
		_ = os.WriteFile(filepath.Join(directory, "late"), []byte("survived"), 0o600)
		time.Sleep(30 * time.Second)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper mode")
		os.Exit(125)
	}
	os.Exit(0)
}

const trialCapturesStderr = true

func trialRunCommand(ctx context.Context, root string, timeout time.Duration, name string, args ...string) ([]byte, int, string, error) {
	return runCommand(ctx, root, timeout, name, args...)
}
