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

func reconcileFixture(t *testing.T) (*intent.Repository, store.ReconcileRequest, string, []byte) {
	t.Helper()
	repo, _ := initialized(t)
	created := mutate(t, repo, envelope("create", mutation.OpCreate, "", "", createPayload("canonical")))
	id, err := wire.ParseTicketID("", created.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", id.Local+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	request := store.ReconcileRequest{RequestID: "reconcile", TargetID: created.Ticket, Choice: transaction.KeepJournal, File: []byte("{manual malformed edit\n"), CanonicalSha256: wire.Sum(raw)}
	fixture.Write(t, path, request.File)
	return repo, request, path, raw
}

func reconcile(t *testing.T, repo *intent.Repository, request store.ReconcileRequest) *store.Report {
	t.Helper()
	report, err := store.Reconcile(context.Background(), repo, operator(), request, now(t))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestTMV0007_AS35_NativeKeepThenMutation(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "malformed", true: "empty"}[empty], func(t *testing.T) {
			repo, request, path, canonical := reconcileFixture(t)
			if empty {
				request.File = []byte{}
				fixture.Write(t, path, request.File)
			}
			first := reconcile(t, repo, request)
			if first.Outcome.Outcome != mutation.OutcomeCompleted || first.Receipt == "" || first.Ticket != request.TargetID {
				t.Fatalf("KEEP: %+v", first)
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, canonical) {
				t.Fatalf("canonical not restored: %v", err)
			}
			discarded, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(wire.Sum(request.File))))
			if err != nil || !bytes.Equal(discarded, request.File) {
				t.Fatalf("discarded bytes lost: %v", err)
			}
			next := mutate(t, repo, envelope("after", mutation.OpPrioritize, request.TargetID, "1", obj("priority", str("P1"), "order", str("0"))))
			if next.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("mutation remains blocked: %+v", next)
			}
			fixture.Write(t, path, []byte("a later edit"))
			fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
			before := storeDigest(t, repo)
			replay := reconcile(t, repo, request)
			if !replay.Outcome.Replayed || replay.Ticket != first.Ticket || replay.Receipt != "" {
				t.Fatalf("replay lost original choice: %+v", replay)
			}
			if storeDigest(t, repo) != before {
				t.Fatal("replay rewrote later state")
			}
			request.File = []byte("different original choice")
			conflict := reconcile(t, repo, request)
			if !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, repo) != before {
				t.Fatalf("different choice not refused: %+v", conflict)
			}
		})
	}
}

func TestTMV0007_AS35_NativeAdoptThenMutation(t *testing.T) {
	repo, request, path, canonical := reconcileFixture(t)
	value, err := wire.Parse(canonical)
	if err != nil {
		t.Fatal(err)
	}
	value.Obj.Set("title", str("adopted title"))
	value.Obj.Set("acceptanceCriteria", wire.Strings([]string{"new criterion"}))
	request.Choice = transaction.AdoptFile
	request.CanonicalSha256 = ""
	request.File = wire.EncodeFile(value)
	fixture.Write(t, path, request.File)
	first := reconcile(t, repo, request)
	if first.Outcome.Outcome != mutation.OutcomeCompleted || *first.Outcome.ResultingRevision != "2" || *first.Outcome.ResultingAcceptanceRevision != "2" {
		t.Fatalf("ADOPT: %+v", first)
	}
	before := storeDigest(t, repo)
	again := reconcile(t, repo, request)
	if !again.Outcome.Replayed || again.Ticket != first.Ticket || storeDigest(t, repo) != before {
		t.Fatalf("adopt retry: %+v", again)
	}
	next := mutate(t, repo, envelope("after", mutation.OpPrioritize, request.TargetID, "2", obj("priority", str("P1"), "order", str("0"))))
	if next.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("after adopt: %+v", next)
	}
}

func TestTMV0007_AS35_NativeReconcileRefusalsPreserveInput(t *testing.T) {
	for _, kind := range []string{"protected", "different-file", "stale-canonical", "other-drift", "wrong-branch", "detached", "restore", "primary", "version", "role", "scope"} {
		t.Run(kind, func(t *testing.T) {
			repo, request, path, canonical := reconcileFixture(t)
			actor := operator()
			switch kind {
			case "protected":
				value, err := wire.Parse(canonical)
				if err != nil {
					t.Fatal(err)
				}
				value.Obj.Set("revision", str("99"))
				request.File = wire.EncodeFile(value)
				request.Choice = transaction.AdoptFile
				request.CanonicalSha256 = ""
				fixture.Write(t, path, request.File)
			case "different-file":
				request.File = []byte("another file")
			case "stale-canonical":
				request.CanonicalSha256 = wire.Sum([]byte("wrong canonical record"))
			case "other-drift":
				fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "OTHER.json"), fixture.Ticket("OTHER").Encode())
			case "wrong-branch":
				fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/other\n"))
			case "detached":
				fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("0000000000000000000000000000000000000000\n"))
			case "version":
				fixture.Write(t, filepath.Join(repo.StateDir, "VERSION"), []byte("future\n"))
			case "restore":
				fixture.Write(t, filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete\n"))
			case "primary":
				editJSON(t, filepath.Join(repo.StateDir, "head.json"), func(v wire.Value) { v.Obj.Set("primaryWorktree", str("/private/tmp/elsewhere")) })
			case "role":
				actor.Role = "WORKER"
			case "scope":
				request.TargetID = "ticket:other:main:AT-0002"
			}
			before := storeDigest(t, repo)
			report, err := store.Reconcile(context.Background(), repo, actor, request, now(t))
			if err == nil && report.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatalf("%s accepted: %+v", kind, report)
			}
			if report.Receipt != "" || storeDigest(t, repo) != before {
				t.Fatalf("%s changed store", kind)
			}
		})
	}
}

