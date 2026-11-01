//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gitstatus"
)

// Real exec of the test binary preserves snapshot engine identity. Every child
// is bounded and reaped by startSnapshotRefresh; tests also kill its group on
// cleanup, including if an assertion fails before normal completion.
func TestSnapshotRefreshWorkerHelper(t *testing.T) {
	root := os.Getenv("V1128_REFRESH_ROOT")
	if root == "" {
		return
	}
	if file, err := os.Create(filepath.Join(root, "refresh-test-stderr")); err == nil {
		os.Stderr = file
	}
	snapshotRefreshSetpriority = func(which, who, priority int) error {
		if os.Getenv("V1128_PRIORITY_DENIED") == "1" {
			return syscall.EPERM
		}
		if priority < 10 {
			return fmt.Errorf("priority request %d is not lowered", priority)
		}
		err := syscall.Setpriority(which, who, priority)
		if os.Getenv("V1128_PRIORITY_STRICT") != "1" && (errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES)) {
			// This is a test-only scheduling substitution, never production fallback.
			fmt.Fprintln(os.Stderr, "NOT_OBSERVED: sandbox denied lowering priority; testing remaining lifecycle with requested priority >=10")
			return nil
		}
		return err
	}
	bound := snapshotRefreshWallBound
	if value := os.Getenv("V1128_REFRESH_BOUND"); value != "" {
		var err error
		bound, err = time.ParseDuration(value)
		if err != nil {
			os.Exit(2)
		}
	}
	budget := snapshotRefreshCPUBudget
	if value := os.Getenv("V1128_REFRESH_CPU"); value != "" {
		var err error
		budget, err = time.ParseDuration(value)
		if err != nil {
			os.Exit(2)
		}
	}
	os.Exit(runSnapshotRefreshWorker([]string{"--root", root, "index", "--if-stale"}, bound, budget))
}

func refreshTestContext(t *testing.T, root string, extra ...string) (context.Context, *[]*exec.Cmd) {
	t.Helper()
	var mu sync.Mutex
	commands := new([]*exec.Cmd)
	factory := snapshotRefreshCommand(func(root string) (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSnapshotRefreshWorkerHelper$")
		cmd.Env = append(os.Environ(), append([]string{"V1128_REFRESH_ROOT=" + root}, extra...)...)
		mu.Lock()
		*commands = append(*commands, cmd)
		mu.Unlock()
		return cmd, nil
	})
	ctx := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root, "CORVINT_INDEX_AUTO_REFRESH": "1"})
	ctx = context.WithValue(ctx, dogfoodEventDeadlineKey{}, func(string, string) time.Duration { return time.Minute })
	ctx = context.WithValue(ctx, snapshotRefreshCommandKey{}, factory)
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, cmd := range *commands {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				waitRefresh(t, func() bool { return errors.Is(syscall.Kill(cmd.Process.Pid, 0), syscall.ESRCH) })
			}
		}
	})
	return ctx, commands
}

func waitRefresh(t *testing.T, predicate func() bool) {
	t.Helper()
	until := time.Now().Add(20 * time.Second)
	for time.Now().Before(until) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background refresh did not reach the expected state")
}

func waitRefreshExit(t *testing.T, command *exec.Cmd) {
	t.Helper()
	waitRefresh(t, func() bool { return errors.Is(syscall.Kill(command.Process.Pid, 0), syscall.ESRCH) })
}

func refreshEvent(ctx context.Context, root, host string) map[string]any {
	if host == "codex" {
		return runCodexAdapter(ctx, map[string]any{"hook_event_name": "UserPromptSubmit", "cwd": root, "session_id": "refresh-test", "prompt": "inspect requirement"})
	}
	return runClaudeAdapter(ctx, "user-prompt", map[string]any{"session_id": "refresh-test", "prompt": "inspect requirement"})
}

