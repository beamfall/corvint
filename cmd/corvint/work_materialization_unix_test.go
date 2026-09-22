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
	if err := os.WriteFile(filepath.Join(fake, "go"), []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, script, "snapshot")
	command.Dir = caller
	command.Env = []string{"PATH=" + fake + ":/usr/bin:/bin:/usr/local/bin", "HOME=" + t.TempDir(), "TMPDIR=" + scratch}
	workContain(command)
	command.WaitDelay = time.Second
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
	build.WaitDelay = time.Second
	workContain(build)
	build.Cancel = func() error { workKillGroup(build); return nil }
	t.Cleanup(func() { workKillGroup(build) })
	if raw, err := build.CombinedOutput(); err != nil {
		t.Fatalf("actual script build: %v: %s", err, raw)
	}
	workKillGroup(build)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // hang detector, not a budget (decision 0082)
	defer cancel()
	producer := exec.CommandContext(ctx, filepath.Join(run.tmp, "corvint-work-queue-owner/build/corvint-work-queue"), "snapshot")
	producer.Dir, producer.Env = run.target, run.environment
	producer.WaitDelay = time.Second
	workContain(producer)
	producer.Cancel = func() error { workKillGroup(producer); return nil }
	// Guardian cleanup is registered before the process can acquire scratch.
	t.Cleanup(func() { workKillGroup(producer) })
	if err := producer.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = producer.Wait(); close(done) }()
	acquired := ""
	// The poll ends on producer exit or the hang detector, not a fixed budget; the scan
	// after an observed exit still runs once.
	exited := false
	for !exited && ctx.Err() == nil {
		select {
		case <-done:
			exited = true
		default:
		}
		paths, err := filepath.Glob(filepath.Join(run.tmp, "corvint-work-source-*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) > 0 {
			// Stop the owned group before asserting the directory still exists, closing
			// the race with a normally completed short root-resolution acquisition.
			_ = syscall.Kill(-producer.Process.Pid, syscall.SIGSTOP)
			if _, err := os.Stat(paths[0]); err == nil {
				acquired = paths[0]
				break
			}
			_ = syscall.Kill(-producer.Process.Pid, syscall.SIGCONT)
		}
		time.Sleep(time.Millisecond)
	}
	workKillGroup(producer)
	select {
	case <-done:
	case <-time.After(time.Minute): // hang detector for a SIGKILLed group reap (decision 0082)
		t.Fatal("producer group did not terminate")
	}
	if acquired == "" {
		t.Fatal("actual producer did not acquire scratch under observer ownership")
	}
	if _, err := os.Stat(acquired); err != nil {
		t.Fatalf("interruption witness disappeared before observer cleanup: %v", err)
	}
	runRoot := run.root
	run.Close()
	if _, err := os.Stat(acquired); !os.IsNotExist(err) {
		t.Fatalf("killed producer acquisition scratch remains: %s", acquired)
	}
	if _, err := os.Stat(runRoot); !os.IsNotExist(err) {
		t.Fatal("observer run root remains after interrupted producer")
	}
}