func TestTMV0009_AS11_NativeReconcileRefusesPending(t *testing.T) {
	repo := pendingMutation(t)
	target := "ticket:acme:main:AT-0002"
	raw, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", "AT-0002.json"))
	if err != nil {
		t.Fatal(err)
	}
	request := store.ReconcileRequest{RequestID: "pending-reconcile", TargetID: target, Choice: transaction.KeepJournal, File: raw, CanonicalSha256: wire.Sum(raw)}
	before := storeDigest(t, repo)
	report, err := store.Reconcile(context.Background(), repo, operator(), request, now(t))
	if wire.CodeOf(err) != wire.CodeRedoPending || report.Redone || storeDigest(t, repo) != before {
		t.Fatalf("pending recovery not refused: %+v %v", report, err)
	}
}

// A precommit descriptor with no occupied slots is valid evidence, not a grant
// for a fresh writer to reuse those slots or erase the unfinished transaction.
func emptyActiveDescriptor(t *testing.T, repo *intent.Repository) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		t.Fatal(err)
	}
	name, err := snapshot.ReceiptName(head.LastSeq.Uint64() + 1)
	if err != nil {
		t.Fatal(err)
	}
	requestPath, err := snapshot.RequestPath("unfinished")
	if err != nil {
		t.Fatal(err)
	}
	dummy := wire.Sum([]byte("not yet staged"))
	descriptor := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: snapshot.StageUnpause, RequestID: "unfinished", RequestSha256: dummy, RecordedAt: now(t), Base: &snapshot.StageBase{LastSeq: head.LastSeq, LastReceiptSha256: *head.LastReceiptSha256}, Artifacts: []snapshot.StageDescription{
		{Slot: "a00", Role: "HEAD", Target: "head.json", Sha256: dummy, Bytes: "1"},
		{Slot: "a01", Role: "POST", Target: requestPath, Sha256: dummy, Bytes: "1"},
		{Slot: "a02", Role: "RECEIPT", Target: "receipts/" + name, Sha256: dummy, Bytes: "1"},
	}}
	data, err := descriptor.Encode()
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(repo.StateDir, "staging", "active.json"), data)
}

func TestTMV0009_AS11_ActiveDescriptorBlocksFreshWriters(t *testing.T) {
	repo, request, path, canonical := reconcileFixture(t)
	fixture.Write(t, path, canonical)
	emptyActiveDescriptor(t, repo)
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	reader := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}
	proof, err := reader.Audit()
	if err != nil || !proof.StagingPresent {
		t.Fatalf("descriptor is not a valid active fixture: %+v %v", proof, err)
	}
	refuseUnchanged(t, repo, wire.CodeUnsupported)
	fixture.Write(t, path, request.File)
	proof, err = reader.Reconciliation(request.TargetID)
	if err != nil || !proof.StagingPresent {
		t.Fatalf("reconcile descriptor: %+v %v", proof, err)
	}
	before := storeDigest(t, repo)
	report, err := store.Reconcile(context.Background(), repo, operator(), request, now(t))
	if wire.CodeOf(err) != wire.CodeUnsupported || report.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("active staging changed: %+v %v", report, err)
	}
}

