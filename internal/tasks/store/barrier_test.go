package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func barrierRequest(operation, id string) store.BarrierRequest {
	return store.BarrierRequest{QueueID: fixture.QueueID, RequestID: id, Operation: operation}
}
func changeBarrier(t *testing.T, repo *intent.Repository, operation, id string) *store.Report {
	t.Helper()
	report, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(operation, id), now(t))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestTMV0016_AS27_NativeBarrierCycleAndReplay(t *testing.T) {
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("create", mutation.OpCreate, "", "", createPayload("barrier cycle")))
	pause := changeBarrier(t, repo, transaction.Pause, "pause")
	if pause.Receipt == "" || pause.Ticket != "" {
		t.Fatalf("pause: %+v", pause)
	}
	barrierPath := filepath.Join(repo.StateDir, "barrier.json")
	raw, err := os.ReadFile(barrierPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := snapshot.DecodeBarrier(raw)
	if err != nil || b.Scope != "ADMISSION" || b.Reason != "OPERATOR" {
		t.Fatalf("barrier: %+v %v", b, err)
	}
	before := storeDigest(t, repo)
	noChange := changeBarrier(t, repo, transaction.Pause, "already-paused")
	if noChange.Kind != "NoChange" || storeDigest(t, repo) != before {
		t.Fatalf("repeat pause: %+v", noChange)
	}
	edit := mutate(t, repo, envelope("edit", mutation.OpPrioritize, created.Ticket, "1", obj("priority", str("P1"), "order", str("0"))))
	if edit.Receipt == "" {
		t.Fatalf("ADMISSION blocked mutation: %+v", edit)
	}
	unpause := changeBarrier(t, repo, transaction.Unpause, "unpause")
	if unpause.Receipt == "" {
		t.Fatalf("unpause: %+v", unpause)
	}
	if _, err := os.Lstat(barrierPath); !os.IsNotExist(err) {
		t.Fatalf("barrier not removed: %v", err)
	}
	rcRaw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", unpause.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(rcRaw)
	if err != nil {
		t.Fatal(err)
	}
	deleted := false
	for i, p := range rc.Post {
		if p.Path == "barrier.json" {
			deleted = p.Sha256 == nil && rc.Pre[i].Sha256 != nil && *rc.Pre[i].Sha256 == wire.Sum(raw)
		}
	}
	if !deleted || rc.Kind != "UNPAUSE" {
		t.Fatalf("deletion receipt: %+v", rc)
	}
	before = storeDigest(t, repo)
	for _, step := range []struct {
		op, id string
		replay bool
	}{{transaction.Pause, "pause", true}, {transaction.Unpause, "unpause", true}, {transaction.Unpause, "already-unpaused", false}} {
		got := changeBarrier(t, repo, step.op, step.id)
		if got.Outcome.Replayed != step.replay || got.Receipt != "" || storeDigest(t, repo) != before {
			t.Fatalf("retry changed later state: %+v", got)
		}
	}
	conflict := changeBarrier(t, repo, transaction.Unpause, "pause")
	if !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, repo) != before {
		t.Fatalf("operation conflict: %+v", conflict)
	}
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	if _, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit(); err != nil {
		t.Fatal(err)
	}
}

func TestTMV0016_AS27_UnpausePreservesMultipleDivergentTickets(t *testing.T) {
	repo, _ := initialized(t)
	for _, id := range []string{"first", "second"} {
		mutate(t, repo, envelope(id, mutation.OpCreate, "", "", createPayload(id)))
	}
	changeBarrier(t, repo, transaction.Pause, "pause")
	fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "AT-0002.json"), []byte{})
	fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "AT-0003.json"), []byte("malformed edit"))
	orphan := []byte("retained orphan")
	fixture.Write(t, filepath.Join(repo.StateDir, "evidence", string(wire.Sum(orphan))), orphan)
	intents := fixture.TreeSnapshot(t, filepath.Join(repo.PrimaryWorktree, intent.Dir))
	evidence := fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "evidence"))
	before := storeDigest(t, repo)
	_, err := store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Pause, "new-pause"), now(t))
	if wire.CodeOf(err) != wire.CodeIntentDiverged || storeDigest(t, repo) != before {
		t.Fatalf("pause bypassed drift: %v", err)
	}
	if got := changeBarrier(t, repo, transaction.Unpause, "remove"); got.Receipt == "" {
		t.Fatalf("unpause blocked: %+v", got)
	}
	if !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, filepath.Join(repo.PrimaryWorktree, intent.Dir))) || !reflect.DeepEqual(evidence, fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "evidence"))) {
		t.Fatal("unpause changed edits or evidence")
	}
}

