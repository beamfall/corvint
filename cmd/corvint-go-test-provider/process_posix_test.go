//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// GLTP-V0-049: every platform outside darwin/arm64 is refused before execution.
func TestSupportedProviderPlatformRefusesLinuxAMD64(t *testing.T) {
	if supportedProviderPlatform("linux", "amd64") {
		t.Fatal("linux/amd64 reported as a supported provider platform")
	}
}

func TestDirectCommandHelper(t *testing.T) {
	mode := os.Getenv("DIRECT_COMMAND_HELPER_MODE")
	if mode == "" {
		if reflect.DeepEqual(os.Args[1:], []string{directCommandHelperArgument}) {
			if len(os.Environ()) != 0 {
				os.Exit(99)
			}
			os.Exit(0)
		}
		return
	}
	switch mode {
	case "exact":
		wantCWD, err := filepath.EvalSymlinks(os.Getenv("DIRECT_COMMAND_EXPECT_CWD"))
		if err != nil {
			os.Exit(90)
		}
		cwd, err := os.Getwd()
		if err != nil || cwd != wantCWD || !reflect.DeepEqual(os.Args[1:], []string{directCommandHelperArgument}) {
			os.Exit(91)
		}
		wantEnvironment := []string{
			"DIRECT_COMMAND_EXPECT_CWD=" + os.Getenv("DIRECT_COMMAND_EXPECT_CWD"),
			"DIRECT_COMMAND_HELPER_MODE=exact",
		}
		if !reflect.DeepEqual(os.Environ(), wantEnvironment) {
			os.Exit(92)
		}
		_, _ = os.Stdout.WriteString("discovery\n")
		os.Exit(0)
	case "exit":
		os.Exit(23)
	case "hold":
		if err := os.WriteFile(os.Getenv("DIRECT_COMMAND_READY"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(93)
		}
		for {
			time.Sleep(time.Hour)
		}
	case "output":
		chunk := bytes.Repeat([]byte("x"), 64<<10)
		for range directCommandOutputLimit/int64(len(chunk)) + 1 {
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(0)
			}
		}
		for {
			time.Sleep(time.Hour)
		}
	case "descendant":
		command := exec.Command(os.Args[0], directCommandHelperArgument)
		command.Env = []string{
			"DIRECT_COMMAND_HELPER_MODE=hold",
			"DIRECT_COMMAND_READY=" + os.Getenv("DIRECT_COMMAND_READY"),
		}
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			os.Exit(94)
		}
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(os.Getenv("DIRECT_COMMAND_READY")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(96)
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(0)
	case "stuck-descendant-pipe":
		if err := os.WriteFile(os.Getenv("DIRECT_COMMAND_LEADER_READY"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			os.Exit(97)
		}
		command := exec.Command(os.Args[0], directCommandHelperArgument)
		command.Env = []string{
			"DIRECT_COMMAND_HELPER_MODE=hold",
			"DIRECT_COMMAND_READY=" + os.Getenv("DIRECT_COMMAND_READY"),
		}
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			os.Exit(98)
		}
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(95)
	}
}

func TestRunDirectCommandDoesNotInheritAmbientEnvironment(t *testing.T) {
	for _, environment := range [][]string{nil, {}} {
		result, err := runDirectCommand(
			context.Background(),
			directCommandTestExecutable(t),
			[]string{directCommandHelperArgument},
			environment,
			t.TempDir(),
			5*time.Second,
		)
		if err != nil || !result.Started || !result.WaitCompleted || !result.Exited || result.ExitCode != 0 ||
			!result.ProcessCleanupDone || !result.PipesDrained || len(result.Stdout) != 0 || len(result.Stderr) != 0 {
			t.Fatalf("environment=%#v result=%#v error=%v", environment, result, err)
		}
	}
}

func TestRunDirectCommandUsesExactInvocationAndCapturesExit(t *testing.T) {
	executable := directCommandTestExecutable(t)
	cwd := t.TempDir()
	environment := []string{
		"DIRECT_COMMAND_EXPECT_CWD=" + cwd,
		"DIRECT_COMMAND_HELPER_MODE=exact",
	}
	result, err := runDirectCommand(
		context.Background(),
		executable,
		[]string{directCommandHelperArgument},
		environment,
		cwd,
		5*time.Second,
	)
	if err != nil || !result.Started || !result.WaitCompleted || !result.Exited || result.ExitCode != 0 ||
		!result.ProcessCleanupDone || !result.PipesDrained || result.Cancelled || result.TimedOut ||
		result.OutputLimitExceeded || string(result.Stdout) != "discovery\n" || len(result.Stderr) != 0 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}

	result, err = runDirectCommand(
		context.Background(),
		executable,
		[]string{directCommandHelperArgument},
		[]string{"DIRECT_COMMAND_HELPER_MODE=exit"},
		cwd,
		5*time.Second,
	)
	if err != nil || !result.WaitCompleted || !result.Exited || result.ExitCode != 23 || !result.ProcessCleanupDone || !result.PipesDrained {
		t.Fatalf("nonzero result = %#v, error = %v", result, err)
	}
}