func TestTMV0009_AS11_KeepEvidencePrecedesCommitAndSurvivesReturnedFault(t *testing.T) {
	repo, request, path, _ := reconcileFixture(t)
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	headPath := filepath.Join(repo.StateDir, "head.json")
	headBefore := read(headPath)
	requests := fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "requests"))
	receipts := fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "receipts"))
	fault := errors.New("returned precommit fault")
	called := false
	report, err := store.ReconcileBeforeCommitForTest(context.Background(), repo, operator(), request, now(t), func() error {
		called = true
		if !bytes.Equal(read(filepath.Join(repo.StateDir, "evidence", string(wire.Sum(request.File)))), request.File) {
			t.Fatal("discarded evidence was not published before commit")
		}
		return fault
	})
	if !called || !errors.Is(err, fault) || report.Receipt != "" {
		t.Fatalf("fault: %+v %v", report, err)
	}
	if !bytes.Equal(read(headPath), headBefore) || !bytes.Equal(read(path), request.File) {
		t.Fatal("precommit fault changed head or projection")
	}
	if !reflect.DeepEqual(requests, fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "requests"))) || !reflect.DeepEqual(receipts, fixture.TreeSnapshot(t, filepath.Join(repo.StateDir, "receipts"))) {
		t.Fatal("precommit fault changed request index or receipts")
	}
	entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging not cleaned: %v %v", entries, err)
	}
	if retried := reconcile(t, repo, request); retried.Receipt == "" {
		t.Fatalf("orphan prevented retry: %+v", retried)
	}
}

// Materialize a journalled barrier in a test-owned queue. This is not a callable
// PAUSE writer and makes no historical acceptance claim.
func reconciliationBarrier(t *testing.T, repo *intent.Repository, scope string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	head, err := snapshot.DecodeHead(raw)
	if err != nil {
		t.Fatal(err)
	}
	seq := wire.SizeOf(head.LastSeq.Uint64() + 1)
	barrier := wire.EncodeFile(obj("profile", str(snapshot.ProfileBarrier), "queueId", str(fixture.QueueID), "scope", str(scope), "reason", str("OPERATOR"), "actor", str("tester"), "sinceSeq", str(string(seq)), "since", str(issued)))
	requestPath, _ := snapshot.RequestPath("fixture-barrier")
	outcome := mutation.Outcome{RequestID: "fixture-barrier", Outcome: mutation.OutcomeCompleted, ReceiptSeq: &seq, Codes: []string{}}
	request := wire.EncodeFile(obj("requestId", str("fixture-barrier"), "seq", str(string(seq)), "mutationSha256", str(string(wire.Sum([]byte("fixture barrier")))), "outcome", outcome.Value()))
	rc := fixture.ReceiptValue(seq.Uint64(), head.LastReceiptSha256, "PAUSE", head.Generation.Uint64())
	rc.Obj.Set("requestId", str("fixture-barrier"))
	var pre, post []wire.Value
	for _, item := range []struct {
		path string
		raw  []byte
	}{{"barrier.json", barrier}, {requestPath, request}} {
		v, err := wire.Parse(item.raw)
		if err != nil {
			t.Fatal(err)
		}
		pre = append(pre, obj("path", str(item.path), "sha256", wire.Null()))
		post = append(post, obj("path", str(item.path), "sha256", str(string(wire.Sum(item.raw))), "record", v, "blobSha256", wire.Null()))
		fixture.Write(t, filepath.Join(repo.StateDir, item.path), item.raw)
	}
	rc.Obj.Set("pre", wire.Array(pre...))
	rc.Obj.Set("post", wire.Array(post...))
	raw = wire.EncodeFile(rc)
	name, _ := snapshot.ReceiptName(seq.Uint64())
	fixture.Write(t, filepath.Join(repo.StateDir, "receipts", name), raw)
	digest := wire.Sum(raw)
	head.LastSeq = seq
	head.LastReceiptSha256 = &digest
	fixture.Write(t, filepath.Join(repo.StateDir, "head.json"), wire.EncodeFile(head.Value()))
}

func TestTMV0016_AS27_ReconciliationUnderBarriersAndNoChange(t *testing.T) {
	for _, scope := range []string{"ALL", "ADMISSION"} {
		for _, choice := range []string{transaction.KeepJournal, transaction.AdoptFile} {
			t.Run(scope+"/"+choice, func(t *testing.T) {
				repo, request, path, canonical := reconcileFixture(t)
				reconciliationBarrier(t, repo, scope)
				if choice == transaction.AdoptFile {
					v, err := wire.Parse(canonical)
					if err != nil {
						t.Fatal(err)
					}
					v.Obj.Set("title", str("adopt through barrier"))
					request.Choice = choice
					request.CanonicalSha256 = ""
					request.File = wire.EncodeFile(v)
					fixture.Write(t, path, request.File)
				}
				report := reconcile(t, repo, request)
				if report.Receipt == "" {
					t.Fatalf("%s refused: %+v", scope, report)
				}
				q, _ := wire.ParseQueueID("", fixture.QueueID)
				r := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}
				if _, err := r.Audit(); err != nil {
					t.Fatalf("barrier journal: %v", err)
				}
			})
		}
	}
	repo, request, path, canonical := reconcileFixture(t)
	request.File = canonical
	fixture.Write(t, path, canonical)
	before := storeDigest(t, repo)
	if report := reconcile(t, repo, request); report.Kind != "NoChange" || report.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("C/C wrote: %+v", report)
	}
}