// IDX-SNAP-V0-031/036, AHI-053: this fixture writes an index, commits content,
// receives a stale hook, and proves the following event consumes the child's
// snapshot instead of rebuilding in memory.
func TestSnapshotRefreshCommitThenEvent(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		t.Run(host, func(t *testing.T) {
			root := staleSnapshotRepository(t)
			if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
				t.Fatal(err)
			}
			ctx, commands := refreshTestContext(t, root)
			first := refreshEvent(ctx, root, host)
			if adapterDegradationReason(first) != claudeSnapshotStaleReason || !strings.Contains(fmt.Sprint(first), "Background index refresh started") || !strings.Contains(fmt.Sprint(first), "building one in memory does not fit the hook deadline") {
				t.Fatalf("first hook: %v", first)
			}
			if len(*commands) != 1 {
				t.Fatalf("spawn count %d", len(*commands))
			}
			waitRefreshExit(t, (*commands)[0])
			if output, _ := os.ReadFile(filepath.Join(root, "refresh-test-stderr")); len(output) != 0 {
				t.Logf("worker stderr: %s", output)
			}
			if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || !fresh {
				t.Fatalf("refresh not published: fresh=%t err=%v", fresh, err)
			}
			built := new(atomic.Bool)
			ctx = context.WithValue(ctx, dogfoodEventBuildKey{}, func(context.Context, string, string) (*contextindex.Index, error) {
				built.Store(true)
				return nil, errors.New("unexpected build")
			})
			second := refreshEvent(ctx, root, host)
			if reason := adapterDegradationReason(second); reason != "" || built.Load() {
				t.Fatalf("next hook not fresh: %v built=%t", second, built.Load())
			}
			if len(*commands) != 1 {
				t.Fatal("fresh event spawned a writer")
			}
		})
	}
}

// slowRefreshGit blocks the opening rev-parse after proving that the fixture can
// index without injection. Its sleep stays in the owned worker group. The ready
// file proves the fault path was reached and carries the descendant PID.
func slowRefreshGit(t *testing.T, root string) (string, string) {
	t.Helper()
	realGit := gitstatus.Executable()
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	script := fmt.Sprintf("#!/bin/sh\nfor arg do\n if [ \"$arg\" = rev-parse ]; then\n /bin/sleep 120 &\n child=$!\n echo \"$child\" > '%s'\n wait \"$child\"\n exit 1\n fi\ndone\nexec '%s' \"$@\"\n", ready, realGit)
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH"), ready
}

func TestSnapshotRefreshSingleFlight(t *testing.T) {
	root := staleSnapshotRepository(t)
	path, ready := slowRefreshGit(t, root)
	ctx, commands := refreshTestContext(t, root, path)
	var wg sync.WaitGroup
	results := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- startSnapshotRefresh(ctx, root) }()
	}
	wg.Wait()
	close(results)
	started := 0
	for result := range results {
		if result == "started" {
			started++
		} else if result != "already running" {
			t.Fatalf("unexpected %s", result)
		}
	}
	if started != 1 || len(*commands) != 1 {
		t.Fatalf("started=%d children=%d", started, len(*commands))
	}
	waitRefresh(t, func() bool { _, err := os.Stat(ready); return err == nil })
	// Both the PID and start time are persisted, not used as lock authority.
	data, err := os.ReadFile(filepath.Join(contextindex.SnapshotDirectory(root), "refresh.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		PID     int       `json:"pid"`
		Started time.Time `json:"started"`
	}
	if json.Unmarshal(data, &record) != nil || record.PID != (*commands)[0].Process.Pid || record.Started.IsZero() {
		t.Fatalf("metadata %s", data)
	}
}

