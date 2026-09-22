//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gorunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMain(main *testing.M) {
	if os.Getenv("GORUNNER_DESCENDANT") == "1" {
		runDescendantHelper()
		os.Exit(0)
	}
	if os.Getenv("GORUNNER_HELPER") == "1" {
		os.Exit(runGoHelper())
	}
	os.Exit(main.Run())
}

func runGoHelper() int {
	mode := os.Getenv("GORUNNER_HELPER_MODE")
	switch mode {
	case "echo":
		expected := []string{"test", "-json", "-count=1", "-vet=off", "example.com/a", "example.com/b"}
		if !reflect.DeepEqual(os.Args[1:], expected) {
			_, _ = fmt.Fprintf(os.Stderr, "argv=%q", os.Args[1:])
			return 90
		}
		cwd, err := os.Getwd()
		expectedCWD, resolveErr := filepath.EvalSymlinks(os.Getenv("GORUNNER_EXPECT_CWD"))
		if err != nil || resolveErr != nil || cwd != expectedCWD || os.Getenv("GORUNNER_AMBIENT") != "" {
			return 91
		}
		_, _ = os.Stdout.WriteString("stdout\n")
		_, _ = os.Stderr.WriteString("stderr\n")
		return 7
	case "sleep":
		if ready := os.Getenv("GORUNNER_READY"); ready != "" {
			_ = os.WriteFile(ready, []byte("ready"), 0o600)
		}
		time.Sleep(30 * time.Second)
		return 0
	case "flood":
		block := []byte(strings.Repeat("x", 32<<10))
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			for {
				if _, err := os.Stdout.Write(block); err != nil {
					return
				}
			}
		}()
		go func() {
			defer workers.Done()
			for {
				if _, err := os.Stderr.Write(block); err != nil {
					return
				}
			}
		}()
		workers.Wait()
		return 0
	case "stdout-flood":
		block := []byte(strings.Repeat("x", 32<<10))
		for {
			if _, err := os.Stdout.Write(block); err != nil {
				return 0
			}
		}
	case "descendant":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "GORUNNER_DESCENDANT=1")
		if err := child.Start(); err != nil {
			return 92
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("GORUNNER_READY")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				return 93
			}
			time.Sleep(10 * time.Millisecond)
		}
		return 0
	case "churning-descendant":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(),
			"GORUNNER_DESCENDANT=1",
			"GORUNNER_DESCENDANT_MODE=churn",
		)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			return 97
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("GORUNNER_READY")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				return 98
			}
			time.Sleep(time.Millisecond)
		}
		time.Sleep(30 * time.Second)
		return 0
	case "escaped-pipe":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "GORUNNER_DESCENDANT=1")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			return 95
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("GORUNNER_READY")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				return 96
			}
			time.Sleep(10 * time.Millisecond)
		}
		return 7
	default:
		return 94
	}
}

func runDescendantHelper() {
	ready := os.Getenv("GORUNNER_READY")
	_ = os.WriteFile(ready, []byte(strconv.Itoa(os.Getpid())), 0o600)
	if os.Getenv("GORUNNER_DESCENDANT_MODE") == "churn" {
		directory := os.Getenv("GORUNNER_CHURN_DIR")
		for index := 0; ; index++ {
			path := filepath.Join(directory, strconv.Itoa(index%32))
			_ = os.WriteFile(path, []byte("active"), 0o600)
		}
	}
	time.Sleep(30 * time.Second)
}

func helperPlan(t *testing.T, mode string, extra ...EnvironmentVariable) Plan {
	t.Helper()
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	environment := []EnvironmentVariable{
		{Name: "GORUNNER_HELPER", Value: "1"},
		{Name: "GORUNNER_HELPER_MODE", Value: mode},
	}
	environment = append(environment, extra...)
	sortEnvironment(environment)
	return Plan{
		GoExecutable:     executable,
		WorkingDirectory: t.TempDir(),
		Environment:      environment,
		Packages:         []string{"example.com/a", "example.com/b"},
		OutputLimitBytes: 1 << 20,
		RetainOutput:     true,
		Timeout:          5 * time.Second,
	}
}

