//go:build darwin || linux

package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// writerLockProbe reports whether a holder in this process has the writer
// lock, probing from a second open file description.
func writerLockProbe(t *testing.T, repo *intent.Repository) bool {
	f, err := os.Open(repo.LockPath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Errorf("probe open: %v", err)
		}
		return false
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true
	}
	if err != nil {
		t.Errorf("probe flock: %v", err)
		return false
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// derivedBytes is what one complete-route lease claim wrote to the derived
// checkpoint files, split by whether the writer lock was held.
type derivedBytes struct {
	locked, unlocked map[string]int
}

// completeRouteClaim runs one lease claim on the complete route of a store
// with tickets tickets and records its derived checkpoint writes.
//
// V1-2638: the property holds for a claim that commits in its first round.
// If the snapshot moves under that commit (a stray change event under host
// load), the next round resumes on the writer route, which retains the read
// checkpoint under the lock by design (CAL-V0-119) and writes no delta; see
// TestJSGV0015_MovedClaimResumesOnTheWriterRoute. Such a run measures that
// route, not this one, so a fresh fixture is taken, at most claimAttempts
// times.
func completeRouteClaim(t *testing.T, tickets int) (*intent.Repository, derivedBytes) {
	t.Helper()
	const claimAttempts = 3
	for i := 1; ; i++ {
		repo, got, timing := completeRouteClaimMoved(t, tickets, false)
		if timing.Rounds == 1 {
			return repo, got
		}
		t.Logf("%d tickets: the claim took %d rounds (resumed %d); the snapshot moved, so it is not a complete-route commit", tickets, timing.Rounds, timing.Resumed)
		if i == claimAttempts {
			t.Fatalf("%d tickets: no single-round claim in %d fixtures", tickets, claimAttempts)
		}
	}
}

// completeRouteClaimMoved is completeRouteClaim; with move set, an outside
// edit under the writer lock of the first commit moves the snapshot, so the
// claim takes a second round. It also returns the claim's lease timing.
func completeRouteClaimMoved(t *testing.T, tickets int, move bool) (*intent.Repository, derivedBytes, LeaseTiming) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := historyStoreAt(t, filepath.Join(base, "repo"), tickets, tickets+8)
	lowerWriterCheckpoint(t, repo)
	holdLeasePolicy(t, repo)
	root := filepath.Join(base, "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	// No writer checkpoint and no retained audit: the claim prepares and
	// commits on the complete route (commitLease).
	for _, p := range []string{journal.WriterCheckpointPath(repo.StateDir), journal.WriterBasePath(repo.StateDir), journal.CheckpointPath(repo.StateDir)} {
		os.Remove(p)
	}
	forgetLeaseAudits()
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	got := derivedBytes{locked: map[string]int{}, unlocked: map[string]int{}}
	derivedWritten = func(path string, n int) {
		if writerLockProbe(t, repo) {
			got.locked[filepath.Base(path)] += n
		} else {
			got.unlocked[filepath.Base(path)] += n
		}
	}
	defer func() { derivedWritten = nil }()
	local := strings.TrimSuffix(entries[0].Name(), ".json")
	var timing LeaseTiming
	ctx := WithLeaseTiming(context.Background(), &timing)
	if move {
		ticket := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", entries[1].Name())
		ctx = context.WithValue(ctx, mutationStageKey{}, at("fast.complete.guards", func() {
			raw, err := os.ReadFile(ticket)
			if err != nil {
				t.Fatal(err)
			}
			historyWrite(t, ticket, raw)
		}))
	}
	report, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("staged-claim-%d", tickets), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(local), Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim %+v %v", report, err)
	}
	return repo, got, timing
}