func TestSnapshotRefreshStaleLock(t *testing.T) {
	for _, metadata := range []string{`{"pid":99999999,"started":"2026-10-10T00:00:00Z"}`, `{"pid":1,"started":"2000-01-01T00:00:00Z"}`} {
		root := staleSnapshotRepository(t)
		if err := os.WriteFile(filepath.Join(contextindex.SnapshotDirectory(root), "refresh.lock"), []byte(metadata), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, commands := refreshTestContext(t, root)
		if status := startSnapshotRefresh(ctx, root); status != "started" {
			t.Fatal(status)
		}
		waitRefreshExit(t, (*commands)[0])
		if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || !fresh {
			t.Fatalf("stale lock prevented publication %t %v", fresh, err)
		}
	}
}

func TestSnapshotRefreshDisabledAndFailures(t *testing.T) {
	root := staleSnapshotRepository(t)
	ctx := adapterEnvContext(context.Background(), map[string]string{"CORVINT_INDEX_AUTO_REFRESH": "0"})
	before := readSnapshotDirectoryState(t, root)
	if notice := snapshotRemediation(ctx, root); !strings.Contains(notice, "Refresh the snapshot once, outside the hook") || strings.Contains(notice, "started") {
		t.Fatal(notice)
	}
	if len(readSnapshotDirectoryState(t, root)) != len(before) {
		t.Fatal("disabled refresh wrote state")
	}
	ctx, commands := refreshTestContext(t, root)
	ctx = context.WithValue(ctx, snapshotRefreshCommandKey{}, snapshotRefreshCommand(func(string) (*exec.Cmd, error) {
		return exec.Command(filepath.Join(t.TempDir(), "missing-corvint")), nil
	}))
	if notice := snapshotRemediation(ctx, root); !strings.Contains(notice, "failed to start") || !strings.Contains(notice, `"index","--if-stale"`) {
		t.Fatal(notice)
	}
	if len(*commands) != 0 {
		t.Fatal("unexpected injected child")
	}
	lock, err := contextindex.AcquireSnapshotRefreshLock(root)
	if err != nil || lock == nil {
		t.Fatalf("failed spawn held lock %v", err)
	}
	lock.Close()
	for _, kind := range []string{"symlink", "dangling-symlink", "fifo", "directory", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root := staleSnapshotRepository(t)
			lockPath := filepath.Join(contextindex.SnapshotDirectory(root), "refresh.lock")
			target := filepath.Join(t.TempDir(), "untouched")
			if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				err = os.Symlink(target, lockPath)
			case "dangling-symlink":
				target = filepath.Join(contextindex.SnapshotDirectory(root), "must-not-create")
				err = os.Symlink(target, lockPath)
			case "fifo":
				err = syscall.Mkfifo(lockPath, 0o600)
			case "directory":
				err = os.Mkdir(lockPath, 0o700)
			case "hardlink":
				err = os.Link(target, lockPath)
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx, _ := refreshTestContext(t, root)
			if status := startSnapshotRefresh(ctx, root); status != "failed to start" {
				t.Fatal(status)
			}
			if kind == "dangling-symlink" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatal("created dangling link target")
				}
				return
			}
			data, _ := os.ReadFile(target)
			if string(data) != "untouched" {
				t.Fatal("lock mutated another file")
			}
		})
	}
}

func TestSnapshotRefreshSlowChild(t *testing.T) {
	root := staleSnapshotRepository(t)
	if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
		t.Fatal(err)
	}
	path, ready := slowRefreshGit(t, root)
	ctx, commands := refreshTestContext(t, root, path)
	started := time.Now()
	output := refreshEvent(ctx, root, "claude")
	elapsed := time.Since(started)
	if adapterDegradationReason(output) != claudeSnapshotStaleReason || elapsed >= 1500*time.Millisecond {
		t.Fatalf("hook %s: %v", elapsed, output)
	}
	waitRefresh(t, func() bool { _, err := os.Stat(ready); return err == nil })
	if syscall.Kill((*commands)[0].Process.Pid, 0) != nil {
		t.Fatal("slow child did not outlive hook")
	}
}

