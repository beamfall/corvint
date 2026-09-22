package procgroup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestRunProcessNormalExactBytesAndUmask(t *testing.T) {
	fixture := t.TempDir()
	created := filepath.Join(fixture, "created")
	observation := Run(context.Background(), processHelperSpec(t, fixture, "normal", "PROCESS_CREATED="+created))
	if observation.Err != nil {
		t.Fatal(observation.Err)
	}
	if observation.ExitStatus != 0 || string(observation.Stdout) != "stdout\x00\n" || string(observation.Stderr) != "stderr\x00\n" {
		t.Fatalf("unexpected result: exit=%d stdout=%q stderr=%q", observation.ExitStatus, observation.Stdout, observation.Stderr)
	}
	if !observation.Started || !observation.WaitCompleted || !observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("incomplete observation: %+v", observation)
	}
	info, err := os.Stat(created)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("fixed umask: got mode %04o, want 0644", got)
	}
}

func TestRunProcessNonzero(t *testing.T) {
	observation := Run(context.Background(), processHelperSpec(t, t.TempDir(), "nonzero"))
	if observation.Err != nil || observation.ExitStatus != 23 {
		t.Fatalf("exit=%d err=%v", observation.ExitStatus, observation.Err)
	}
	if !observation.WaitCompleted || !observation.PipesDrained || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("incomplete observation: %+v", observation)
	}
}

func TestRunProcessTimeout(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "hold")
	spec.Timeout = 50 * time.Millisecond
	observation := Run(context.Background(), spec)
	assertProcessErrorCode(t, observation.Err, "process-timeout")
	if !observation.TimedOut || observation.Cancelled || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("unexpected timeout observation: %+v", observation)
	}
}

func TestRunProcessShutdownDeadlineLeavesExitUnobserved(t *testing.T) {
	fixture := t.TempDir()
	ready := filepath.Join(fixture, "ready")
	spec := processHelperSpec(t, fixture, "hold", "PROCESS_READY="+ready)
	spec.ShutdownTimeout = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	neverExitObserved := make(chan error)
	neverWaitDone := make(chan error)
	done := make(chan Observation, 1)
	go func() {
		done <- run(ctx, spec, processWaitChannels{
			exitObserved: neverExitObserved,
			waitDone:     neverWaitDone,
		})
	}()
	waitForProcessFile(t, ready)
	cancel()
	var observation Observation
	select {
	case observation = <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown exceeded its deterministic test bound")
	}
	assertProcessErrorCode(t, observation.Err, "process-wait-timeout")
	if !observation.Started || !observation.Cancelled {
		t.Fatalf("process was not cancelled after launch: %+v", observation)
	}
	if observation.ExitObserved || observation.WaitCompleted || observation.ExitStatus != -1 {
		t.Fatalf("unobserved exit was collapsed into an exit status: %+v", observation)
	}
}

func TestRunProcessContextCancellation(t *testing.T) {
	fixture := t.TempDir()
	ready := filepath.Join(fixture, "ready")
	spec := processHelperSpec(t, fixture, "hold", "PROCESS_READY="+ready)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan Observation, 1)
	go func() { done <- Run(ctx, spec) }()
	waitForProcessFile(t, ready)
	cancel()
	observation := <-done
	assertProcessErrorCode(t, observation.Err, "process-cancelled")
	if !observation.Cancelled || observation.TimedOut || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("unexpected cancellation observation: %+v", observation)
	}
}

func TestRunProcessOutputOverflow(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "overflow")
	spec.OutputLimit = 128
	observation := Run(context.Background(), spec)
	assertProcessErrorCode(t, observation.Err, "process-output-overflow")
	if !observation.OutputOverflow || len(observation.Stdout) != spec.OutputLimit || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("unexpected overflow observation: %+v", observation)
	}
}

func TestRunProcessReapsDescendantBeforeDelayedSideEffect(t *testing.T) {
	fixture := t.TempDir()
	pidFile := filepath.Join(fixture, "descendant.pid")
	sideEffect := filepath.Join(fixture, "late")
	spec := processHelperSpec(t, fixture, "descendant", "PROCESS_PID_FILE="+pidFile, "PROCESS_SIDE_EFFECT="+sideEffect)
	observation := Run(context.Background(), spec)
	if observation.Err != nil || observation.ExitStatus != 0 || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("unexpected descendant observation: %+v", observation)
	}
	leaderPID, descendantPID := readProcessPIDs(t, pidFile)
	if groupID, err := processGroupID(descendantPID); err == nil && groupID != leaderPID {
		t.Fatalf("descendant pid %d escaped owned process group %d into %d", descendantPID, leaderPID, groupID)
	}
	waitForProcessGone(t, descendantPID)
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(sideEffect); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("descendant produced delayed side effect: %v", err)
	}
}

