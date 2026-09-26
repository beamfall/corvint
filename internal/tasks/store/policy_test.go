package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// policyVersion returns the canonical fixture policy at version n.
func policyVersion(n string) []byte {
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", str(n))
	return wire.EncodeFile(v)
}

func policyRequest(id, expected string, raw []byte) store.PolicyRequest {
	return store.PolicyRequest{QueueID: fixture.QueueID, RequestID: id, ExpectedPolicyVersion: wire.Size(expected), Policy: raw}
}

func auditOK(t *testing.T, repo *intent.Repository) {
	t.Helper()
	q, err := wire.ParseQueueID("queueId", fixture.QueueID)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := (journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.PrimaryWorktree}, QueueID: q, PrimaryWorktree: repo.PrimaryWorktree}).Audit()
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
		t.Fatalf("audit: %+v %v", proof, err)
	}
}

func TestTMV0030_AS11_PolicyUpdateCommitsAndReplays(t *testing.T) {
	repo, _ := initialized(t)
	oldRaw := fixture.PolicyBytes()
	newRaw := policyVersion("2")
	report, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", newRaw), now(t))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("update: %+v %v", report, err)
	}
	projected, err := os.ReadFile(filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json"))
	if err != nil || string(projected) != string(newRaw) {
		t.Fatalf("projection not replaced: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil || rc.Kind != "POLICY_UPDATE" {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	var pre, post *wire.Digest
	for _, e := range rc.Pre {
		if e.Path == "intent/policy.json" {
			pre = e.Sha256
		}
	}
	for _, e := range rc.Post {
		if e.Path == "intent/policy.json" {
			post = e.Sha256
		}
	}
	if pre == nil || post == nil || *pre != wire.Sum(oldRaw) || *post != wire.Sum(newRaw) || len(rc.Post) != 2 {
		t.Fatalf("receipt policy digests: pre=%v post=%v posts=%d", pre, post, len(rc.Post))
	}
	auditOK(t, repo)

	before := storeDigest(t, repo)
	replay, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", newRaw), now(t))
	if err != nil || replay.Kind != "Replay" || !replay.Outcome.Replayed || replay.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	conflict, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "2", policyVersion("3")), now(t))
	if err != nil || !conflict.Outcome.HasCode(wire.CodeRequestIDConflict) || storeDigest(t, repo) != before {
		t.Fatalf("conflict: %+v %v", conflict, err)
	}
	// Existing writers keep working against the new policy.
	mutate(t, repo, envelope("after-policy", mutation.OpCreate, "", "", createPayload("after policy")))
	auditOK(t, repo)
}

func TestTMV0030_AS11_PolicyUpdateRefusalsPreserveStore(t *testing.T) {
	cases := []struct {
		name    string
		outcome string
		code    string
	}{
		{"stale", mutation.OutcomeRevisionConflict, ""},
		{"skip", mutation.OutcomeValidationFailed, wire.CodeMalformed},
		{"same", mutation.OutcomeValidationFailed, wire.CodeMalformed},
		{"malformed", "", wire.CodeMalformed},
		{"profile", "", wire.CodeUnsupportedVersion},
		{"noncanonical", "", wire.CodeMalformed},
		{"worker", mutation.OutcomeUnauthorized, ""},
		{"reviewer", mutation.OutcomeUnauthorized, ""},
		{"scope", "", wire.CodeOutOfScope},
		{"all", mutation.OutcomeBlocked, wire.CodePaused},
		{"branch", mutation.OutcomeBlocked, wire.CodeIntentBranchMismatch},
		{"staging", "", wire.CodeUnsupported},
		{"restore", "", wire.CodeRestoreIncomplete},
		{"drift", "", wire.CodeIntentDiverged},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, _ := initialized(t)
			actor := operator()
			request := policyRequest("policy", "1", policyVersion("2"))
			switch c.name {
			case "stale":
				request.ExpectedPolicyVersion = "0"
			case "skip":
				request.Policy = policyVersion("3")
			case "same":
				request.Policy = policyVersion("1")
			case "malformed":
				request.Policy = []byte("{}\n")
			case "profile":
				v := fixture.PolicyValue()
				v.Obj.Set("policyVersion", str("2"))
				v.Obj.Set("profile", str("taskman-policy/1"))
				request.Policy = wire.EncodeFile(v)
			case "noncanonical":
				request.Policy = append([]byte(" "), policyVersion("2")...)
			case "worker":
				actor.Role = "WORKER"
			case "reviewer":
				actor.Role = "REVIEWER"
			case "scope":
				request.QueueID = "queue:other:main"
			case "all":
				reconciliationBarrier(t, repo, "ALL")
			case "branch":
				fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/elsewhere\n"))
			case "staging":
				emptyActiveDescriptor(t, repo)
			case "restore":
				fixture.Write(t, filepath.Join(repo.StateDir, "RESTORE_INCOMPLETE"), []byte("incomplete"))
			case "drift":
				fixture.Write(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json"), policyVersion("7"))
			}
			before := storeDigest(t, repo)
			report, err := store.PolicyUpdate(context.Background(), repo, actor, request, now(t))
			if err == nil && report.Outcome.Outcome == mutation.OutcomeCompleted {
				t.Fatalf("accepted: %+v", report)
			}
			if c.outcome != "" && report.Outcome.Outcome != c.outcome {
				t.Fatalf("outcome %s want %s: %+v %v", report.Outcome.Outcome, c.outcome, report, err)
			}
			if c.code != "" && wire.CodeOf(err) != c.code && !report.Outcome.HasCode(c.code) {
				t.Fatalf("code want %s: %+v %v", c.code, report, err)
			}
			if report.Receipt != "" || storeDigest(t, repo) != before {
				t.Fatal("refusal changed store or intent")
			}
		})
	}
}

func TestTMV0030_AS11_PolicyUpdateRedoesPendingReceiptFirst(t *testing.T) {
	repo := pendingMutation(t)
	report, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t))
	if err != nil || !report.Redone || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("pending: %+v %v", report, err)
	}
	auditOK(t, repo)
}

func TestTMV0030_AS11_PolicyUpdatePublicationReturnedFault(t *testing.T) {
	repo, _ := initialized(t)
	before := storeDigest(t, repo)
	fault := errors.New("returned precommit fault")
	report, err := store.PolicyUpdateBeforeCommitForTest(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t), func() error { return fault })
	if !errors.Is(err, fault) || report.Receipt != "" || storeDigest(t, repo) != before {
		t.Fatalf("precommit fault: %+v %v", report, err)
	}
	entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging: %v %v", entries, err)
	}
	retry, err := store.PolicyUpdate(context.Background(), repo, operator(), policyRequest("policy-2", "1", policyVersion("2")), now(t))
	if err != nil || retry.Receipt == "" {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	auditOK(t, repo)
}

func TestTMV0030_AS11_PolicyUpdateModelRejectsForeignInputs(t *testing.T) {
	r := transaction.Request{Operation: transaction.Pause, QueueID: fixture.QueueID, RequestID: "pause", Actor: operator(), ExpectedPolicyVersion: "1"}
	if _, err := transaction.Digest(r); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("expected version on PAUSE: %v", err)
	}
	r = transaction.Request{Operation: transaction.Pause, QueueID: fixture.QueueID, RequestID: "pause", Actor: operator(), Policy: policyVersion("2")}
	if _, err := transaction.Digest(r); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("policy on PAUSE: %v", err)
	}
}