func TestSnapshotRefreshInterrupted(t *testing.T) {
	for _, mode := range []string{"group-killed", "leader-killed", "expired", "terminated"} {
		t.Run(mode, func(t *testing.T) {
			root := staleSnapshotRepository(t)
			if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
				t.Fatal(err)
			}
			if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || fresh {
				t.Fatalf("fixture must miss before injection: %t %v", fresh, err)
			}
			path, ready := slowRefreshGit(t, root)
			extra := []string{path}
			if mode == "expired" {
				extra = append(extra, "V1128_REFRESH_BOUND=3s")
			}
			ctx, commands := refreshTestContext(t, root, extra...)
			if status := startSnapshotRefresh(ctx, root); status != "started" {
				t.Fatal(status)
			}
			waitRefresh(t, func() bool { _, err := os.Stat(ready); return err == nil })
			if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || fresh {
				t.Fatalf("fault must precede publication: %t %v", fresh, err)
			}
			data, _ := os.ReadFile(ready)
			var descendant int
			if _, err := fmt.Sscan(string(data), &descendant); err != nil || descendant <= 0 {
				t.Fatalf("no injected descendant: %s", data)
			}
			if mode == "group-killed" {
				_ = syscall.Kill(-(*commands)[0].Process.Pid, syscall.SIGKILL)
			}
			if mode == "leader-killed" {
				_ = syscall.Kill((*commands)[0].Process.Pid, syscall.SIGKILL)
			}
			if mode == "terminated" {
				_ = syscall.Kill((*commands)[0].Process.Pid, syscall.SIGTERM)
			}
			waitRefreshExit(t, (*commands)[0])
			if mode == "leader-killed" {
				// A killed process cannot execute its timer/defer. This residual is
				// explicit: the operator must signal the group, not just its leader.
				// Prove lock recovery before the test retires the surviving group.
				lock, err := contextindex.AcquireSnapshotRefreshLock(root)
				if err != nil || lock == nil {
					t.Fatalf("leader death held lock: %v", err)
				}
				lock.Close()
				if err := syscall.Kill(descendant, 0); err != nil {
					t.Fatalf("leader-only kill residual not exercised: %v", err)
				}
				_ = syscall.Kill(-(*commands)[0].Process.Pid, syscall.SIGKILL)
			}
			waitRefresh(t, func() bool { return errors.Is(syscall.Kill(descendant, 0), syscall.ESRCH) })
			if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || fresh {
				t.Fatalf("interrupted pre-publication writer published: %t %v", fresh, err)
			}
			// Keep another refresh from obscuring the stale outcome assertion.
			disabled := adapterEnvContext(context.Background(), map[string]string{"CLAUDE_PROJECT_DIR": root, "CORVINT_INDEX_AUTO_REFRESH": "0"})
			output := refreshEvent(disabled, root, "claude")
			if adapterDegradationReason(output) != claudeSnapshotStaleReason || !strings.Contains(fmt.Sprint(output), `"index","--if-stale"`) {
				t.Fatalf("lost fallback: %v", output)
			}
			lock, err := contextindex.AcquireSnapshotRefreshLock(root)
			if err != nil || lock == nil {
				t.Fatalf("killed lock not recovered: %v", err)
			}
			// Proposed IDX-SNAP-V0-515: the stopped writer stays visible to the next start.
			previous := previousSnapshotRefresh(lock)
			lock.Close()
			// Every mode here ends at a kill: the injected Git ignores cancellation, so even the
			// expired writer meets its hard bound before it can record an outcome.
			if !strings.Contains(previous, "did not finish successfully: it stopped before recording an outcome") {
				t.Fatalf("previous outcome %q", previous)
			}
			// A writer killed during encoding can leave a temporary. Readers must
			// ignore it; the existing writer sweeps it only after ten minutes.
			temporary := filepath.Join(contextindex.SnapshotDirectory(root), "snapshot-interrupted.tmp")
			if err := os.WriteFile(temporary, []byte("partial"), 0o600); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-11 * time.Minute)
			if err := os.Chtimes(temporary, old, old); err != nil {
				t.Fatal(err)
			}
			runIndexForTest(t, root, true)
			if _, err := os.Stat(temporary); !os.IsNotExist(err) {
				t.Fatalf("abandoned temporary not swept: %v", err)
			}
		})
	}
}

// IDX-SNAP-V0-033: inspect effective priority, not the requested syscall value.
// Linux checks every runtime thread while Git is blocked and a Git descendant.
func TestSnapshotRefreshEffectivePriority(t *testing.T) {
	root := staleSnapshotRepository(t)
	path, ready := slowRefreshGit(t, root)
	ctx, commands := refreshTestContext(t, root, path, "V1128_PRIORITY_STRICT=1", snapshotRefreshPriorityEnv+"=untrusted-parent-marker")
	if status := startSnapshotRefresh(ctx, root); status != "started" {
		t.Fatal(status)
	}
	worker := (*commands)[0].Process.Pid
	waitRefresh(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil || errors.Is(syscall.Kill(worker, 0), syscall.ESRCH)
	})
	data, err := os.ReadFile(ready)
	if err != nil {
		output, _ := os.ReadFile(filepath.Join(root, "refresh-test-stderr"))
		if strings.Contains(string(output), "index refresh priority: operation not permitted") || strings.Contains(string(output), "index refresh priority: permission denied") {
			t.Skipf("NOT_OBSERVED: sandbox denies real niceness: %s", output)
		}
		t.Fatalf("worker failed before Git witness: %s (%v)", output, err)
	}
	var descendant int
	if _, err := fmt.Sscan(string(data), &descendant); err != nil || descendant <= 0 {
		t.Fatalf("Git descendant missing: %s", data)
	}
	pids := []int{worker, descendant}
	if runtime.GOOS == "linux" {
		entries, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", worker))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) < 2 {
			t.Fatal("multiple runtime threads not exercised")
		}
		for _, entry := range entries {
			tid, err := strconv.Atoi(entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			pids = append(pids, tid)
		}
	}
	for _, pid := range pids {
		priority, err := syscall.Getpriority(syscall.PRIO_PROCESS, pid)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS == "linux" {
			priority = 20 - priority
		}
		if priority < 10 {
			t.Fatalf("pid/tid %d effective nice=%d, want >=10", pid, priority)
		}
	}
}

