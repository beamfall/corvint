//go:build darwin || linux

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const lockHolderEnv = "ATR_LOCK_HOLDER_ROOT"

// TestATRV0050_WriterLockHolderHelper is the re-executed process that holds
// the store's writer lock and preparation gate for the contention tests until
// it is killed.
func TestATRV0050_WriterLockHolderHelper(t *testing.T) {
	root := os.Getenv(lockHolderEnv)
	if root == "" {
		t.Skip("helper process for the writer-contention tests")
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		os.Exit(3)
	}
	// Lease writes commit under the preparation gate (the writer-checkpoint
	// route) or the writer lock, so a contending writer holds both, in the
	// writers' own order.
	gate, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		os.Exit(4)
	}
	defer gate.Close()
	lock, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		os.Exit(4)
	}
	defer lock.Close()
	if err := os.WriteFile(os.Getenv("ATR_LOCK_HOLDER_READY"), []byte("held"), 0o600); err != nil {
		os.Exit(5)
	}
	time.Sleep(10 * time.Minute) // until the test kills it
}

// holdWriterLock starts a helper process that holds the store writer lock
// and returns once it is held; the returned function releases it by killing
// the helper.
func holdWriterLock(t *testing.T, root string) func() {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "held")
	c := exec.Command(os.Args[0], "-test.run=^TestATRV0050_WriterLockHolderHelper$")
	c.Env = append(os.Environ(), lockHolderEnv+"="+root, "ATR_LOCK_HOLDER_READY="+ready)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_ = c.Process.Kill()
			_ = c.Wait()
		}
	}
	t.Cleanup(release)
	waitFile(t, ready)
	return release
}

func noMalformed(t *testing.T, x run) {
	t.Helper()
	if hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("a contended store write reported MALFORMED: %s", x.stdout)
	}
}

// TestATRV0050_ContendedPrelaunchWriteIsLockTimeout: a pre-launch lease
// write whose bounded context ends while another process holds the writer
// lock refuses with the retryable LOCK_TIMEOUT, exit 1, and starts nothing.
func TestATRV0050_ContendedPrelaunchWriteIsLockTimeout(t *testing.T) {
	root, a := attemptStore(t)
	defer cli.SetRunWriteBudget(1500 * time.Millisecond)()
	holdWriterLock(t, root)
	marker := filepath.Join(t.TempDir(), "ran")
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "5"}, "/bin/sh", "-c", "touch "+marker)
	noMalformed(t, r)
	if r.code != 1 || !hasCode(r.res, wire.CodeLockTimeout) || !bytes.Contains(r.stdout, []byte(`"retryable":true`)) {
		t.Fatalf("want exit 1 retryable LOCK_TIMEOUT: code %d %s", r.code, r.stdout)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the command started although the pre-launch write failed")
	}
}

// TestATRV0051_ContendedTimedOutRunExits127WithOneItem: a started run that
// times out while another process holds the writer lock retries its outcome
// write within the budget, then exits 127 with LOCK_TIMEOUT and exactly one
// run item of class TIMEOUT; it never ends MALFORMED (ATR-V0-051, ATR-V0-052).
func TestATRV0051_ContendedTimedOutRunExits127WithOneItem(t *testing.T) {
	root, a := attemptStore(t)
	defer cli.SetRunWriteBudget(1500 * time.Millisecond)()
	started := filepath.Join(t.TempDir(), "started")
	done := make(chan run, 1)
	go func() {
		done <- runAttempt(t, root, a, gen(a), []string{"--timeout", "3"}, "/bin/sh", "-c", "touch "+started+"; exec sleep 30")
	}()
	waitFile(t, started)
	release := holdWriterLock(t, root)
	r := <-done
	release()
	noMalformed(t, r)
	if r.code != 127 || r.res.Outcome != wire.OutcomeError || !hasCode(r.res, wire.CodeLockTimeout) || !hasCode(r.res, wire.CodeLimitExceeded) {
		t.Fatalf("want 127 with LIMIT_EXCEEDED and LOCK_TIMEOUT: code %d %s", r.code, r.stdout)
	}
	if len(r.res.Items) != 1 {
		t.Fatalf("want exactly one run item: %s", r.stdout)
	}
	item := r.res.Items[0]
	if field(item, "class").Str != transaction.RunTimeout || field(item, "exitStatus").Str != "127" || field(item, "outcomeReceipt").Kind != wire.KindNull {
		t.Fatalf("item %+v", item)
	}
}

// TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees: a retryable outcome
// write is retried with the same request id within the budget, so a holder
// that releases in time leaves a recorded timeout that exits 124.
func TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees(t *testing.T) {
	root, a := attemptStore(t)
	defer cli.SetRunWriteBudget(20 * time.Second)()
	started := filepath.Join(t.TempDir(), "started")
	done := make(chan run, 1)
	go func() {
		done <- runAttempt(t, root, a, gen(a), []string{"--timeout", "2"}, "/bin/sh", "-c", "touch "+started+"; exec sleep 30")
	}()
	waitFile(t, started)
	release := holdWriterLock(t, root)
	time.Sleep(4 * time.Second)
	release()
	r := <-done
	noMalformed(t, r)
	if r.code != 124 || len(r.res.Items) != 1 || field(r.res.Items[0], "class").Str != transaction.RunTimeout {
		t.Fatalf("want a recorded timeout exiting 124: code %d %s", r.code, r.stdout)
	}
	recordedOutcome(t, root, r)
}

// TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout: a detached
// claim whose ceiling expires, or whose process is interrupted, while
// another process holds the writer lock keeps the retryable LOCK_TIMEOUT,
// never MALFORMED.
func TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout(t *testing.T) {
	t.Run("ceiling", func(t *testing.T) {
		root, repo, id := detachClaimStore(t)
		t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
		holdWriterLock(t, root)
		x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-contended", "--detach", "--wait", "60")
		noMalformed(t, x)
		if !hasCode(x.res, wire.CodeLockTimeout) || !bytes.Contains(x.stdout, []byte(`"retryable":true`)) {
			t.Fatalf("ceiling under contention: code %d %s", x.code, x.stdout)
		}
		if kept, err := os.ReadFile(filepath.Join(cli.LeaseDetachDir(repo, "claim-contended"), "result.json")); err != nil || !bytes.Equal(kept, x.stdout) {
			t.Fatalf("result not kept: %v", err)
		}
	})
	t.Run("interrupt", func(t *testing.T) {
		root, repo, id := detachClaimStore(t)
		// The child writes one byte to its log once its interrupt handling
		// is in place; the interrupt follows.
		t.Setenv("CORVINT_TEST_LEASE_DETACH_LOG_BYTES", "1")
		holdWriterLock(t, root)
		x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-interrupted", "--detach", "--wait", "0")
		if len(x.res.Items) != 1 || field(x.res.Items[0], "state").Str != "PENDING" {
			t.Fatalf("want PENDING: %s", x.stdout)
		}
		pid := field(x.res.Items[0], "childPid").Str
		n := 0
		for _, c := range pid {
			n = n*10 + int(c-'0')
		}
		if n <= 0 {
			t.Fatalf("no child pid: %s", x.stdout)
		}
		logPath := filepath.Join(cli.LeaseDetachDir(repo, "claim-interrupted"), "child.log")
		for i := 0; ; i++ {
			if st, err := os.Stat(logPath); err == nil && st.Size() > 0 {
				break
			}
			if i > 1000 {
				t.Fatal("the detached child never started")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := syscall.Kill(n, syscall.SIGINT); err != nil {
			t.Fatal(err)
		}
		y := cliRun(t, root, "claim", "--attach", "--request-id", "claim-interrupted", "--wait", "30")
		noMalformed(t, y)
		if !hasCode(y.res, wire.CodeLockTimeout) || !bytes.Contains(y.stdout, []byte(`"retryable":true`)) {
			t.Fatalf("interrupt under contention: code %d %s", y.code, y.stdout)
		}
	})
}

// TestATRV0066_KeptLockTimeoutRelaunchesOnSameRequestID: a detached claim
// that kept LOCK_TIMEOUT runs again when --detach is repeated under the same
// request id, keeping the earlier result as result.1.json, and claims once.
func TestATRV0066_KeptLockTimeoutRelaunchesOnSameRequestID(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	release := holdWriterLock(t, root)
	x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-retry", "--detach", "--wait", "60")
	if !hasCode(x.res, wire.CodeLockTimeout) {
		t.Fatalf("want a kept LOCK_TIMEOUT: code %d %s", x.code, x.stdout)
	}
	release()
	// The first child ends once its result is kept; while it still runs the
	// record, a repeat --detach keeps replaying LOCK_TIMEOUT.
	dir := cli.LeaseDetachDir(repo, "claim-retry")
	var child struct {
		Pid int `json:"pid"`
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "child.json")); err != nil || json.Unmarshal(raw, &child) != nil || child.Pid <= 0 {
		t.Fatalf("no child record: %v", err)
	}
	for i := 0; syscall.Kill(child.Pid, 0) == nil; i++ {
		if i > 3000 {
			t.Fatal("the first detached child never ended")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "120000")
	y := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-retry", "--detach", "--wait", "60")
	if y.code != 0 || y.res.Outcome != wire.OutcomeOK {
		t.Fatalf("same-id retry replayed instead of relaunching: code %d %s", y.code, y.stdout)
	}
	if prev, err := os.ReadFile(filepath.Join(dir, "result.1.json")); err != nil || !bytes.Equal(prev, x.stdout) {
		t.Fatalf("earlier LOCK_TIMEOUT result not kept as result.1.json: %v", err)
	}
	// The OK result is final: a third --detach replays it.
	z := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-retry", "--detach", "--wait", "60")
	if z.code != 0 || !bytes.Equal(z.stdout, y.stdout) {
		t.Fatalf("an OK result was not replayed: code %d %s", z.code, z.stdout)
	}
}
