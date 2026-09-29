package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func releaseRequest(id, op, revision, title string) []byte {
	rev := wire.Null()
	if revision != "" {
		rev = str(revision)
	}
	payload := obj("version", str("1"), "title", str(title), "ticketIds", wire.Array(), "predecessorReleaseIds", wire.Array(), "requiredGates", wire.Array(), "acceptanceCriteria", wire.Strings([]string{"accepted"}))
	return wire.EncodeFile(obj("profile", str(release.MutationProfile), "requestId", str(id), "actor", obj("id", str("tester"), "role", str("OWNER")), "queueId", str(fixture.QueueID), "releaseId", str("v1"), "expectedRevision", rev, "operation", str(op), "payload", payload, "issuedAt", str(issued)))
}

func applyRelease(t *testing.T, repo *intent.Repository, raw []byte) *store.Report {
	t.Helper()
	r, err := store.Release(context.Background(), repo, operator(), raw, now(t))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v %v", r, err)
	}
	return r
}

// CAL-V0-027: release intent is admitted after the existing qualification switch too.
func TestCALV0027_ReleaseAfterQualifiedCutover(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_ReleaseAfterQualifiedCutover)
}

func testCALV0027_ReleaseAfterQualifiedCutover(t *testing.T) {
	s := leaseStoreOn(t, nonFixture(t, "NATIVE"))
	applyRelease(t, s.repo, releaseRequest("create", release.OpCreate, "", "Before"))
	if r := s.executionCutover(t, operator(), "qualified", suiteRun()); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatal(r)
	}
	before, err := os.ReadFile(filepath.Join(filepath.Join(s.repo.PrimaryWorktree, intent.Dir), "queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	applyRelease(t, s.repo, releaseRequest("update", release.OpUpdate, "1", "After"))
	after, err := os.ReadFile(filepath.Join(filepath.Join(s.repo.PrimaryWorktree, intent.Dir), "queue.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("release changed queue authority", err)
	}
	auditOK(t, s.repo)
}

// CAL-V0-027: the receipt publication boundary decides fresh retry versus exact replay.
func TestCALV0027_ReleaseInterruptionRecovery(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_ReleaseInterruptionRecovery)
}

func testCALV0027_ReleaseInterruptionRecovery(t *testing.T) {
	for _, role := range []string{"RECEIPT", "POST"} {
		t.Run(role, func(t *testing.T) {
			repo := nonFixture(t, "NATIVE")
			applyRelease(t, repo, releaseRequest("create", release.OpCreate, "", "Original"))
			path := filepath.Join(filepath.Join(repo.PrimaryWorktree, intent.Dir), "releases", "v1.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			raw := releaseRequest("update", release.OpUpdate, "1", "After")
			head, count := journalState(t, repo)
			fired := false
			restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
				if !fired && a.Role == role {
					fired = true
					return errInjected
				}
				return nil
			})
			_, err = store.Release(context.Background(), repo, operator(), raw, now(t))
			restore()
			if !fired || !errors.Is(err, errInjected) {
				t.Fatalf("fault: %v", err)
			}
			committed := role == "POST"
			gotHead, gotCount := journalState(t, repo)
			if gotHead != head || gotCount != count+btoi(committed) {
				t.Fatal("wrong receipt boundary")
			}
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatal("projection changed before fault", err)
			}
			if committed {
				request := store.ReconcileRequest{RequestID: "pending-keep", TargetID: "v1", Choice: transaction.KeepJournal, File: original, CanonicalSha256: wire.Sum(original)}
				before := storeDigest(t, repo)
				r, e := store.Reconcile(context.Background(), repo, operator(), request, now(t))
				if wire.CodeOf(e) != wire.CodeRedoPending || r.Redone || storeDigest(t, repo) != before {
					t.Fatalf("pending reconcile: %+v %v", r, e)
				}
			}
			// A killed writer can leave an unassigned slot; read observation must not erase it.
			slot := filepath.Join(repo.StateDir, "staging", "a00")
			fixture.Write(t, slot, []byte("orphan"))
			r := applyRelease(t, repo, raw)
			if r.Redone != committed || r.Outcome.Replayed != committed {
				t.Fatalf("recovery: %+v", r)
			}
			if _, err := os.Stat(slot); !os.IsNotExist(err) {
				t.Fatalf("orphan not recovered: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			record, err := release.Decode(after)
			if err != nil || record.Revision != "2" || record.Title != "After" {
				t.Fatalf("afterimage: %+v %v", record, err)
			}
			before := storeDigest(t, repo)
			if r := applyRelease(t, repo, raw); !r.Outcome.Replayed {
				t.Fatal("same request did not replay")
			}
			_, err = store.Release(context.Background(), repo, operator(), releaseRequest("update", release.OpUpdate, "1", "Conflict"), now(t))
			if wire.CodeOf(err) != wire.CodeRequestIDConflict || storeDigest(t, repo) != before {
				t.Fatalf("conflict: %v", err)
			}
			_, gotCount = journalState(t, repo)
			if gotCount != count+1 {
				t.Fatal("retry duplicated receipt")
			}
			auditOK(t, repo)
		})
	}
}

// CAL-V0-027: reconciliation retains the settled-state boundary and original bytes.
func TestCALV0027_ReleaseActiveStageAndReconciliation(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_ReleaseActiveStageAndReconciliation)
}

