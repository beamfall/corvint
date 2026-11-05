// SPDX-License-Identifier: Apache-2.0
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestATRV07203_PairedHoldSample is an opt-in worker for the paired BASE/HEAD
// experiment. The foreground driver alternates identical worker binaries at
// each size. Each worker retains its own fixture between samples; cache and
// writer checkpoint are cleared before both measured calls to exercise the
// complete route, not the unrelated fast writer route. Setup/submit/gate are
// outside the measured locks. Raw loads bracket every individual operation.
func TestATRV07203_PairedHoldSample(t *testing.T) {
	dir := os.Getenv("PK_TSTORE_PAIR_DIR")
	if dir == "" {
		t.Skip("paired hold experiment only")
	}
	n, err := strconv.Atoi(os.Getenv("PK_TSTORE_PAIR_SIZE"))
	if err != nil {
		t.Fatal(err)
	}
	repoPath, root := filepath.Join(dir, "repo"), filepath.Join(dir, "worktree")
	if os.Getenv("PK_TSTORE_PAIR_SETUP") == "1" {
		repo := historyStoreAt(t, repoPath, 880, n)
		if err := sealSegmentsLocked(context.Background(), repo); err != nil {
			t.Fatal(err)
		}
		gateLeasePolicy(t, repo)
		holdGit(t, root, "init", "-q", "-b", "main")
		holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
		return
	}
	repo, err := intent.Resolve(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets"))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := strconv.Atoi(os.Getenv("PK_TSTORE_PAIR_REP"))
	if err != nil {
		t.Fatal(err)
	}
	call := func(ctx context.Context, verb string, req transaction.LeaseRequest) *Report {
		t.Helper()
		out, err := Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("pair-%d-%s", rep, verb), Root: root, Lease: req}, WallClock())
		if err != nil || out.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %+v %v", verb, out, err)
		}
		return out
	}
	type sample struct {
		Version, Verb, LoadBefore, LoadAfter string
		Receipts, Repetition, Locks          int
		HoldMS                               float64
	}
	measure := func(verb string, req transaction.LeaseRequest) *Report {
		t.Helper()
		if err := os.Remove(journal.WriterCheckpointPath(repo.StateDir)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		forgetLeaseAudits()
		s := sample{Version: os.Getenv("PK_TSTORE_PAIR_VERSION"), Verb: verb, Receipts: n, Repetition: rep, LoadBefore: hostLoad()}
		ctx := authority.WithLockObserver(context.Background(), func(d time.Duration) { s.HoldMS += ms(d); s.Locks++ })
		out := call(ctx, verb, req)
		s.LoadAfter = hostLoad()
		if s.Locks != 1 {
			t.Fatalf("%s: expected one writer lock, got %d", verb, s.Locks)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(os.Getenv("PK_TSTORE_PAIR_OUTPUT"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := f.Write(append(raw, '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatalf("report: %v %v", writeErr, closeErr)
		}
		t.Log(string(raw))
		return out
	}
	claim := measure("claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: fixture.TicketID(strings.TrimSuffix(entries[rep].Name(), ".json")), Holder: "agent-1", LeaseMinutes: "60", Scope: []string{"src/"}})
	historyWrite(t, filepath.Join(root, "src", fmt.Sprintf("file%d.go", rep)), []byte("package src\n"))
	holdGit(t, root, "add", "-A")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "change")
	commit, tree := holdGitOut(t, root, "rev-parse", "HEAD"), holdGitOut(t, root, "rev-parse", "HEAD^{tree}")
	call(context.Background(), "submit", transaction.LeaseRequest{Verb: transaction.LeaseSubmit, AttemptID: claim.AttemptID, Generation: claim.Generation, Tree: tree})
	choice := LeaseChoice{QueueID: fixture.QueueID, RequestID: fmt.Sprintf("pair-%d-gate", rep), Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: claim.AttemptID, Generation: claim.Generation, Gate: "verify"}}
	gate, err := GateRun(context.Background(), repo, historyActor, choice, root, time.Now)
	if err != nil || gate.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("gate: %+v %v", gate, err)
	}
	measure("complete", transaction.LeaseRequest{Verb: transaction.LeaseComplete, AttemptID: claim.AttemptID, Generation: claim.Generation, Commit: commit})
}