func TestRunProcessSetsidEscapeQualificationIsPartial(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "setsid-escape")
	spec.RequireDescendantCleanup = true
	observation := Run(context.Background(), spec)
	assertProcessErrorCode(t, observation.Err, descendantCleanupUnsupported)
	var failure *processError
	if !errors.As(observation.Err, &failure) || failure.Verdict != processVerdictPartial {
		t.Fatalf("qualification=%+v, want typed PARTIAL", observation.Err)
	}
	if observation.Started || observation.OwnedProcessGroupCleanup || observation.DescendantCleanupStatus != descendantCleanupUnsupported || observation.DescendantCleanupQualification != processVerdictPartial {
		t.Fatalf("setsid escape was overclaimed: %+v", observation)
	}
}

func TestRunProcessFastExitDoesNotSignalReusedProcessGroups(t *testing.T) {
	fixture := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	spec := Spec{
		Argv:            []string{executable, "-test.run=^TestProcessHelper$", "--", "fast"},
		Dir:             fixture,
		Env:             []string{"PROCESS_HELPER=1"},
		Timeout:         2 * time.Second,
		ShutdownTimeout: time.Second,
		InputLimit:      1,
		OutputLimit:     1,
	}
	verify := func() error {
		observation := Run(context.Background(), spec)
		if observation.Err != nil {
			return observation.Err
		}
		if observation.ExitStatus != 0 || !observation.WaitCompleted || !observation.OwnedProcessGroupCleanup {
			return fmt.Errorf("incomplete fast-process observation: %+v", observation)
		}
		return nil
	}
	for index := 0; index < 32; index++ {
		if err := verify(); err != nil {
			t.Fatalf("sequential invocation %d: %v", index, err)
		}
	}

	const workers = 16
	const invocationsPerWorker = 8
	failures := make(chan error, workers)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for invocation := 0; invocation < invocationsPerWorker; invocation++ {
				if err := verify(); err != nil {
					failures <- err
					return
				}
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
}

func TestRunProcessQuiescenceIgnoresReusedPIDIdentity(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "fast")
	spec.ShutdownTimeout = 20 * time.Millisecond
	reads := 0
	observation := run(context.Background(), spec, processWaitChannels{
		identity: func(context.Context, int) (string, error) {
			reads++
			if reads == 1 {
				return "original-start", nil
			}
			return "reused-start", nil
		},
		groupProbe: func(int, syscall.Signal) error { return nil },
	})
	if observation.Err != nil || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("reused PID identity caused a false cleanup failure: %+v", observation)
	}
}

// TestRunProcessQuiescenceTreatsPermissionDeniedAsPresent: a post-reap
// signal-0 probe answering EPERM means a group member still exists that this
// process cannot signal; with an unchanged leader identity that is not a
// quiescence proof (CRR-V0-003).
func TestRunProcessQuiescenceTreatsPermissionDeniedAsPresent(t *testing.T) {
	spec := processHelperSpec(t, t.TempDir(), "fast")
	spec.ShutdownTimeout = 30 * time.Millisecond
	observation := run(context.Background(), spec, processWaitChannels{
		identity:   func(context.Context, int) (string, error) { return "original-start", nil },
		groupProbe: func(int, syscall.Signal) error { return syscall.EPERM },
	})
	if observation.OwnedProcessGroupCleanup || observation.Err == nil {
		t.Fatalf("an EPERM group probe was reported as owned cleanup: %+v", observation)
	}
}