// IDX-SNAP-V0-033: denied priority must never reach Git or publish a snapshot.
func TestSnapshotRefreshPriorityDenied(t *testing.T) {
	root := staleSnapshotRepository(t)
	path, ready := slowRefreshGit(t, root)
	ctx, commands := refreshTestContext(t, root, path, "V1128_PRIORITY_DENIED=1")
	if status := startSnapshotRefresh(ctx, root); status != "started" {
		t.Fatal(status)
	}
	waitRefreshExit(t, (*commands)[0])
	output, _ := os.ReadFile(filepath.Join(root, "refresh-test-stderr"))
	if !strings.Contains(string(output), "index refresh priority: operation not permitted") {
		t.Fatalf("denial not exercised: %s", output)
	}
	if _, err := os.Stat(ready); !os.IsNotExist(err) {
		t.Fatalf("denied worker dispatched Git: %v", err)
	}
	if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || fresh {
		t.Fatalf("denied worker published: %t %v", fresh, err)
	}
	lock, err := contextindex.AcquireSnapshotRefreshLock(root)
	if err != nil || lock == nil {
		t.Fatalf("denied worker retained lock: %v", err)
	}
	// Proposed IDX-SNAP-V0-515: the refused priority setup is recorded as failed.
	previous := previousSnapshotRefresh(lock)
	lock.Close()
	if !strings.Contains(previous, "did not finish successfully: it failed.") {
		t.Fatalf("denied worker left no failed outcome: %q", previous)
	}
}

// Proposed IDX-SNAP-V0-515: a started refresh names how the previous writer stopped, from the
// lock metadata only; a completed writer's truncated record and malformed bytes name nothing.
func TestSnapshotRefreshPreviousOutcome(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"not json":  "",
		`{"pid":7}`: "",
		`{"pid":7,"started":"2026-10-30T19:36:04.5Z"}`:                                                  "started at 2026-10-30T19:36:04Z, did not finish successfully: it stopped before recording an outcome",
		`{"pid":7,"started":"2026-10-30T19:36:04Z","ended":"2026-10-30T19:37:00Z","outcome":"expired"}`: "it reached its work limit (55 CPU-seconds, or 50 seconds before its 10-minute hard limit",
		`{"pid":7,"started":"2026-10-30T19:36:04Z","outcome":"failed"}`:                                 "did not finish successfully: it failed.",
		`{"pid":7,"started":"2026-10-30T19:36:04Z","outcome":"<injected>"}`:                             "it stopped before recording an outcome",
	}
	for metadata, want := range cases {
		lock, err := os.CreateTemp(t.TempDir(), "refresh.lock")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lock.WriteString(metadata); err != nil {
			t.Fatal(err)
		}
		got := previousSnapshotRefresh(lock)
		lock.Close()
		if (want == "") != (got == "") || !strings.Contains(got, want) || strings.Contains(got, "injected") || strings.Contains(got, "pid") {
			t.Fatalf("%s: %q", metadata, got)
		}
	}
}