// JSG-V0-015 (V1-0850, CAL-V0-026): a complete-route lease commit encodes
// and writes the read checkpoint and the fresh writer-checkpoint base before
// the writer lock; under the lock it writes only the bounded writer delta,
// whose size does not grow with the ticket count. Retaining either file
// under the lock again (retainCheckpoint or retainWriterCheckpoint in
// commitLease) makes the locked bytes nonzero and ticket-proportional.
func TestJSGV0015_LeaseCommitStagesCheckpointsOutsideWriterLock(t *testing.T) {
	small, smallBytes := completeRouteClaim(t, 40)
	large, largeBytes := completeRouteClaim(t, 400)
	cpName := filepath.Base(journal.CheckpointPath(small.StateDir))
	baseName := filepath.Base(journal.WriterBasePath(small.StateDir))
	deltaName := filepath.Base(journal.WriterCheckpointPath(small.StateDir))
	for _, c := range []struct {
		tickets int
		got     derivedBytes
	}{{40, smallBytes}, {400, largeBytes}} {
		if c.got.locked[cpName] != 0 || c.got.locked[baseName] != 0 {
			t.Errorf("%d tickets: %v bytes of O(store) checkpoint written under the writer lock", c.tickets, c.got.locked)
		}
		if c.got.unlocked[cpName] == 0 || c.got.unlocked[baseName] == 0 {
			t.Errorf("%d tickets: nothing staged before the lock: %v", c.tickets, c.got.unlocked)
		}
		if c.got.locked[deltaName] == 0 {
			t.Errorf("%d tickets: no writer delta under the lock: %v", c.tickets, c.got.locked)
		}
	}
	// The delta carries a few decimal counts, so it may gain a digit; it
	// must not grow with the store the way the staged files do (8x here).
	if largeBytes.locked[deltaName] > smallBytes.locked[deltaName]+32 {
		t.Errorf("locked writer delta grew with tickets: %d at 40, %d at 400", smallBytes.locked[deltaName], largeBytes.locked[deltaName])
	}
	if largeBytes.unlocked[cpName] <= smallBytes.unlocked[cpName] {
		t.Errorf("staged read checkpoint did not grow with tickets: %d at 40, %d at 400", smallBytes.unlocked[cpName], largeBytes.unlocked[cpName])
	}
	t.Logf("locked %v / %v; staged %v / %v", smallBytes.locked, largeBytes.locked, smallBytes.unlocked, largeBytes.unlocked)

	// Recovery is unchanged: the published files are the ones a complete
	// audit derives, so a read resumed from them and the next writer agree
	// with a complete audit, and no staged temporary is left behind.
	for _, repo := range []*intent.Repository{small, large} {
		head, err := readHead(repo)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(journal.CheckpointPath(repo.StateDir))
		if err != nil {
			t.Fatal(err)
		}
		cp, err := journal.DecodeCheckpoint(raw)
		if err != nil || cp.Seq.Uint64() != head.LastSeq.Uint64()-1 {
			t.Fatalf("published checkpoint %+v %v, want the audited head %d", cp, err, head.LastSeq.Uint64()-1)
		}
		full, err := journalReader(repo, head).Audit()
		if err != nil {
			t.Fatal(err)
		}
		r := journalReader(repo, head)
		r.Checkpoint = cp
		resumed, err := r.Audit()
		if err != nil || resumed.Mode != journal.ModeCheckpoint || resumed.Identity.HeadSha256 != full.Identity.HeadSha256 || resumed.Identity.IntentTreeSha256 != full.Identity.IntentTreeSha256 || resumed.LastSeq != full.LastSeq || resumed.LastReceiptSha256 != full.LastReceiptSha256 || resumed.ProjectionAgreement != full.ProjectionAgreement {
			t.Fatalf("resumed audit %v mode %s head %v tree %v seq %v agreement %s; complete %v %v %v %s", err, resumed.Mode, resumed.Identity.HeadSha256, resumed.Identity.IntentTreeSha256, resumed.LastSeq, resumed.ProjectionAgreement, full.Identity.HeadSha256, full.Identity.IntentTreeSha256, full.LastSeq, full.ProjectionAgreement)
		}
		if wc := readWriterCheckpoint(repo); wc == nil || wc.Seq.Uint64() != head.LastSeq.Uint64()-1 {
			t.Fatalf("published writer checkpoint %+v, want seq %d", wc, head.LastSeq.Uint64()-1)
		}
		if left := stagedLeft(t, repo); len(left) != 0 {
			t.Fatalf("staged temporaries left: %v", left)
		}
	}
}