func testCALV0027_ReleaseActiveStageAndReconciliation(t *testing.T) {
	repo := nonFixture(t, "NATIVE")
	applyRelease(t, repo, releaseRequest("create", release.OpCreate, "", "Original"))
	path := filepath.Join(filepath.Join(repo.PrimaryWorktree, intent.Dir), "releases", "v1.json")
	canonical, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	emptyActiveDescriptor(t, repo)
	stage := filepath.Join(repo.StateDir, "staging", "active.json")
	raw, err := os.ReadFile(stage)
	if err != nil {
		t.Fatal(err)
	}
	discarded := []byte("untrusted manual edit\n")
	fixture.Write(t, path, discarded)
	request := store.ReconcileRequest{RequestID: "keep", TargetID: "v1", Choice: transaction.KeepJournal, File: discarded, CanonicalSha256: wire.Sum(canonical)}
	before := storeDigest(t, repo)
	r, err := store.Reconcile(context.Background(), repo, operator(), request, now(t))
	if wire.CodeOf(err) != wire.CodeUnsupported || r.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("active reconcile: %+v %v", r, err)
	}
	got, err := os.ReadFile(stage)
	if err != nil || !bytes.Equal(raw, got) {
		t.Fatal("active stage changed", err)
	}
	// The test operator removes its descriptor; reconciliation itself has no cleanup authority.
	if err := os.Remove(stage); err != nil {
		t.Fatal(err)
	}
	r, err = store.Reconcile(context.Background(), repo, operator(), request, now(t))
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("settled keep: %+v %v", r, err)
	}
	got, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(got, canonical) {
		t.Fatal("canonical bytes not restored", err)
	}
	got, err = os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(wire.Sum(discarded))))
	if err != nil || !bytes.Equal(got, discarded) {
		t.Fatal("original bytes not retained", err)
	}
	auditOK(t, repo)
}

// CAL-V0-027: the trusted caller binding still outranks the submitted actor.
func TestCALV0027_ReleaseWrongActor(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_ReleaseWrongActor)
}

func testCALV0027_ReleaseWrongActor(t *testing.T) {
	repo := nonFixture(t, "NATIVE")
	before := storeDigest(t, repo)
	r, err := store.Release(context.Background(), repo, mutation.Binding{ID: "other", Role: "OWNER"}, releaseRequest("create", release.OpCreate, "", "Release"), now(t))
	if err == nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatal("actor mismatch accepted")
	}
	if storeDigest(t, repo) != before {
		t.Fatal("actor mismatch mutated store")
	}
}