func TestRunProcessQuiescenceDisclosesIdentityFallback(t *testing.T) {
	t.Run("initial read unavailable", func(t *testing.T) {
		assertProcessIdentityFallback(t,
			func(context.Context, int) (string, error) { return "", errors.New("unavailable") },
			func(int, syscall.Signal) error { return syscall.ESRCH },
		)
	})
	t.Run("post-reap read unavailable", func(t *testing.T) {
		reads := 0
		probes := 0
		assertProcessIdentityFallback(t,
			func(context.Context, int) (string, error) {
				reads++
				if reads == 1 {
					return "original-start", nil
				}
				return "", errors.New("unavailable")
			},
			func(int, syscall.Signal) error {
				probes++
				if probes == 1 {
					return nil
				}
				return syscall.ESRCH
			},
		)
	})
}

func assertProcessIdentityFallback(t *testing.T, identity func(context.Context, int) (string, error), groupProbe func(int, syscall.Signal) error) {
	t.Helper()
	spec := processHelperSpec(t, t.TempDir(), "fast")
	spec.ShutdownTimeout = 30 * time.Millisecond
	observation := run(context.Background(), spec, processWaitChannels{identity: identity, groupProbe: groupProbe})
	if observation.Err != nil || !observation.OwnedProcessGroupCleanup {
		t.Fatalf("signal-only fallback failed: %+v", observation)
	}
	if observation.DescendantCleanupQualification != processVerdictPartial {
		t.Fatalf("fallback qualification = %q, want %q", observation.DescendantCleanupQualification, processVerdictPartial)
	}
}

func processHelperSpec(t *testing.T, directory, mode string, environment ...string) Spec {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Spec{
		Argv:            []string{executable, "-test.run=^TestProcessHelper$", "--", mode},
		Dir:             directory,
		Env:             append([]string{"PROCESS_HELPER=1"}, environment...),
		Timeout:         2 * time.Second,
		ShutdownTimeout: time.Second,
		InputLimit:      1024,
		OutputLimit:     1024,
	}
}

func assertProcessErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var failure *processError
	if errors.As(err, &failure) && failure.Code == code {
		return
	}
	t.Fatalf("error %v does not contain code %q", err, code)
}

func waitForProcessFile(t *testing.T, path string) {
	t.Helper()
	// This readiness wait is a hang detector, not a performance budget
	// (decision 0082).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("file did not appear: %s", path)
}

func readProcessPIDs(t *testing.T, path string) (int, int) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(content))
	if len(fields) != 2 {
		t.Fatalf("invalid process IDs %q", content)
	}
	leaderPID, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	descendantPID, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatal(err)
	}
	return leaderPID, descendantPID
}