// Proposed IDX-SNAP-V0-515: the hook notice carries the previous writer's outcome after the
// started status, and the worker that then completes clears the record.
func TestSnapshotRefreshNoticeNamesPreviousOutcome(t *testing.T) {
	root := staleSnapshotRepository(t)
	if err := contextindex.RecordBuildCost(root, time.Hour); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(contextindex.SnapshotDirectory(root), "refresh.lock")
	if err := os.WriteFile(lockPath, []byte(`{"pid":1,"started":"2000-01-01T00:00:00Z","ended":"2000-01-01T00:00:55Z","outcome":"expired"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, commands := refreshTestContext(t, root)
	output := refreshEvent(ctx, root, "claude")
	text := fmt.Sprint(output)
	if adapterDegradationReason(output) != claudeSnapshotStaleReason || !strings.Contains(text, "Background index refresh started. The previous background refresh, started at 2000-01-01T00:00:00Z, did not finish successfully: it reached its work limit (55 CPU-seconds") {
		t.Fatalf("notice: %v", output)
	}
	waitRefreshExit(t, (*commands)[0])
	if data, err := os.ReadFile(lockPath); err != nil || len(data) != 0 {
		t.Fatalf("completed refresh left metadata %q %v", data, err)
	}
	if _, fresh, err := contextindex.ProbeSnapshot(context.Background(), root); err != nil || !fresh {
		t.Fatalf("refresh did not publish: %t %v", fresh, err)
	}
}

// Proposed IDX-SNAP-V0-515: the previous outcome is dropped, never the frame, status or argv,
// when it would push the hook output past adapterOutputLimit.
func TestSnapshotRefreshPreviousOutcomeYieldsToOutputLimit(t *testing.T) {
	root := staleSnapshotRepository(t)
	lockPath := filepath.Join(contextindex.SnapshotDirectory(root), "refresh.lock")
	if err := os.WriteFile(lockPath, []byte(`{"pid":1,"started":"2000-01-01T00:00:00Z","outcome":"expired"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, commands := refreshTestContext(t, root)
	refresh, _ := json.Marshal([]string{"corvint", "--root", root, "index", "--if-stale"})
	plain := "\nNo index snapshot matches the current tree, and building one in memory does not fit the hook deadline. Background index refresh started. Manual fallback, outside the hook:\n" + string(refresh)
	probe, _ := json.Marshal(map[string]any{"systemMessage": plain})
	frame := "Corvint FALLBACK degraded: " + claudeSnapshotStaleReason + "; coding continues" + strings.Repeat(" ", adapterOutputLimit-1-len(probe)-200)
	output := withSnapshotRemediation(ctx, root, "stop", claudeSnapshotStaleReason, map[string]any{"systemMessage": frame})
	waitRefreshExit(t, (*commands)[0])
	text, _ := output["systemMessage"].(string)
	if text != frame+plain || !withinAdapterOutputLimit(output) {
		t.Fatalf("over-limit notice kept the outcome or lost the argv: %d bytes", len(text))
	}
}

// IDX-SNAP-V0-912: the work bound is CPU time. A spent budget cancels the
// build with the budget cause (recorded as expired); an unspent one leaves the
// work running however long it waits, and process CPU time is what is read.
func TestSnapshotRefreshCPUBudgetCancelsWork_IDXSNAPV0912(t *testing.T) {
	spent, stop := snapshotRefreshCPUContext(context.Background(), 0, 5*time.Millisecond)
	defer stop()
	select {
	case <-spent.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("a spent CPU budget did not cancel the work")
	}
	if !errors.Is(context.Cause(spent), errSnapshotRefreshCPUBudget) || errors.Is(spent.Err(), context.DeadlineExceeded) {
		t.Fatalf("cause = %v, err = %v", context.Cause(spent), spent.Err())
	}
	waiting, stopWaiting := snapshotRefreshCPUContext(context.Background(), time.Hour, 5*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if waiting.Err() != nil {
		t.Fatalf("an idle wait spent the CPU budget: %v", context.Cause(waiting))
	}
	stopWaiting()
	before := snapshotRefreshCPUTime()
	for deadline := time.Now().Add(200 * time.Millisecond); time.Now().Before(deadline); {
	}
	if after := snapshotRefreshCPUTime(); after-before < 50*time.Millisecond {
		t.Fatalf("CPU time %s -> %s across a 200 ms busy loop", before, after)
	}
	if snapshotRefreshWallBound != 10*time.Minute || snapshotRefreshCPUBudget != 55*time.Second {
		t.Fatalf("bounds %s wall, %s CPU", snapshotRefreshWallBound, snapshotRefreshCPUBudget)
	}
}