// observePublishedStage reconstructs the plan descriptor from the writer's actual
// artifacts and leaves it beside its completed head, as a final-cleanup interruption.
func observePublishedStage(t *testing.T, repo *intent.Repository, operation, kind string, run func()) {
	t.Helper()
	beforeHead, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := snapshot.DecodeHead(beforeHead)
	if err != nil {
		t.Fatal(err)
	}
	var artifacts []transaction.Artifact
	restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error { artifacts = append(artifacts, a); return nil })
	func() { defer restore(); run() }()
	var receipt *snapshot.Receipt
	var request *snapshot.Request
	for _, a := range artifacts {
		if a.Role == "RECEIPT" {
			receipt, err = snapshot.DecodeReceipt(a.Data)
			if err != nil {
				t.Fatal(err)
			}
		}
		if strings.HasPrefix(a.Target, "requests/") {
			request, err = snapshot.DecodeRequest(a.Data)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if receipt == nil || request == nil || receipt.Kind != kind {
		t.Fatalf("missing actual artifacts or kind: %+v %+v", receipt, request)
	}
	d := snapshot.StageDescriptor{QueueID: fixture.QueueID, Operation: operation, RequestID: request.Entry.RequestID, RequestSha256: request.Entry.MutationSha256, RecordedAt: receipt.RecordedAt, Base: &snapshot.StageBase{LastSeq: base.LastSeq, LastReceiptSha256: *base.LastReceiptSha256}}
	for _, a := range artifacts {
		d.Artifacts = append(d.Artifacts, a.Description)
	}
	sort.Slice(d.Artifacts, func(i, j int) bool { return d.Artifacts[i].Slot < d.Artifacts[j].Slot })
	raw, err := d.Encode()
	if err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(repo.StateDir, "staging", "active.json"), raw)
	before := storeDigest(t, repo)
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	reader := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}
	proof, err := reader.Audit()
	if err != nil || !proof.StagingPresent {
		t.Fatalf("completed %s/%s: %+v %v", operation, kind, proof, err)
	}
	if storeDigest(t, repo) != before {
		t.Fatal("completed observation wrote")
	}
}

// CAL-V0-027: real writers, including gate/manifest and recorded refusal, bind completed stages.
func TestCALV0027_ActualCompletedStages(t *testing.T) {
	t.Run("CAL-V0-027 witness", testCALV0027_ActualCompletedStages)
}

func testCALV0027_ActualCompletedStages(t *testing.T) {
	t.Run("MUTATION", func(t *testing.T) {
		repo := nonFixture(t, "NATIVE")
		observePublishedStage(t, repo, snapshot.StageMutate, "MUTATION", func() {
			if r := mutate(t, repo, envelope("actual-create", mutation.OpCreate, "", "", createPayload("actual"))); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatal(r)
			}
		})
	})
	for _, v := range leaseVerbs {
		kind := map[string]string{"claim": "ADMIT", "renew": "TRANSITION", "gate-run": "GATE_RESULT", "complete": "MANIFEST"}[v.name]
		if kind == "" {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			s := leaseStoreOn(t, nonFixture(t, "NATIVE"), commandGate("verify", "printf ok", "30", true))
			if r := s.executionCutover(t, operator(), "qualified", suiteRun()); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatal(r)
			}
			run := v.setup(t, s)
			observePublishedStage(t, s.repo, snapshot.StageLease, kind, func() {
				r, err := run("actual")
				if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
					t.Fatalf("writer: %+v %v", r, err)
				}
			})
		})
	}
	t.Run("FENCED", func(t *testing.T) {
		s := leaseStoreOn(t, nonFixture(t, "NATIVE"))
		if r := s.executionCutover(t, operator(), "qualified", suiteRun()); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatal(r)
		}
		c := s.claim(t, "claim", s.ticket(t, "one"), 0, "src/")
		s.lease(t, "release", releaseOf(c), 1, nil)
		observePublishedStage(t, s.repo, snapshot.StageLease, "TRANSITION", func() {
			refusedWith(t, s.lease(t, "stale", renewOf(c), 2, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
		})
	})
}