func TestRunOmitsTransientOutputWhenRetentionIsDisabled(t *testing.T) {
	plan := helperPlan(t, "echo")
	plan.Environment = append(plan.Environment, EnvironmentVariable{Name: "GORUNNER_EXPECT_CWD", Value: plan.WorkingDirectory})
	sortEnvironment(plan.Environment)
	plan.RetainOutput = false
	result, err := Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout.Data != nil || result.Stderr.Data != nil {
		t.Fatalf("streams = (%#v, %#v)", result.Stdout, result.Stderr)
	}
	if result.Stdout.RawSHA256 != digestString("stdout\n") || result.Stderr.RawSHA256 != digestString("stderr\n") {
		t.Fatalf("digests = (%s, %s)", result.Stdout.RawSHA256, result.Stderr.RawSHA256)
	}
}

func sortEnvironment(environment []EnvironmentVariable) {
	for index := 1; index < len(environment); index++ {
		for cursor := index; cursor > 0 && environment[cursor].Name < environment[cursor-1].Name; cursor-- {
			environment[cursor], environment[cursor-1] = environment[cursor-1], environment[cursor]
		}
	}
}

func TestRunUsesFixedArgvExplicitCwdEnvAndRawDigests(t *testing.T) {
	t.Setenv("GORUNNER_AMBIENT", "must-not-leak")
	plan := helperPlan(t, "echo")
	plan.Environment = append(plan.Environment, EnvironmentVariable{Name: "GORUNNER_EXPECT_CWD", Value: plan.WorkingDirectory})
	sortEnvironment(plan.Environment)
	result, err := Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 || !result.Exited || result.TimedOut || result.Cancelled || result.OutputLimitExceeded {
		t.Fatalf("result = %#v", result)
	}
	if string(result.Stdout.Data) != "stdout\n" || string(result.Stderr.Data) != "stderr\n" || !result.Stdout.Drained || !result.Stderr.Drained {
		t.Fatalf("streams = (%#v, %#v)", result.Stdout, result.Stderr)
	}
	if result.Stdout.RawSHA256 != digestString("stdout\n") || result.Stderr.RawSHA256 != digestString("stderr\n") {
		t.Fatalf("digests = (%s, %s)", result.Stdout.RawSHA256, result.Stderr.RawSHA256)
	}
}

func TestRunCancelsOwnedProcessGroup(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	plan := helperPlan(t, "sleep", EnvironmentVariable{Name: "GORUNNER_READY", Value: ready})
	ctx, cancel := context.WithCancel(context.Background())
	resultDone := make(chan Result, 1)
	errorDone := make(chan error, 1)
	go func() {
		result, err := Run(ctx, plan)
		resultDone <- result
		errorDone <- err
	}()
	waitForFile(t, ready)
	cancel()
	result := <-resultDone
	if err := <-errorDone; err != nil {
		t.Fatal(err)
	}
	if !result.Cancelled || result.TimedOut {
		t.Fatalf("result = %#v", result)
	}
	if !result.ProcessCleanupDone {
		t.Fatalf("cleanup was incomplete: %#v", result)
	}
}

// afterFuncFirstContext cancels its children through AfterFunc while its Done
// channel stays open, so the process kill always beats Run's ctx.Done case.
type afterFuncFirstContext struct {
	context.Context
	done      chan struct{}
	mu        sync.Mutex
	fired     bool
	callbacks []func()
}

func (ctx *afterFuncFirstContext) Done() <-chan struct{} { return ctx.done }

func (ctx *afterFuncFirstContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.fired {
		return context.Canceled
	}
	return nil
}

func (ctx *afterFuncFirstContext) AfterFunc(callback func()) func() bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.callbacks = append(ctx.callbacks, callback)
	return func() bool { return false }
}

func (ctx *afterFuncFirstContext) fire() {
	ctx.mu.Lock()
	ctx.fired = true
	callbacks := ctx.callbacks
	ctx.mu.Unlock()
	for _, callback := range callbacks {
		callback()
	}
}