func TestTMV0009_AS11_BarrierPublicationReturnedFaults(t *testing.T) {
	for _, point := range []string{"precommit", "predelete"} {
		t.Run(point, func(t *testing.T) {
			repo, _ := initialized(t)
			changeBarrier(t, repo, transaction.Pause, "pause")
			read := func(path string) []byte {
				t.Helper()
				raw, err := os.ReadFile(filepath.Join(repo.StateDir, path))
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			headBefore, barrierBefore := read("head.json"), read("barrier.json")
			before := storeDigest(t, repo)
			fault := errors.New("returned " + point + " fault")
			hook := func() error { return fault }
			var commit, remove func() error
			if point == "precommit" {
				commit = hook
			} else {
				remove = hook
			}
			report, err := store.BarrierWithFaultsForTest(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t), commit, remove)
			if !errors.Is(err, fault) || !bytes.Equal(read("head.json"), headBefore) || !bytes.Equal(read("barrier.json"), barrierBefore) {
				t.Fatalf("ordering: %+v %v", report, err)
			}
			entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("staging: %v %v", entries, err)
			}
			if point == "precommit" {
				if report.Receipt != "" || storeDigest(t, repo) != before {
					t.Fatal("precommit fault changed state")
				}
				if got := changeBarrier(t, repo, transaction.Unpause, "remove"); got.Receipt == "" {
					t.Fatalf("retry: %+v", got)
				}
				return
			}
			if report.Receipt == "" {
				t.Fatal("deletion attempted without receipt")
			}
			before = storeDigest(t, repo)
			_, err = store.Barrier(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t))
			if wire.CodeOf(err) != wire.CodeRedoPending || storeDigest(t, repo) != before {
				t.Fatalf("pending unpause changed: %v", err)
			}
			refuseUnchanged(t, repo, wire.CodeUnsupported)
		})
	}
}

func TestTMV0016_AS27_UnpauseALLAndBranchIndependence(t *testing.T) {
	repo, _ := initialized(t)
	reconciliationBarrier(t, repo, "ALL")
	before := storeDigest(t, repo)
	refused := changeBarrier(t, repo, transaction.Pause, "pause")
	if !refused.Outcome.HasCode(wire.CodePaused) || storeDigest(t, repo) != before {
		t.Fatalf("ALL pause: %+v", refused)
	}
	fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("0000000000000000000000000000000000000000\n"))
	if got := changeBarrier(t, repo, transaction.Unpause, "remove"); got.Receipt == "" {
		t.Fatalf("ALL unpause: %+v", got)
	}
	if got := changeBarrier(t, repo, transaction.Pause, "new-pause"); got.Receipt == "" {
		t.Fatalf("branch restricted pause: %+v", got)
	}
	before = storeDigest(t, repo)
	actor := operator()
	actor.Role = "OPERATOR"
	report, err := store.Barrier(context.Background(), repo, actor, barrierRequest(transaction.Pause, "new-pause"), now(t))
	if err != nil || !report.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, repo) != before {
		t.Fatalf("actor conflict: %+v %v", report, err)
	}
}

func TestTMV0009_AS11_BarrierRefusalsPreserveStore(t *testing.T) {
	for _, kind := range []string{"missing", "extra", "queue", "policy", "private", "pending", "staging", "restore", "primary", "version", "role", "identity", "scope", "request", "operation"} {
		t.Run(kind, func(t *testing.T) {
			repo, _ := initialized(t)
			mutate(t, repo, envelope("create", mutation.OpCreate, "", "", createPayload("canonical")))
			changeBarrier(t, repo, transaction.Pause, "pause")
			request := barrierRequest(transaction.Unpause, "remove")
			actor := operator()
			switch kind {
			case "missing":
				if err := os.Remove(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "AT-0002.json")); err != nil {
					t.Fatal(err)
				}
			case "extra":
				fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "EXTRA.json"), []byte("untracked"))
			case "queue", "policy":
				fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, kind+".json"), []byte("{}\n"))
			case "private":
				fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{}\n"))
			case "pending":
				repo = pendingMutation(t)
			case "staging":
				emptyActiveDescriptor(t, repo)
			case "restore":
				fixture.Write(t, filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete"))
			case "primary":
				editJSON(t, filepath.Join(repo.StateDir, "head.json"), func(v wire.Value) { v.Obj.Set("primaryWorktree", str("/private/tmp/other")) })
			case "version":
				fixture.Write(t, filepath.Join(repo.StateDir, "VERSION"), []byte("future\n"))
			case "role":
				actor.Role = "WORKER"
			case "identity":
				actor.ID = ""
			case "scope":
				request.QueueID = "queue:other:main"
			case "request":
				request.RequestID = ""
			case "operation":
				request.Operation = transaction.Mutate
			}
			before := storeDigest(t, repo)
			report, err := store.Barrier(context.Background(), repo, actor, request, now(t))
			if err == nil && report.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatalf("%s accepted: %+v", kind, report)
			}
			if report.Receipt != "" || storeDigest(t, repo) != before {
				t.Fatalf("%s changed store", kind)
			}
		})
	}
}

func TestTMV0009_AS11_ChangedBarrierAfterReceiptStopsBeforeHead(t *testing.T) {
	repo, _ := initialized(t)
	changeBarrier(t, repo, transaction.Pause, "pause")
	headPath := filepath.Join(repo.StateDir, "head.json")
	headBefore, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.StateDir, "barrier.json")
	changed := []byte("third value at deletion boundary")
	report, err := store.BarrierWithFaultsForTest(context.Background(), repo, operator(), barrierRequest(transaction.Unpause, "remove"), now(t), nil, func() error { fixture.Write(t, path, changed); return nil })
	if err == nil || report.Receipt == "" {
		t.Fatalf("changed pre accepted: %+v %v", report, err)
	}
	current, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(current, changed) {
		t.Fatal("third-value barrier deleted")
	}
	headAfter, readErr := os.ReadFile(headPath)
	if readErr != nil || !bytes.Equal(headBefore, headAfter) {
		t.Fatal("head advanced after deletion refusal")
	}
	entries, readErr := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("staging: %v %v", entries, readErr)
	}
}