func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("descendant pid %d survived cleanup", pid)
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("PROCESS_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(125)
	}
	mode := os.Args[separator+1]
	if ready := os.Getenv("PROCESS_READY"); ready != "" {
		if err := os.WriteFile(ready, []byte("ready"), 0o666); err != nil {
			os.Exit(125)
		}
	}
	switch mode {
	case "normal":
		if err := os.WriteFile(os.Getenv("PROCESS_CREATED"), []byte("created"), 0o666); err != nil {
			os.Exit(125)
		}
		_, _ = os.Stdout.Write([]byte("stdout\x00\n"))
		_, _ = os.Stderr.Write([]byte("stderr\x00\n"))
	case "nonzero":
		os.Exit(23)
	case "fast":
	case "hold":
		time.Sleep(10 * time.Second)
	case "overflow":
		_, _ = os.Stdout.Write([]byte(strings.Repeat("x", 4096)))
		time.Sleep(time.Second)
	case "bounded-output":
		streams := os.Getenv("PROCESS_STREAMS")
		if streams != "stderr" {
			_, _ = os.Stdout.Write([]byte(strings.Repeat("o", 7)))
		}
		if streams != "stdout" {
			_, _ = os.Stderr.Write([]byte(strings.Repeat("e", 11)))
		}
		if os.Getenv("PROCESS_HOLD_AFTER_OUTPUT") == "1" {
			time.Sleep(10 * time.Second)
		}
		if err := os.WriteFile(os.Getenv("PROCESS_COMPLETE"), []byte("complete"), 0o600); err != nil {
			os.Exit(125)
		}
		code, _ := strconv.Atoi(os.Getenv("PROCESS_EXIT"))
		os.Exit(code)
	case "supervisor":
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
		executable, err := os.Executable()
		if err != nil {
			os.Exit(125)
		}
		o := Run(ctx, Spec{Argv: []string{executable, "-test.run=^TestProcessHelper$", "--", "descendant-hold"}, Dir: filepath.Dir(os.Getenv("PROCESS_PID_FILE")), Env: []string{"PROCESS_HELPER=1", "PROCESS_PID_FILE=" + os.Getenv("PROCESS_PID_FILE"), "PROCESS_SIDE_EFFECT=" + os.Getenv("PROCESS_SIDE_EFFECT")}, Timeout: 30 * time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1024})
		stop()
		if !o.Cancelled || !o.ExitObserved || !o.OwnedProcessGroupCleanup {
			os.Exit(125)
		}
	case "supervisor-join":
		// No signal handling: this leader is deliberately uncooperative, so
		// the only thing that can stop its nested descendant is the OS
		// group-directed kill reaching it directly through shared group
		// membership, not any code this process runs.
		executable, err := os.Executable()
		if err != nil {
			os.Exit(125)
		}
		_ = Run(context.Background(), Spec{Argv: []string{executable, "-test.run=^TestProcessHelper$", "--", "descendant-hold"}, Dir: filepath.Dir(os.Getenv("PROCESS_PID_FILE")), Env: []string{"PROCESS_HELPER=1", "PROCESS_PID_FILE=" + os.Getenv("PROCESS_PID_FILE"), "PROCESS_SIDE_EFFECT=" + os.Getenv("PROCESS_SIDE_EFFECT")}, Timeout: 30 * time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1024, JoinAncestorProcessGroup: true})
	case "descendant-hold":
		spawnProcessDescendant(os.Getenv("PROCESS_PID_FILE"), os.Getenv("PROCESS_SIDE_EFFECT"))
		time.Sleep(10 * time.Second)
	case "descendant":
		spawnProcessDescendant(os.Getenv("PROCESS_PID_FILE"), os.Getenv("PROCESS_SIDE_EFFECT"))
	case "descendant-child":
		time.Sleep(3 * time.Second)
		_ = os.WriteFile(os.Getenv("PROCESS_SIDE_EFFECT"), []byte("escaped"), 0o666)
	case "setsid-escape":
		os.Exit(124)
	case "fill-pipe-then-signal":
		// Write well past any platform's pipe buffer before doing anything
		// else, so a parent that has not started draining yet blocks this
		// process in write(2) before it can reach the signal file below.
		if _, err := os.Stdout.Write([]byte(strings.Repeat("x", 1<<20))); err != nil {
			os.Exit(125)
		}
		if err := os.WriteFile(os.Getenv("PROCESS_SIGNAL"), []byte("signalled"), 0o600); err != nil {
			os.Exit(125)
		}
	default:
		os.Exit(125)
	}
	os.Exit(0)
}

func spawnProcessDescendant(pidFile, sideEffect string) {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(125)
	}
	command := exec.Command(executable, "-test.run=^TestProcessHelper$", "--", "descendant-child")
	command.Env = []string{"PROCESS_HELPER=1", "PROCESS_SIDE_EFFECT=" + sideEffect}
	if err := command.Start(); err != nil {
		os.Exit(125)
	}
	processes := fmt.Sprintf("%d %d", os.Getpid(), command.Process.Pid)
	// Write then rename, so waitForProcessFile never observes an empty pid file.
	if err := os.WriteFile(pidFile+".tmp", []byte(processes), 0o666); err != nil {
		_ = command.Process.Kill()
		os.Exit(125)
	}
	if err := os.Rename(pidFile+".tmp", pidFile); err != nil {
		_ = command.Process.Kill()
		os.Exit(125)
	}
}

func TestRunProcessNonPOSIXContract(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("non-POSIX contract is compiled on unsupported targets")
	}
	observation := Run(context.Background(), Spec{})
	if observation.Started || observation.DescendantCleanupStatus != descendantCleanupUnsupported {
		t.Fatalf("unsupported platform overclaimed execution: %+v", observation)
	}
}