// V1-2638, V1-2358: an outside edit under the first commit's lock moves the
// snapshot; the second round resumes from the first round's audit on the
// writer route, which once retained the read checkpoint under the lock
// (CAL-V0-119). It now encodes and writes it after the lock is released
// (JSG-V0-891), so no read checkpoint byte is written under the lock.
func TestJSGV0015_MovedClaimResumesOnTheWriterRoute(t *testing.T) {
	_, got, timing := completeRouteClaimMoved(t, 40, true)
	cp := filepath.Base(journal.CheckpointPath("taskman"))
	if timing.Rounds != 2 || timing.Resumed != 1 || got.locked[cp] != 0 || got.unlocked[cp] == 0 {
		t.Fatalf("moved claim: rounds %d resumed %d, locked %v, staged %v", timing.Rounds, timing.Resumed, got.locked, got.unlocked)
	}
}

func stagedLeft(t *testing.T, repo *intent.Repository) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(repo.StateDir))
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		if strings.Contains(e.Name(), stagedInfix) {
			left = append(left, e.Name())
		}
	}
	return left
}

// JSG-V0-015: staged bytes are published only under the token they were
// encoded with and only while the lock holder still reads that token. A
// token the commit replaced falls back to encoding under the lock; a token
// some other writer replaced retains nothing. A stale staged temporary from
// a stopped process is swept; a fresh one is left to its owner.
func TestJSGV0015_StagedWriterCheckpointNeedsItsToken(t *testing.T) {
	t.Parallel()
	repo := writerStore(t, historyTickets+8)
	// A complete audit derives a base-free checkpoint, so its base is fresh.
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := journalReader(repo, head).AuditForMutation("staged-token")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := audit.WriterCheckpoint()
	if err != nil || !fresh.FreshBase() {
		t.Fatalf("complete audit checkpoint %v", err)
	}
	token := writerInvalidation(repo)
	deltaPath := journal.WriterCheckpointPath(repo.StateDir)

	staged := stageWriterCheckpoint(repo, fresh, token)
	if staged == nil {
		t.Fatal("nothing staged")
	}
	// Another writer replaces the token: nothing is retained, and the
	// staged base is discarded.
	invalidateWriterCheckpoint(context.Background(), repo)
	retainStagedWriterCheckpoint(repo, fresh, token, staged)
	staged.discard()
	if _, err := os.Lstat(deltaPath); !os.IsNotExist(err) {
		t.Fatalf("retained under a replaced token: %v", err)
	}
	if left := stagedLeft(t, repo); len(left) != 0 {
		t.Fatalf("staged temporaries left: %v", left)
	}

	// The commit's own token differs from the staged one: the fallback
	// encodes under the lock with the current token.
	current := writerInvalidation(repo)
	staged = stageWriterCheckpoint(repo, fresh, token)
	retainStagedWriterCheckpoint(repo, fresh, current, staged)
	staged.discard()
	if got := readWriterCheckpoint(repo); got == nil || got.Invalidation != current.digest() {
		t.Fatalf("fallback retained %+v, want the current token", got)
	}

	// The matching token publishes the staged base and delta.
	os.Remove(deltaPath)
	staged = stageWriterCheckpoint(repo, fresh, current)
	retainStagedWriterCheckpoint(repo, fresh, current, staged)
	staged.discard()
	if got := readWriterCheckpoint(repo); got == nil || got.Seq != fresh.Seq || got.Invalidation != current.digest() {
		t.Fatalf("staged publish retained %+v", got)
	}
	if left := stagedLeft(t, repo); len(left) != 0 {
		t.Fatalf("staged temporaries left: %v", left)
	}

	// Sweep: an old leftover is removed by the next stager, a fresh one
	// (another process's, mid-commit) is not.
	dest := journal.CheckpointPath(repo.StateDir)
	old, recent := dest+stagedInfix+"old", dest+stagedInfix+"recent"
	for _, p := range []string{old, recent} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * stagedMaxAge)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	s := stageDerived(dest, []byte("checkpoint"))
	if s == nil {
		t.Fatal("stage failed")
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Fatalf("stale staged temporary kept: %v", err)
	}
	if _, err := os.Lstat(recent); err != nil {
		t.Fatalf("fresh staged temporary removed: %v", err)
	}
	if !s.publish() {
		t.Fatal("publish failed")
	}
	if raw, err := os.ReadFile(dest); err != nil || !bytes.Equal(raw, []byte("checkpoint")) {
		t.Fatalf("published %q %v", raw, err)
	}
	os.Remove(recent)
}