func TestRunDirectCommandCancellationKillsAndQuiescesGroup(t *testing.T) {
	executable := directCommandTestExecutable(t)
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	resultDone := make(chan directCommandResult, 1)
	errorDone := make(chan error, 1)
	go func() {
		result, err := runDirectCommand(
			ctx,
			executable,
			[]string{directCommandHelperArgument},
			[]string{"DIRECT_COMMAND_HELPER_MODE=hold", "DIRECT_COMMAND_READY=" + ready},
			filepath.Dir(ready),
			5*time.Second,
		)
		resultDone <- result
		errorDone <- err
	}()
	waitForDirectCommandFile(t, ready)
	cancel()
	result, err := <-resultDone, <-errorDone
	if !errors.Is(err, errDirectCommandCancelled) || !errors.Is(err, context.Canceled) ||
		!result.Cancelled || result.TimedOut || !result.WaitCompleted || !result.ProcessCleanupDone || !result.PipesDrained || result.Exited || result.ExitCode != -1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	assertDirectCommandProcessGone(t, readDirectCommandPID(t, ready))
}

func TestRunDirectCommandTimeoutKillsAndQuiescesGroup(t *testing.T) {
	executable := directCommandTestExecutable(t)
	ready := filepath.Join(t.TempDir(), "ready")
	result, err := runDirectCommand(
		context.Background(),
		executable,
		[]string{directCommandHelperArgument},
		[]string{"DIRECT_COMMAND_HELPER_MODE=hold", "DIRECT_COMMAND_READY=" + ready},
		filepath.Dir(ready),
		200*time.Millisecond,
	)
	if !errors.Is(err, errDirectCommandTimeout) || result.Cancelled || !result.TimedOut ||
		!result.WaitCompleted || !result.ProcessCleanupDone || !result.PipesDrained || result.Exited || result.ExitCode != -1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	assertDirectCommandProcessGone(t, readDirectCommandPID(t, ready))
}

func TestRunDirectCommandOutputLimitKillsAndQuiescesGroup(t *testing.T) {
	result, err := runDirectCommand(
		context.Background(),
		directCommandTestExecutable(t),
		[]string{directCommandHelperArgument},
		[]string{"DIRECT_COMMAND_HELPER_MODE=output"},
		t.TempDir(),
		5*time.Second,
	)
	if !errors.Is(err, errDirectCommandOutputLimit) || !result.OutputLimitExceeded ||
		!result.WaitCompleted || !result.ProcessCleanupDone || !result.PipesDrained || len(result.Stdout) != int(directCommandOutputLimit) {
		t.Fatalf("exit=%d started=%v exited=%v cleanup=%v drained=%v exceeded=%v stdout=%d stderr=%d error=%v",
			result.ExitCode, result.Started, result.Exited, result.ProcessCleanupDone, result.PipesDrained,
			result.OutputLimitExceeded, len(result.Stdout), len(result.Stderr), err)
	}
}