func TestRunProcessEmptyEnvironmentDoesNotInherit(t *testing.T) {
	t.Setenv("CORVINT_AMBIENT_SECRET", "must-not-leak")
	spec := Spec{Argv: []string{"/usr/bin/env"}, Dir: t.TempDir(), Env: nil, Timeout: time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1024}
	o := Run(context.Background(), spec)
	if o.Err != nil || !o.ExitObserved || len(o.Stdout) != 0 {
		t.Fatalf("environment inherited or incomplete: %+v", o)
	}
}
func TestRunProcessSignalIsObservedDistinctly(t *testing.T) {
	o := Run(context.Background(), Spec{Argv: []string{"/bin/sh", "-c", "kill -KILL $$"}, Dir: t.TempDir(), Env: []string{}, Timeout: time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1})
	if !o.ExitObserved || o.Signal != "killed" || o.ExitStatus != -1 || o.TimedOut || o.Cancelled {
		t.Fatalf("signal collapsed: %+v", o)
	}
}
func TestRunProcessSimultaneousStreamOverflow(t *testing.T) {
	notify := make(chan struct{}, 1)
	a := newProcessCapture(1, notify, OverflowFail)
	b := newProcessCapture(1, notify, OverflowFail)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = a.Write([]byte("xx")) }()
	go func() { defer wg.Done(); _, _ = b.Write([]byte("xx")) }()
	wg.Wait()
	_, ao := a.result()
	_, bo := b.result()
	if !ao || !bo {
		t.Fatal("stream evidence collapsed")
	}
}
func TestRunProcessInterruptionLeavesNoDescendant(t *testing.T) {
	fixture := t.TempDir()
	pidFile := filepath.Join(fixture, "pids")
	late := filepath.Join(fixture, "late")
	spec := processHelperSpec(t, fixture, "descendant-hold", "PROCESS_PID_FILE="+pidFile, "PROCESS_SIDE_EFFECT="+late)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan Observation, 1)
	go func() { done <- Run(ctx, spec) }()
	finished := false
	t.Cleanup(func() {
		cancel()
		if !finished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
		}
	})
	waitForProcessFile(t, pidFile)
	_, pid := readProcessPIDs(t, pidFile)
	t.Cleanup(func() {
		if processExists(pid) {
			p, _ := os.FindProcess(pid)
			_ = p.Kill()
		}
	})
	cancel()
	var o Observation
	select {
	case o = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("interruption exceeded shutdown bound")
	}
	finished = true
	if !o.Cancelled || !o.ExitObserved || !o.OwnedProcessGroupCleanup {
		t.Fatalf("%+v", o)
	}
	waitForProcessGone(t, pid)
	if _, err := os.Stat(late); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delayed effect: %v", err)
	}
}

func TestSupervisorSignalReapsNestedOwnedGroup(t *testing.T) {
	fixture := t.TempDir()
	pidFile := filepath.Join(fixture, "pids")
	late := filepath.Join(fixture, "late")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestProcessHelper$", "--", "supervisor")
	command.Env = []string{"PROCESS_HELPER=1", "PROCESS_PID_FILE=" + pidFile, "PROCESS_SIDE_EFFECT=" + late}
	command.Dir = fixture
	configureProcessCommand(command, false)
	if err = startProcessCommand(command); err != nil {
		t.Fatal(err)
	}
	finished := false
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		if !finished {
			_ = command.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
				return
			case <-time.After(2 * time.Second):
			}
			_ = command.Process.Kill()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
	})
	waitForProcessFile(t, pidFile)
	leader, descendant := readProcessPIDs(t, pidFile)
	t.Cleanup(func() {
		for _, pid := range []int{leader, descendant} {
			if processExists(pid) {
				p, _ := os.FindProcess(pid)
				_ = p.Kill()
			}
		}
	})
	if err = command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		finished = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not shut down")
	}
	waitForProcessGone(t, leader)
	waitForProcessGone(t, descendant)
	if _, err = os.Stat(late); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("descendant survived signal: %v", err)
	}
}

