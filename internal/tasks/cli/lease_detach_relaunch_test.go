//go:build darwin || linux

package cli_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0095_RelaunchWaitsOutTheFinishingChild: after a detached child
// keeps a LOCK_TIMEOUT, a repeat --detach waits for the running lock the
// finishing child still holds, and then relaunches even while that child's
// process is still visible under its recorded identity (ATR-V0-095,
// V1-1671). Before ATR-V0-095 the first case returned the kept LOCK_TIMEOUT
// and the second found the exiting child live, started nothing and then
// reported SUPERVISOR_LOST.
func TestATRV0095_RelaunchWaitsOutTheFinishingChild(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	// Every recorded child stays visible with its recorded identity, as the
	// child that kept the result is between its lock release and its reaping.
	t.Cleanup(cli.SetLeaseDetachIdentity(func(int) (string, error) { return "test:exiting", nil }))
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-exiting", "--detach", "--wait", "30"}
	first := cliRun(t, root, claim...)
	if first.code == 0 || first.code == 75 || !slices.Contains(first.res.Codes, wire.CodeLockTimeout) {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	// The finishing child still holds the running lock for a moment.
	dir := cli.LeaseDetachDir(repo, "claim-exiting")
	running := acquireRunning(t, dir)
	// The unlock goroutine uses running; only the kernel lock orders it
	// before acquireRunning's Close, which the race detector cannot see.
	// This cleanup runs before that Close and waits for the unlock.
	unlocked := make(chan struct{})
	t.Cleanup(func() { <-unlocked })
	time.AfterFunc(300*time.Millisecond, func() {
		defer close(unlocked)
		_ = syscall.Flock(int(running.Fd()), syscall.LOCK_UN)
	})
	again := cliRun(t, root, claim...)
	if again.code != 0 || again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("repeated --detach after the ceiling: code %d %s", again.code, again.stdout)
	}
	if prev, err := os.ReadFile(filepath.Join(dir, "result.1.json")); err != nil || !bytes.Equal(prev, first.stdout) {
		t.Fatalf("result.1.json: %v %s, want %s", err, prev, first.stdout)
	}
}