func TestRunDirectCommandCleansDescendantAfterNormalExit(t *testing.T) {
	executable := directCommandTestExecutable(t)
	ready := filepath.Join(t.TempDir(), "ready")
	result, err := runDirectCommand(
		context.Background(),
		executable,
		[]string{directCommandHelperArgument},
		[]string{"DIRECT_COMMAND_HELPER_MODE=descendant", "DIRECT_COMMAND_READY=" + ready},
		filepath.Dir(ready),
		5*time.Second,
	)
	if err != nil || result.ExitCode != 0 || !result.WaitCompleted || !result.ProcessCleanupDone || !result.PipesDrained {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	assertDirectCommandProcessGone(t, readDirectCommandPID(t, ready))
}

func TestRunDirectCommandCancellationBoundsStuckDescendantHoldingPipes(t *testing.T) {
	temporary := t.TempDir()
	leaderReady := filepath.Join(temporary, "leader-ready")
	descendantReady := filepath.Join(temporary, "descendant-ready")
	executable := directCommandTestExecutable(t)
	ctx, cancel := context.WithCancel(context.Background())
	resultDone := make(chan directCommandResult, 1)
	errorDone := make(chan error, 1)
	go func() {
		result, err := runDirectCommand(
			ctx,
			executable,
			[]string{directCommandHelperArgument},
			[]string{
				"DIRECT_COMMAND_HELPER_MODE=stuck-descendant-pipe",
				"DIRECT_COMMAND_LEADER_READY=" + leaderReady,
				"DIRECT_COMMAND_READY=" + descendantReady,
			},
			temporary,
			5*time.Second,
		)
		resultDone <- result
		errorDone <- err
	}()
	waitForDirectCommandFile(t, leaderReady)
	waitForDirectCommandFile(t, descendantReady)
	started := time.Now()
	cancel()
	result, err := <-resultDone, <-errorDone
	if elapsed := time.Since(started); elapsed > directCommandShutdownLimit+500*time.Millisecond {
		t.Fatalf("cancellation exceeded shutdown bound: %s", elapsed)
	}
	if !errors.Is(err, errDirectCommandCancelled) || !result.WaitCompleted ||
		!result.ProcessCleanupDone || !result.PipesDrained {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	assertDirectCommandProcessGone(t, readDirectCommandPID(t, leaderReady))
	assertDirectCommandProcessGone(t, readDirectCommandPID(t, descendantReady))
}

func TestRunDirectCommandBoundsStuckWaitAtShutdownDeadline(t *testing.T) {
	original := directCommandWait
	release := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		directCommandWait = original
	})
	directCommandWait = func(command *exec.Cmd) error {
		err := command.Wait()
		<-release
		return err
	}
	timeout := 50 * time.Millisecond
	started := time.Now()
	result, err := runDirectCommand(
		context.Background(),
		directCommandTestExecutable(t),
		[]string{directCommandHelperArgument},
		[]string{"DIRECT_COMMAND_HELPER_MODE=exit"},
		t.TempDir(),
		timeout,
	)
	if elapsed := time.Since(started); elapsed > timeout+directCommandShutdownLimit+500*time.Millisecond {
		t.Fatalf("stuck Wait exceeded run plus shutdown bound: %s", elapsed)
	}
	if !errors.Is(err, errDirectCommandTimeout) || !errors.Is(err, errDirectCommandWaitDeadline) ||
		result.WaitCompleted || result.ProcessCleanupDone || !result.PipesDrained || !result.TimedOut {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}

func TestCleanupDirectCommandGroupSurfacesFailure(t *testing.T) {
	original := directCommandSignalProcessGroup
	t.Cleanup(func() { directCommandSignalProcessGroup = original })
	directCommandSignalProcessGroup = func(pid int, signal syscall.Signal) error {
		if pid != -12345 || (signal != syscall.SIGTERM && signal != syscall.SIGKILL && signal != 0) {
			t.Fatalf("kill = (%d, %v)", pid, signal)
		}
		return syscall.EPERM
	}
	err := cleanupDirectCommandGroup(12345, time.Now().Add(20*time.Millisecond))
	if !errors.Is(err, syscall.EPERM) || !errors.Is(err, errDirectCommandNotQuiescent) {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestRunDirectCommandRejectsInvalidOrCancelledRequestBeforeLaunch(t *testing.T) {
	result, err := runDirectCommand(context.Background(), "go", nil, nil, t.TempDir(), time.Second)
	if !errors.Is(err, errDirectCommandInvalid) || result.Started {
		t.Fatalf("invalid result = %#v, error = %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = runDirectCommand(ctx, directCommandTestExecutable(t), nil, nil, t.TempDir(), time.Second)
	if !errors.Is(err, errDirectCommandCancelled) || !errors.Is(err, context.Canceled) || result.Started || !result.Cancelled {
		t.Fatalf("cancelled result = %#v, error = %v", result, err)
	}
}

func directCommandTestExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(executable)
}

func waitForDirectCommandFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		raw, err := os.ReadFile(path)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && parseErr == nil && pid > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for PID handoff at %s: read=%v parse=%v value=%q", path, err, parseErr, raw)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readDirectCommandPID(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid PID handoff %q: %v", raw, err)
	}
	return pid
}

func assertDirectCommandProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d survived cleanup: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
