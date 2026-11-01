//go:build unix

package main

import (
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

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/worksource"
)

// WQO-V0-004/034: standalone script interruption owns its compiler group and
// removes temporary artifacts. This fixture does not create escaped children.
func TestWorkSelfAdapterStandaloneInterruption(t *testing.T) {
	caller := materializationFixture(t)
	script, err := filepath.Abs(filepath.Join("..", "..", "script", "corvint-work-queue"))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	fake := t.TempDir()
	pidfile := filepath.Join(fake, "descendant.pid")
	storagefile := filepath.Join(fake, "owned-storage")
	body := fmt.Sprintf("#!/bin/bash\nchild=\ntrap 'kill -TERM \"$child\" 2>/dev/null || :; wait \"$child\" 2>/dev/null || :; exit 143' TERM INT\nsleep 60 &\nchild=$!\nprintf '%%s\\n' \"$child\" > %q\nwait \"$child\"\n", pidfile)
	body = strings.Replace(body, "#!/bin/bash\n", "#!/bin/bash\n"+`printf '%s\n' "$TMPDIR" > `+strconv.Quote(storagefile)+"\n", 1)
	if err := execfile.WriteFile(filepath.Join(fake, "go"), []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, script, "snapshot")
	command.Dir = caller
	command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + t.TempDir(), "TMPDIR=" + scratch}
	workContain(command)
	command.WaitDelay = 10 * time.Second
	command.Cancel = func() error { workKillGroup(command); return nil }
	descendant := 0
	// Registered before launch; owns cleanup even if any assertion fails.
	t.Cleanup(func() {
		if descendant > 0 {
			_ = syscall.Kill(descendant, syscall.SIGKILL)
		}
		workKillGroup(command)
	})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(pidfile)
		if err == nil {
			descendant, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			if descendant > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if descendant == 0 {
		cancel()
		<-done
		t.Fatal("compiler descendant not registered")
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("interrupted script reported success")
		}
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("script interruption unbounded")
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && syscall.Kill(descendant, 0) == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(descendant, 0); err != syscall.ESRCH {
		t.Fatalf("compiler descendant remains: %v", err)
	}
	ownedRaw, err := os.ReadFile(storagefile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(strings.TrimSpace(string(ownedRaw)))); !os.IsNotExist(err) {
		t.Fatal("fixed standalone scratch remains after interruption")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("standalone scratch remains: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(caller, ".git", "corvint")); !os.IsNotExist(err) {
		t.Fatal("script wrote caller Git cache")
	}
}

// WQO-V0-004/034: SIGKILL cannot run producer defers. Every actual producer
// acquisition directory must therefore belong to the observer-owned run root.
func TestWorkActualProducerInterruptedScratchCleanup(t *testing.T) {
	t.Parallel()
	caller := workProductionFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	run, err := newWorkMaterialization(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	// Parallel cold producer build: a hang detector, not a budget (decision 0082).
	buildContext, stopBuild := context.WithTimeout(context.Background(), 30*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildContext, filepath.Join(run.target, "script/corvint-work-queue"), "snapshot")
	build.Dir, build.Env = run.target, run.environment
	build.WaitDelay = 10 * time.Second
	workContain(build)
	build.Cancel = func() error { workKillGroup(build); return nil }
	t.Cleanup(func() { workKillGroup(build) })
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("actual script build: %v: %s", err, raw)
	}
	workKillGroup(build)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // hang detector, not a budget (decision 0082)
	defer cancel()
	// V1-0849 (V1-0638): force the interleaving CI lost. A producer observed with
	// live scratch can remove it and exit before SIGKILL lands (in CI, SIGSTOP had
	// not yet taken effect). The attempt must then witness nothing, not report a
	// disappeared witness.
	letProducerWin := func(done <-chan struct{}) { <-done }
	forced := false
	for attempt := 0; attempt < workInterruptAttempts && !forced; attempt++ {
		var witnesses []string
		if forced, witnesses = workInterruptActualProducer(t, ctx, run, letProducerWin); len(witnesses) != 0 {
			t.Fatalf("producer that removed its scratch left witnesses: %v", witnesses)
		}
	}
	if !forced {
		t.Fatal("actual producer did not acquire scratch under observer ownership")
	}
	// The interruption outcome is decided only after the producer's reap: a
	// directory present then is one the SIGKILLed producer never removed. Attempts
	// that observed nothing or lost the race are counted, not timed.
	var witnesses []string
	observedAttempts := 0
	for attempt := 0; attempt < workInterruptAttempts && len(witnesses) == 0; attempt++ {
		var observed bool
		observed, witnesses = workInterruptActualProducer(t, ctx, run, nil)
		if observed {
			observedAttempts++
		}
		if len(witnesses) == 0 {
			t.Logf("attempt %d interrupted no acquisition (observed %v); retrying", attempt+1, observed)
		}
	}
	if observedAttempts == 0 {
		t.Fatal("actual producer did not acquire scratch under observer ownership")
	}
	if len(witnesses) == 0 {
		t.Fatalf("interruption witness disappeared before observer cleanup in all %d observed attempts", observedAttempts)
	}
	runRoot := run.root
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	for _, witness := range witnesses {
		if _, err := os.Stat(witness); !os.IsNotExist(err) {
			t.Fatalf("killed producer acquisition scratch remains: %s", witness)
		}
	}
	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatal("observer run root remains after interrupted producer")
	}
}

// workInterruptAttempts bounds producer runs that observe no acquisition or
// lose the kill race; it is an attempt count, not a timing budget.
const workInterruptAttempts = 8

