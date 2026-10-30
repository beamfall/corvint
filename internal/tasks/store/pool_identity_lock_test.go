//go:build unix

package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const lockIdentitySource = `{"revision":"1111111111111111111111111111111111111111","tree":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`

// identityLockFixture is a store with pool db of members a and b. Member a's
// identity command blocks only while dir/gate exists, and then until
// dir/release does; each blocking run appends to dir/count. Member b's
// identity command never blocks. Member a is QUARANTINED by a passed guarded
// refresh, waiting for confirm-safe.
type identityLockFixture struct {
	repo   *intent.Repository
	dir    string
	root   string
	safe   LeaseChoice
	lifted func(context.Context) context.Context
}

// identityLockPolicy is the fixture policy at version with maxActive;
// member a's identity command watches dir and member b's watches quiet.
func identityLockPolicy(dir, quiet, version, maxActive string) wire.Value {
	script := `if [ -e "$1/gate" ]; then printf x >> "$1/count"; : > "$1/started"; while [ ! -e "$1/release" ]; do sleep 0.02; done; fi; printf '%s\n' '` + lockIdentitySource + `'`
	member := func(at string) wire.Value {
		identity := historyObject("argv", wire.Strings([]string{"/bin/sh", "-c", script, "identity", at}), "cwd", wire.String("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", wire.String("120"))
		return historyObject("identity", identity, "operations", historyObject("refresh", historyObject("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", wire.String("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", wire.String("5"))))
	}
	source := historyObject("revision", wire.String("1111111111111111111111111111111111111111"), "tree", wire.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String(version))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String(maxActive), "maxWorkersTotal", wire.String(maxActive), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	v.Obj.Set("pools", wire.Array(historyObject("id", wire.String("db"), "members", wire.Strings([]string{"a", "b"}), "memberConfig", historyObject("a", member(dir), "b", member(quiet)), "allowedSources", wire.Array(source), "defaultSource", source)))
	return v
}

func newIdentityLockFixture(t *testing.T) *identityLockFixture {
	t.Helper()
	f := &identityLockFixture{repo: writerStore(t, 70), dir: t.TempDir()}
	if rep, err := PolicyUpdate(context.Background(), f.repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "identity-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(identityLockPolicy(f.dir, t.TempDir(), "2", "4"))}, WallClock()); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", rep, err)
	}
	f.root = filepath.Join(filepath.Dir(f.repo.PrimaryWorktree), "worktree")
	holdGit(t, f.root, "init", "-q", "-b", "main")
	holdGit(t, f.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	f.lifted = func(ctx context.Context) context.Context {
		return context.WithValue(ctx, sweepObservationBoundKey{}, time.Minute)
	}
	// A passed guarded refresh leaves the member QUARANTINED until an
	// operator confirms it safe (PSR-V0-163).
	f.refresh(t, "a")
	if run := writerMutate(t, f.repo, "identity-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	en := quarantinedMember(t, f.repo, "a")
	f.safe = LeaseChoice{QueueID: fixture.QueueID, RequestID: "identity-safe", Root: f.root, Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "a", Allocation: string(en.AllocationID), Evidence: "operator-reset-record", Reason: "fixture"}}
	return f
}

func (f *identityLockFixture) refresh(t *testing.T, member string) {
	t.Helper()
	refresh := LeaseChoice{QueueID: fixture.QueueID, RequestID: "identity-refresh-" + member, Root: f.root, Lease: transaction.LeaseRequest{Pool: "db", Member: member, Operation: "refresh"}}
	if rep, err := PoolOperate(f.lifted(context.Background()), f.repo, historyActor, refresh); err != nil || rep.PoolOperation == nil {
		t.Fatalf("refresh %s: %+v %v", member, rep, err)
	}
}

// confirm runs member a's confirm-safe with its identity command blocked,
// calls during while the command is blocked, then releases it. It returns
// the write and the number of identity runs while the gate existed.
func (f *identityLockFixture) confirm(t *testing.T, during func()) (writerRun, int) {
	t.Helper()
	at := func(name string) string { return filepath.Join(f.dir, name) }
	if err := os.WriteFile(at("gate"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := WallClock()
	done := make(chan writerRun, 1)
	go func() {
		done <- stagedWrite(func(ctx context.Context) (*Report, error) {
			return Lease(f.lifted(ctx), f.repo, historyActor, f.safe, now)
		}, nil)
	}()
	release := func() { _ = os.WriteFile(at("release"), nil, 0o600) }
	defer release()
	waitFile(t, at("started"), done)
	// The identity command is now blocked and cannot finish before release
	// exists.
	during()
	if _, err := os.Stat(at("release")); !os.IsNotExist(err) {
		t.Fatalf("release existed before the concurrent writer committed: %v", err)
	}
	release()
	var run writerRun
	select {
	case run = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("confirm-safe never returned after release (hang guard)")
	}
	raw, _ := os.ReadFile(at("count"))
	return run, len(raw)
}

// identityDeclines counts the writer route's identity deferrals.
func identityDeclines(run writerRun) int {
	n := 0
	for _, s := range run.stages {
		if why, ok := strings.CutPrefix(s, "fast.declined: "); ok && strings.Contains(why, "identity") {
			n++
		}
	}
	return n
}

// PSR-V0-168 (V1-1656): the writer route never runs a guarded member's
// identity command under the writer lock. With a slow identity command
// blocked mid-run, a concurrent writer commits; once the command finishes,
// the confirmation commits on the writer route against the observation it
// revalidated under the lock, and the command ran exactly once.
func TestPSRV0168_IdentityProbeOutsideWriterLock(t *testing.T) {
	t.Parallel()
	f := newIdentityLockFixture(t)
	run, count := f.confirm(t, func() {
		concurrent := writerMutate(t, f.repo, "identity-concurrent", nil)
		if !concurrent.completed() {
			t.Fatalf("a concurrent writer was blocked by the identity probe: %+v %v %v", concurrent.rep, concurrent.err, concurrent.stages)
		}
	})
	if !run.completed() || !run.fast() {
		t.Fatalf("confirm-safe not committed on the writer route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if n := identityDeclines(run); n != 1 {
		t.Fatalf("writer route deferred the identity probe %d times, want 1: %v", n, run.stages)
	}
	if count != 1 {
		t.Fatalf("identity command ran %d times, want once", count)
	}
	if got := memberEntry(t, f.repo, "a"); got != nil && got.State != "FREE" {
		t.Fatalf("confirmed member not free: %+v", got)
	}
}

// PSR-V0-168: an observation made outside the lock is served only for
// byte-identical policy and pool state. When a concurrent writer changes the
// policy or the pool state while the identity command runs, the retried
// writer route declines again and the complete route observes afresh: the
// stale observation is never committed.
func TestPSRV0168_IdentityProbeStaleStateNotReused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, f *identityLockFixture)
	}{
		{"policy", func(t *testing.T, f *identityLockFixture) {
			if rep, err := PolicyUpdate(context.Background(), f.repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "identity-policy-moved", ExpectedPolicyVersion: "2", Policy: wire.EncodeFile(identityLockPolicy(f.dir, t.TempDir(), "3", "5"))}, WallClock()); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("concurrent policy update: %+v %v", rep, err)
			}
		}},
		{"pools", func(t *testing.T, f *identityLockFixture) {
			before := poolsBytes(t, f.repo)
			f.refresh(t, "b")
			if string(poolsBytes(t, f.repo)) == string(before) {
				t.Fatal("concurrent refresh left the pool state unchanged")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newIdentityLockFixture(t)
			run, count := f.confirm(t, func() { tc.change(t, f) })
			if !run.completed() {
				t.Fatalf("confirm-safe not committed: %+v %v %v", run.rep, run.err, run.stages)
			}
			if n := identityDeclines(run); n != 2 || run.fast() {
				t.Fatalf("stale observation reused: %d identity declines (want 2), fast %v: %v", n, run.fast(), run.stages)
			}
			if count != 2 {
				t.Fatalf("identity command ran %d times, want a fresh probe (2)", count)
			}
			if got := memberEntry(t, f.repo, "a"); got != nil && got.State != "FREE" {
				t.Fatalf("confirmed member not free: %+v", got)
			}
		})
	}
}

func poolsBytes(t *testing.T, repo *intent.Repository) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func memberEntry(t *testing.T, repo *intent.Repository, member string) *snapshot.PoolEntry {
	t.Helper()
	state, err := snapshot.DecodePools(poolsBytes(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	for i := range state.Entries {
		if state.Entries[i].MemberID == member {
			return &state.Entries[i]
		}
	}
	return nil
}

func quarantinedMember(t *testing.T, repo *intent.Repository, member string) *snapshot.PoolEntry {
	t.Helper()
	en := memberEntry(t, repo, member)
	if en == nil || en.State != "QUARANTINED" {
		t.Fatalf("member %s not quarantined: %+v", member, en)
	}
	return en
}

// waitFile waits for path to exist; a write that returns first ends the wait
// as a failure.
func waitFile(t *testing.T, path string, done <-chan writerRun) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case run := <-done:
			t.Fatalf("write returned before the identity command started: %+v %v %v", run.rep, run.err, run.stages)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatalf("identity command never started (hang guard)")
}