// GLTP-V0-048: a cancellation that kills the process before Run observes
// ctx.Done still reports Cancelled, not a signal-terminated process.
func TestRunReportsCancelThatWinsTheKillButLosesTheSelect(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	plan := helperPlan(t, "sleep", EnvironmentVariable{Name: "GORUNNER_READY", Value: ready})
	plan.Timeout = time.Minute
	ctx := &afterFuncFirstContext{Context: context.Background(), done: make(chan struct{})}
	defer close(ctx.done)
	resultDone := make(chan Result, 1)
	errorDone := make(chan error, 1)
	go func() {
		result, err := Run(ctx, plan)
		resultDone <- result
		errorDone <- err
	}()
	waitForFile(t, ready)
	ctx.fire()
	result := <-resultDone
	if err := <-errorDone; err != nil {
		t.Fatal(err)
	}
	if !result.Cancelled || result.TimedOut || result.Exited {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunTimesOut(t *testing.T) {
	plan := helperPlan(t, "sleep")
	plan.Timeout = 100 * time.Millisecond
	result, err := Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TimedOut || result.Cancelled {
		t.Fatalf("result = %#v", result)
	}
	if !result.ProcessCleanupDone {
		t.Fatalf("cleanup was incomplete: %#v", result)
	}
}

func TestRunBoundsAndConcurrentlyDrainsOutput(t *testing.T) {
	plan := helperPlan(t, "flood")
	plan.OutputLimitBytes = 4_096
	result, err := Run(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OutputLimitExceeded {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Stdout.Data)+len(result.Stderr.Data) > int(plan.OutputLimitBytes) {
		t.Fatalf("retained output = %d", len(result.Stdout.Data)+len(result.Stderr.Data))
	}
	if result.Stdout.Bytes+result.Stderr.Bytes <= uint64(plan.OutputLimitBytes) || !result.Stdout.Drained || !result.Stderr.Drained {
		t.Fatalf("streams = (%#v, %#v)", result.Stdout, result.Stderr)
	}
}

func TestRunCleansInheritedDescendantAfterNormalExit(t *testing.T) {
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	plan := helperPlan(t, "descendant", EnvironmentVariable{Name: "GORUNNER_READY", Value: ready})
	result, err := Run(context.Background(), plan)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	rawPID, err := os.ReadFile(ready)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(rawPID))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d survived cleanup: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunBoundsEscapedDescendantHoldingPipes(t *testing.T) {
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	plan := helperPlan(t, "escaped-pipe", EnvironmentVariable{Name: "GORUNNER_READY", Value: ready})
	plan.Timeout = 10 * time.Second
	started := time.Now()
	result, err := Run(context.Background(), plan)
	if !errors.Is(err, ErrPipeWait) || !result.PipeWaitExpired || result.ExitCode != 7 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if elapsed := time.Since(started); elapsed > MaxPipeWait+2*time.Second {
		t.Fatalf("escaped pipe delayed return for %s", elapsed)
	}
	rawPID, readErr := os.ReadFile(ready)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, parseErr := strconv.Atoi(string(rawPID))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if killErr := syscall.Kill(pid, syscall.SIGKILL); killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
		t.Fatal(killErr)
	}
}

func TestRunRepeatedCancellationWaitsForChurningGroup(t *testing.T) {
	for iteration := range 5 {
		t.Run(strconv.Itoa(iteration), func(t *testing.T) {
			runChurningGroup(t, false)
		})
	}
}

func TestRunRepeatedTimeoutWaitsForChurningGroup(t *testing.T) {
	for iteration := range 5 {
		t.Run(strconv.Itoa(iteration), func(t *testing.T) {
			runChurningGroup(t, true)
		})
	}
}

func runChurningGroup(t *testing.T, timeout bool) {
	t.Helper()
	temporary := t.TempDir()
	ready := filepath.Join(temporary, "ready")
	churn := filepath.Join(temporary, "churn")
	if err := os.Mkdir(churn, 0o700); err != nil {
		t.Fatal(err)
	}
	plan := helperPlan(t, "churning-descendant",
		EnvironmentVariable{Name: "GORUNNER_CHURN_DIR", Value: churn},
		EnvironmentVariable{Name: "GORUNNER_READY", Value: ready},
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if timeout {
		// 100ms left too little runway for the churning-descendant helper to
		// fork and write its ready file before Run's own deadline fired
		// under host load, so this asserted setup failure ("did not start")
		// rather than exercising cleanup. 1s keeps this a real timeout while
		// giving the helper a realistic amount of time to start.
		plan.Timeout = time.Second
		result, err := Run(ctx, plan)
		if _, readyErr := os.Stat(ready); readyErr != nil {
			t.Fatalf("churning descendant did not start before timeout: %v", readyErr)
		}
		assertChurningGroupStopped(t, result, err, churn, false)
		return
	}
	resultDone := make(chan Result, 1)
	errorDone := make(chan error, 1)
	go func() {
		result, err := Run(ctx, plan)
		resultDone <- result
		errorDone <- err
	}()
	waitForFile(t, ready)
	started := time.Now()
	cancel()
	result := <-resultDone
	err := <-errorDone
	if elapsed := time.Since(started); elapsed > MaxPipeWait+time.Second {
		t.Fatalf("cancellation exceeded shutdown bound: %s", elapsed)
	}
	assertChurningGroupStopped(t, result, err, churn, true)
}

func assertChurningGroupStopped(t *testing.T, result Result, err error, churn string, cancelled bool) {
	t.Helper()
	if err != nil || !result.ProcessCleanupDone || !result.Stdout.Drained || !result.Stderr.Drained ||
		result.Cancelled != cancelled || result.TimedOut == cancelled {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if err := os.RemoveAll(churn); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := os.Stat(churn); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("descendant recreated files after qualified cleanup: %v", err)
	}
}

func TestCleanupCommandSurfacesProcessGroupFailure(t *testing.T) {
	original := signalProcessGroup
	t.Cleanup(func() { signalProcessGroup = original })
	signalProcessGroup = func(pid int, signal syscall.Signal) error {
		if pid != -12345 || (signal != syscall.SIGKILL && signal != 0) {
			t.Fatalf("kill = (%d, %v)", pid, signal)
		}
		return syscall.EPERM
	}
	command := &exec.Cmd{Process: mustFindProcess(t, 12345)}
	if err := cleanupCommand(command, time.Now().Add(20*time.Millisecond)); !errors.Is(err, syscall.EPERM) || !errors.Is(err, errProcessGroupNotQuiescent) {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestCleanupCommandWaitsForProcessGroupDisappearance(t *testing.T) {
	original := signalProcessGroup
	t.Cleanup(func() { signalProcessGroup = original })
	polls := 0
	signalProcessGroup = func(pid int, signal syscall.Signal) error {
		if pid != -12345 {
			t.Fatalf("pid = %d", pid)
		}
		if signal == syscall.SIGKILL {
			return nil
		}
		if signal != 0 {
			t.Fatalf("signal = %v", signal)
		}
		polls++
		if polls >= 3 {
			return syscall.ESRCH
		}
		return nil
	}
	command := &exec.Cmd{Process: mustFindProcess(t, 12345)}
	if err := cleanupCommand(command, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if polls < 4 {
		t.Fatalf("quiescence was not confirmed: polls = %d", polls)
	}
}

func TestCleanupCommandBoundsNonQuiescentProcessGroup(t *testing.T) {
	original := signalProcessGroup
	t.Cleanup(func() { signalProcessGroup = original })
	signalProcessGroup = func(_ int, _ syscall.Signal) error { return nil }
	command := &exec.Cmd{Process: mustFindProcess(t, 12345)}
	started := time.Now()
	err := cleanupCommand(command, time.Now().Add(20*time.Millisecond))
	if !errors.Is(err, errProcessGroupNotQuiescent) {
		t.Fatalf("cleanup error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("cleanup exceeded deadline by %s", elapsed)
	}
}

func TestRunSurfacesNormalExitCleanupFailure(t *testing.T) {
	plan := helperPlan(t, "echo")
	plan.Environment = append(plan.Environment, EnvironmentVariable{Name: "GORUNNER_EXPECT_CWD", Value: plan.WorkingDirectory})
	sortEnvironment(plan.Environment)
	original := signalProcessGroup
	t.Cleanup(func() { signalProcessGroup = original })
	signalProcessGroup = func(int, syscall.Signal) error { return syscall.EPERM }
	result, err := Run(context.Background(), plan)
	if !errors.Is(err, ErrContainment) || !errors.Is(err, syscall.EPERM) {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if result.ProcessCleanupDone {
		t.Fatalf("cleanup unexpectedly complete: %#v", result)
	}
}

func mustFindProcess(t *testing.T, pid int) *os.Process {
	t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	return process
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s after %s", path, 30*time.Second)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