// workInterruptActualProducer runs the built producer once and SIGKILLs its
// group while an acquisition directory exists under run.tmp. It reports whether
// an acquisition was observed, and the acquisition directories that remain after
// the producer is reaped. beforeKill, when set, runs between the observation and
// SIGKILL.
func workInterruptActualProducer(t *testing.T, ctx context.Context, run *workMaterialization, beforeKill func(<-chan struct{})) (bool, []string) {
	t.Helper()
	producer := exec.CommandContext(ctx, filepath.Join(run.tmp, "corvint-work-queue-owner/build/corvint-work-queue"), "snapshot")
	producer.Dir, producer.Env = run.target, run.environment
	producer.WaitDelay = 10 * time.Second
	workContain(producer)
	producer.Cancel = func() error { workKillGroup(producer); return nil }
	done := make(chan struct{})
	// Guardian cleanup is registered before the process can acquire scratch; a
	// reaped producer's process-group ID is no longer this test's to signal.
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			workKillGroup(producer)
		}
	})
	if err := producer.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = producer.Wait(); close(done) }()
	pattern := filepath.Join(run.tmp, "corvint-work-source-*")
	observed := false
	// The poll ends on producer exit or the hang detector, not a fixed budget; the scan
	// after an observed exit still runs once. No SIGSTOP: a stop can catch a forked Git
	// child after it leaves the producer group and before exec, and SIGCONT to the
	// group then cannot resume it, so the producer never exits.
	exited := false
	for !observed && !exited && ctx.Err() == nil {
		select {
		case <-done:
			exited = true
		default:
		}
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) > 0 {
			observed = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if observed && beforeKill != nil {
		beforeKill(done)
	}
	workKillGroup(producer)
	select {
	case <-done:
	case <-time.After(time.Minute): // hang detector for a SIGKILLed group reap (decision 0082)
		t.Fatal("producer group did not terminate")
	}
	witnesses, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return observed, witnesses
}

// TestWorkGitGroupProducerHelper is the producer TestWorkCloseRetiresKilledProducerGit
// kills: it acquires source in the run's scratch and blocks in a fixture Git read.
func TestWorkGitGroupProducerHelper(t *testing.T) {
	target := os.Getenv("CORVINT_WORK_TEST_GIT_PRODUCER_TARGET")
	if target == "" {
		t.Skip("producer helper")
	}
	source, err := worksource.AcquireWithScratch(context.Background(), target, os.Getenv("CORVINT_WORK_TEST_GIT_PRODUCER_SCRATCH"))
	if err != nil {
		t.Fatal(err)
	}
	source.GitPath = os.Getenv("CORVINT_WORK_TEST_GIT_PRODUCER_FIXTURE")
	_, _ = source.Git(context.Background(), 100, "cat-file")
}

// WQO-V0-052 (V1-2291, decision 0552): a producer SIGKILLed during a Git read
// leaves that Git group running in its acquisition scratch; run.Close retires
// it, so no Git child outlives Close, and then removes the run root.
func TestWorkCloseRetiresKilledProducerGit(t *testing.T) {
	caller := materializationFixture(t)
	source, err := worksource.Acquire(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	run, err := newWorkMaterialization(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	fixtures := t.TempDir()
	pidPath := filepath.Join(fixtures, "pids")
	fixture := filepath.Join(fixtures, "git-fixture")
	if err := execfile.WriteFile(fixture, []byte("#!/bin/sh\nsleep 600 &\nprintf '%s %s\\n' \"$$\" \"$!\" > "+pidPath+".tmp\nmv "+pidPath+".tmp "+pidPath+"\nwait\n"), 0755); err != nil {
		t.Fatal(err)
	}
	producer := exec.Command(os.Args[0], "-test.run=^TestWorkGitGroupProducerHelper$", "-test.count=1")
	producer.Env = append(os.Environ(), "CORVINT_WORK_TEST_GIT_PRODUCER_TARGET="+run.target, "CORVINT_WORK_TEST_GIT_PRODUCER_SCRATCH="+run.tmp, "CORVINT_WORK_TEST_GIT_PRODUCER_FIXTURE="+fixture)
	workContain(producer)
	if err := producer.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = producer.Wait(); close(done) }()
	var pids []int
	t.Cleanup(func() {
		workKillGroup(producer)
		<-done
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	// Readiness ends on the marker or producer exit; the deadline is a hang detector.
	deadline := time.Now().Add(5 * time.Minute)
	for pids == nil {
		if raw, err := os.ReadFile(pidPath); err == nil {
			for _, field := range strings.Fields(string(raw)) {
				pid, err := strconv.Atoi(field)
				if err != nil || pid <= 1 {
					t.Fatalf("fixture pids %q", raw)
				}
				pids = append(pids, pid)
			}
			break
		}
		select {
		case <-done:
			t.Fatal("producer exited before its Git fixture was ready")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Git fixture readiness exceeded the hang bound")
		}
		time.Sleep(5 * time.Millisecond)
	}
	workKillGroup(producer)
	<-done
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("Git process %d did not survive the producer kill: %v", pid, err)
		}
	}
	runRoot := run.root
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	// Close returned only after every recorded holder exited; the orphaned
	// zombies are reaped by the system, bounded by a hang detector.
	deadline = time.Now().Add(time.Minute)
	for _, pid := range pids {
		for syscall.Kill(pid, 0) == nil {
			if time.Now().After(deadline) {
				t.Fatalf("Git process %d outlived run.Close", pid)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatal("run root remains after retirement")
	}
}