// TestATRV0095_RunningWaitIsBoundedByWait: while the finishing child still
// holds the running lock, a repeat --detach waits for it only within the
// caller's --wait: --wait 0 and no --wait return the kept LOCK_TIMEOUT at
// once, and --wait 1 within its bound, with nothing moved or started
// (ATR-V0-095, ATR-V0-024).
func TestATRV0095_RunningWaitIsBoundedByWait(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-bounded", "--detach"}
	first := cliRun(t, root, append(append([]string{}, claim...), "--wait", "30")...)
	if first.code == 0 || first.code == 75 || !slices.Contains(first.res.Codes, wire.CodeLockTimeout) {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	dir := cli.LeaseDetachDir(repo, "claim-bounded")
	running := acquireRunning(t, dir)
	for _, c := range []struct {
		wait []string
		want time.Duration
	}{
		{[]string{"--wait", "0"}, 0},
		{nil, 0},
		{[]string{"--wait", "1"}, time.Second},
	} {
		started := time.Now()
		x := cliRun(t, root, append(append([]string{}, claim...), c.wait...)...)
		if elapsed := time.Since(started); elapsed > c.want+2*time.Second {
			t.Fatalf("%v returned after %v, bound %v", c.wait, elapsed, c.want)
		}
		if !bytes.Equal(x.stdout, first.stdout) {
			t.Fatalf("%v while the running lock is held: code %d %s, want the kept %s", c.wait, x.code, x.stdout, first.stdout)
		}
		if _, err := os.Lstat(filepath.Join(dir, "result.1.json")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%v moved the kept result: %v", c.wait, err)
		}
	}
	// Once the lock frees, the same --detach relaunches.
	if err := syscall.Flock(int(running.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if again := cliRun(t, root, append(append([]string{}, claim...), "--wait", "30")...); again.code != 0 || again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("repeated --detach after the release: code %d %s", again.code, again.stdout)
	}
}

// acquireRunning takes the record's running lock for the test once the
// child that kept the result has released it: the caller's wait returns as
// soon as result.json is readable, before the child's deferred unlock.
func acquireRunning(t *testing.T, dir string) *os.File {
	t.Helper()
	running, err := os.OpenFile(filepath.Join(dir, "running"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = running.Close() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := syscall.Flock(int(running.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return running
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			t.Fatalf("running lock: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestATRV0095_StaleStartingChildAfterRelaunchKeepsOneResult rules out the
// suspected V1-1716 race: a child started by a launcher that died before
// recording it (ATR-V0-021) takes the running lock after a repeat --detach
// moved a kept LOCK_TIMEOUT and started a new child. The new child cannot
// take running and exits 1 without a result, but the record stays PENDING
// while the stale child holds running, and attach returns the stale child's
// single kept result, never SUPERVISOR_LOST, with one claim in the store.
func TestATRV0095_StaleStartingChildAfterRelaunchKeepsOneResult(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-stale"}
	first := cliRun(t, root, append(append([]string{}, claim...), "--detach", "--wait", "30")...)
	if first.code == 0 || first.code == 75 || !slices.Contains(first.res.Codes, wire.CodeLockTimeout) {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	dir := cli.LeaseDetachDir(repo, "claim-stale")
	// Wait until the child that kept the LOCK_TIMEOUT has released running.
	if err := syscall.Flock(int(acquireRunning(t, dir).Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	// The relaunched child is slow to start, so the stale child wins running.
	t.Setenv("CORVINT_TEST_LEASE_DETACH_START_MS", "1500")
	relaunch := cliRun(t, root, append(append([]string{}, claim...), "--detach", "--wait", "0")...)
	if relaunch.code != 75 || field(relaunch.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("relaunch: code %d %s", relaunch.code, relaunch.stdout)
	}
	if _, err := os.Lstat(filepath.Join(dir, "result.1.json")); err != nil {
		t.Fatalf("the kept LOCK_TIMEOUT was not moved: %v", err)
	}
	newChild := detachedChildPid(t, dir)
	// The stale starting child: the unrecorded internal child of the same
	// request, here in process. It takes running at once and then waits on
	// the store's preparation lock, which the test still holds.
	staleDone := make(chan int, 1)
	go func() {
		var out, errb bytes.Buffer
		staleDone <- cli.Run(cli.Env{Cwd: root, Args: append(append([]string{}, claim...), "--detached-child"), Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
	}()
	for deadline := time.Now().Add(10 * time.Second); !cli.LeaseDetachRunningHeld(dir); {
		if !time.Now().Before(deadline) {
			t.Fatal("the stale child never took running")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The recorded new child finds running held and exits without a result.
	for deadline := time.Now().Add(15 * time.Second); syscall.Kill(newChild, 0) == nil; {
		if !time.Now().Before(deadline) {
			t.Fatal("the relaunched child did not exit")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Lstat(filepath.Join(dir, "result.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the relaunched child kept a result: %v", err)
	}
	pending := cliRun(t, root, "claim", "--attach", "--request-id", "claim-stale")
	if pending.code != 75 || field(pending.res.Items[0], "state").Str != "PENDING" || hasCode(pending.res, wire.CodeSupervisorLost) {
		t.Fatalf("attach while the stale child runs: code %d %s", pending.code, pending.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if code := <-staleDone; code != 0 {
		t.Fatalf("stale child exit %d", code)
	}
	done := cliRun(t, root, "claim", "--attach", "--request-id", "claim-stale", "--wait", "30")
	attempt := field(done.res.Items[0], "attemptId").Str
	if done.code != 0 || done.res.Outcome != wire.OutcomeOK || attempt == "" {
		t.Fatalf("attach after the stale child: code %d %s", done.code, done.stdout)
	}
	// Exactly one mutation: a repeat --detach answers from the kept result.
	if again := cliRun(t, root, append(append([]string{}, claim...), "--detach", "--wait", "30")...); !bytes.Equal(again.stdout, done.stdout) {
		t.Fatalf("repeat --detach: %s, want %s", again.stdout, done.stdout)
	}
}
