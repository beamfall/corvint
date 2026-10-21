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

// PSR-V0-168 (V1-1656): the writer route never runs a guarded member's
// identity command under the writer lock. With a slow identity command
// blocked mid-run, a concurrent writer commits; once the command finishes,
// the confirmation commits on the writer route against the observation it
// revalidated under the lock, and the command ran exactly once.
func TestPSRV0168_IdentityProbeOutsideWriterLock(t *testing.T) {
	repo := writerStore(t, 70)
	dir := t.TempDir()
	at := func(name string) string { return filepath.Join(dir, name) }
	// The command blocks only while the gate file exists, and then until
	// the release file does; each blocking run appends to count.
	script := `if [ -e "$1/gate" ]; then printf x >> "$1/count"; : > "$1/started"; while [ ! -e "$1/release" ]; do sleep 0.02; done; fi; printf '%s\n' '` + lockIdentitySource + `'`
	identity := historyObject("argv", wire.Strings([]string{"/bin/sh", "-c", script, "identity", dir}), "cwd", wire.String("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", wire.String("120"))
	source := historyObject("revision", wire.String("1111111111111111111111111111111111111111"), "tree", wire.String("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	member := historyObject("identity", identity, "operations", historyObject("refresh", historyObject("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"}), "cwd", wire.String("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", wire.String("5"))))
	v.Obj.Set("pools", wire.Array(historyObject("id", wire.String("db"), "members", wire.Strings([]string{"a"}), "memberConfig", historyObject("a", member), "allowedSources", wire.Array(source), "defaultSource", source)))
	if rep, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "identity-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock()); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", rep, err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	lifted := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, sweepObservationBoundKey{}, time.Minute)
	}
	// A passed guarded refresh leaves the member QUARANTINED until an
	// operator confirms it safe (PSR-V0-163).
	refresh := LeaseChoice{QueueID: fixture.QueueID, RequestID: "identity-refresh", Root: root, Lease: transaction.LeaseRequest{Pool: "db", Member: "a", Operation: "refresh"}}
	if rep, err := PoolOperate(lifted(context.Background()), repo, historyActor, refresh); err != nil || rep.PoolOperation == nil {
		t.Fatalf("refresh: %+v %v", rep, err)
	}
	if run := writerMutate(t, repo, "identity-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	en := quarantinedMember(t, repo, "a")
	if err := os.WriteFile(at("gate"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := WallClock()
	safe := LeaseChoice{QueueID: fixture.QueueID, RequestID: "identity-safe", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: "a", Allocation: string(en.AllocationID), Evidence: "operator-reset-record", Reason: "fixture"}}
	done := make(chan writerRun, 1)
	go func() {
		done <- stagedWrite(func(ctx context.Context) (*Report, error) { return Lease(lifted(ctx), repo, historyActor, safe, now) }, nil)
	}()
	release := func() { _ = os.WriteFile(at("release"), nil, 0o600) }
	defer release()
	waitFile(t, at("started"), done)
	// The identity command is now blocked and cannot finish before release
	// exists. A writer that needs the writer lock must still commit.
	concurrent := writerMutate(t, repo, "identity-concurrent", nil)
	if !concurrent.completed() {
		t.Fatalf("a concurrent writer was blocked by the identity probe: %+v %v %v", concurrent.rep, concurrent.err, concurrent.stages)
	}
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
	if !run.completed() || !run.fast() {
		t.Fatalf("confirm-safe not committed on the writer route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if why := run.declined(); !strings.Contains(why, "identity") {
		t.Fatalf("writer route did not defer the identity probe: %q %v", why, run.stages)
	}
	if raw, _ := os.ReadFile(at("count")); string(raw) != "x" {
		t.Fatalf("identity command ran %d times, want once", len(raw))
	}
	if got := memberEntry(t, repo, "a"); got != nil && got.State != "FREE" {
		t.Fatalf("confirmed member not free: %+v", got)
	}
}

func memberEntry(t *testing.T, repo *intent.Repository, member string) *snapshot.PoolEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := snapshot.DecodePools(raw)
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