// TestForcefulAncestorKillReapsJoinedNestedGroup proves the fix for the
// "nested procgroup.Run group is unreachable by the ancestor's kill" defect:
// an ancestor-style group-directed kill sent straight to the outer owned
// group, with no SIGTERM grace and no cooperation from the middle process,
// must still reach a nested Run's leader and its own descendant because they
// joined that same OS process group (JoinAncestorProcessGroup) instead of
// creating a disjoint one.
func TestForcefulAncestorKillReapsJoinedNestedGroup(t *testing.T) {
	fixture := t.TempDir()
	pidFile := filepath.Join(fixture, "pids")
	late := filepath.Join(fixture, "late")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestProcessHelper$", "--", "supervisor-join")
	command.Env = []string{"PROCESS_HELPER=1", "PROCESS_PID_FILE=" + pidFile, "PROCESS_SIDE_EFFECT=" + late}
	command.Dir = fixture
	configureProcessCommand(command, false)
	if err = startProcessCommand(command); err != nil {
		t.Fatal(err)
	}
	leaderGroup := command.Process.Pid
	finished := false
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		if !finished {
			_ = killTestProcessGroup(leaderGroup)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	})
	waitForProcessFile(t, pidFile)
	leader, descendant := readProcessPIDs(t, pidFile)
	t.Cleanup(func() {
		for _, pid := range []int{leader, descendant} {
			if processExists(pid) {
				p, _ := os.FindProcess(pid)
				_ = p.Kill()
			}
		}
	})
	// Simulate an ancestor's forceful cleanup: a bare group-directed SIGKILL,
	// no grace period, no chance for the uncooperative middle process to run
	// any cleanup of its own.
	if err = killTestProcessGroup(leaderGroup); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		finished = true
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor group did not die")
	}
	waitForProcessGone(t, leader)
	waitForProcessGone(t, descendant)
	if _, err = os.Stat(late); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("nested descendant survived forceful ancestor kill: %v", err)
	}
}

// TestJoinedRunDoesNotClaimOwnedGroupCleanup proves a Run that joined an
// ancestor's group, and so stops and probes only its own pid, never reports
// owned-group cleanup while its own descendant is still alive: the cleanup
// scope is "ancestor-process-group" and PARTIAL, not "owned-process-group".
func TestJoinedRunDoesNotClaimOwnedGroupCleanup(t *testing.T) {
	fixture := t.TempDir()
	pidFile := filepath.Join(fixture, "pids")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	observation := Run(context.Background(), Spec{Argv: []string{executable, "-test.run=^TestProcessHelper$", "--", "descendant-hold"}, Dir: fixture, Env: []string{"PROCESS_HELPER=1", "PROCESS_PID_FILE=" + pidFile, "PROCESS_SIDE_EFFECT=" + filepath.Join(fixture, "late")}, Timeout: 500 * time.Millisecond, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1024, JoinAncestorProcessGroup: true})
	_, descendant := readProcessPIDs(t, pidFile)
	t.Cleanup(func() {
		if p, findErr := os.FindProcess(descendant); findErr == nil {
			_ = p.Kill()
		}
	})
	if !observation.TimedOut || !processExists(descendant) {
		t.Fatalf("fixture did not leave a live descendant: timedOut=%v", observation.TimedOut)
	}
	if observation.OwnedProcessGroupCleanup {
		t.Fatal("joined run claimed owned process group cleanup with a live descendant")
	}
	if observation.DescendantCleanupStatus != descendantCleanupAncestorProcessGroup || observation.DescendantCleanupQualification != processVerdictPartial {
		t.Fatalf("cleanup scope = %q/%q", observation.DescendantCleanupStatus, observation.DescendantCleanupQualification)
	}
}

// TestAfterStartDoesNotDeadlockBehindUnstartedDrain proves the fix for the
// "child's pipes are not drained until after identity capture and the
// AfterStart hook" defect: a child that fills a pipe before signalling
// readiness must still be observed promptly by AfterStart, because draining
// now starts as soon as the pipes exist rather than after the hook runs.
func TestAfterStartDoesNotDeadlockBehindUnstartedDrain(t *testing.T) {
	fixture := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	signalFile := filepath.Join(fixture, "signal")
	const payloadSize = 1 << 20 // well past any platform's pipe buffer
	spec := Spec{
		Argv:            []string{executable, "-test.run=^TestProcessHelper$", "--", "fill-pipe-then-signal"},
		Dir:             fixture,
		Env:             []string{"PROCESS_HELPER=1", "PROCESS_SIGNAL=" + signalFile},
		Timeout:         5 * time.Second,
		ShutdownTimeout: 4 * time.Second,
		InputLimit:      1,
		OutputLimit:     4 << 20,
		AfterStart: func(ctx context.Context, pid int) error {
			for {
				if _, err := os.Stat(signalFile); err == nil {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Millisecond):
				}
			}
		},
	}
	observation := Run(context.Background(), spec)
	if observation.Err != nil {
		t.Fatalf("child deadlocked behind an unstarted drain: %+v", observation)
	}
	if len(observation.Stdout) != payloadSize {
		t.Fatalf("stdout truncated: got %d bytes, want %d", len(observation.Stdout), payloadSize)
	}
}
