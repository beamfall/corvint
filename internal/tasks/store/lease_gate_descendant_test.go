package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// descendantCase builds two attempts of two tickets: seal stands at the tree
// of commit S (src/a.go) and later at the tree of commit C (lib/a.go). With
// sideSeal, S is on branch side and C on main from the empty base; with
// sideLater, C is on branch side from S and main stays at S; otherwise C
// follows S on main. later passes the reusable gate at C's tree, and the
// checkout is left at S for the seal's gate run.
func descendantCase(t *testing.T, sideSeal, sideLater bool) (*leaseStore, *store.Report, *store.Report, string) {
	t.Helper()
	return descendantCaseWith(t, reuseGate("verify", "0", "30", true, true), sideSeal, sideLater)
}

// descendantCaseWith is descendantCase under the given "verify" gate.
func descendantCaseWith(t *testing.T, gate wire.Value, sideSeal, sideLater bool) (*leaseStore, *store.Report, *store.Report, string) {
	t.Helper()
	s := newLeaseStore(t, gate)
	store.SetDescentRootForTest(s.repo, s.root)
	sealTicket, laterTicket := s.ticket(t, "src"), s.ticket(t, "lib")
	seal := s.claim(t, "claim-seal", sealTicket, 0, "src/")
	if sideSeal {
		gitRun(t, s.root, "checkout", "-q", "-b", "side")
	}
	sCommit, sTree := s.commit(t, "src/a.go")
	if r := s.lease(t, "submit-seal", submitOf(seal, sTree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit seal: %+v", r)
	}
	switch {
	case sideSeal:
		gitRun(t, s.root, "checkout", "-q", "main")
	case sideLater:
		gitRun(t, s.root, "checkout", "-q", "-b", "side")
	}
	later := s.claim(t, "claim-later", laterTicket, 1, "lib/")
	_, cTree := s.commit(t, "lib/a.go")
	if r := s.lease(t, "submit-later", submitOf(later, cTree), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit later: %+v", r)
	}
	s.passes(t, "gate-later", gateOf(later, "verify"), 2)
	gitRun(t, s.root, "checkout", "-q", "--detach", sCommit)
	return s, seal, later, sCommit
}

// TestTCDV0151_DescendantPassCoversAMergedSeal: a reusable gate's PASS at a
// commit on the protected intent branch covers an attempt whose candidate
// tree is carried by an ancestor of that commit, without executing; complete
// accepts the reused result, and the receipt audit verifies the ancestry
// from git, refusing the history once the branch no longer carries it.
func TestTCDV0151_DescendantPassCoversAMergedSeal(t *testing.T) {
	t.Parallel()
	s, seal, later, sCommit := descendantCase(t, false, false)
	source := s.resultOf(t, later.AttemptID, "verify")
	report, err := s.gate(t, "gate-seal", gateOf(seal, "verify"), 3)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.Unretryable {
		t.Fatalf("descendant reuse: %+v %v", report, err)
	}
	if s.runs(t, "verify") != 1 {
		t.Fatalf("the covered gate executed: %d runs", s.runs(t, "verify"))
	}
	if r := report.GateReusedFrom; r == nil || string(r.Result) != source || r.AttemptID != later.AttemptID {
		t.Fatalf("report names %+v, want %s of %s", r, source, later.AttemptID)
	}
	done := s.lease(t, "complete-seal", completeOf(seal, sCommit), 4, nil)
	if done.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete: %+v", done)
	}
	auditOK(t, s.repo)
	last := done.Outcome.ReceiptSeq.Uint64()
	if err := store.FoldReceiptBindings(s.repo, last, nil); err != nil {
		t.Fatalf("binding fold: %v", err)
	}
	// The protected branch is rewound behind the covering commit: the audit
	// can no longer observe the ancestry and refuses the reuse.
	gitRun(t, s.root, "branch", "-f", "main", sCommit)
	err = store.FoldReceiptBindings(s.repo, last, nil)
	if err == nil || !strings.Contains(err.Error(), "not a commit on the protected intent branch") {
		t.Fatalf("binding fold after rewind: %v", err)
	}
}

// TestTCDV0152_DescendantReuseRefusals: a PASS whose commit is not on the
// protected branch, or whose commit does not descend from the candidate's,
// is never chosen (the gate executes), and the model refuses the same reuse
// handed to it directly with STALE_TREE without writing.
func TestTCDV0152_DescendantReuseRefusals(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name                string
		sideSeal, sideLater bool
		detail              string
	}{
		{"not-on-protected-branch", false, true, "is not a commit on the protected intent branch"},
		{"not-an-ancestor", true, false, "is not an ancestor of the reused result's tree"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s, seal, later, _ := descendantCase(t, c.sideSeal, c.sideLater)
			from := s.resultOf(t, later.AttemptID, "verify")
			source := s.evidence(t, from)
			src, err := snapshot.DecodeGateResult(source)
			if err != nil {
				t.Fatal(err)
			}
			g := transaction.ReusedGateResult(src, wire.Digest(from), s.attempt(t, seal.AttemptID))
			record, err := g.Encode()
			if err != nil {
				t.Fatal(err)
			}
			before := storeDigest(t, s.repo)
			choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "forge-" + c.name, Root: s.root, Lease: gateOf(seal, "verify")}
			report, err := store.GateReuseForTest(context.Background(), s.repo, operator(), choice, record, source, s.at(t, 3))
			if err != nil {
				t.Fatal(err)
			}
			refusedWith(t, report, mutation.OutcomeBlocked, wire.CodeStaleTree)
			if !strings.Contains(fmt.Sprintf("%+v", report), c.detail) {
				t.Fatalf("refusal does not say %q: %+v", c.detail, report)
			}
			if storeDigest(t, s.repo) != before {
				t.Fatal("a refused reuse wrote")
			}
			ran, err := s.gate(t, "gate-seal", gateOf(seal, "verify"), 4)
			if err != nil || ran.Outcome.Outcome != mutation.OutcomeCompleted || ran.GateReusedFrom != nil || s.runs(t, "verify") != 2 {
				t.Fatalf("gate run: %+v %v, runs %d", ran, err, s.runs(t, "verify"))
			}
		})
	}
}

// TestTCDV0806_StackedLowerAttemptCompletesVerifiedFromDescendantPass
// (V1-3548): a joint gate passes only once both stacked changes are in the
// tree. The lower attempt's own run fails and names the upper attempt's PASS
// as awaiting integration; once the stack is integrated, a rerun reuses that
// PASS without executing, and complete records a VERIFIED completion whose
// evidence names the descendant result.
func TestTCDV0806_StackedLowerAttemptCompletesVerifiedFromDescendantPass(t *testing.T) {
	t.Parallel()
	joint := reuseGate("verify", "0", "30", true, true)
	argv, _ := joint.Obj.Get("argv")
	argv.Arr[2] = wire.String("printf x >> .git/runs-verify; test -e lib/a.go")
	s, seal, later, sCommit := descendantCaseWith(t, joint, false, true)
	source := s.resultOf(t, later.AttemptID, "verify")
	alone, err := s.gate(t, "gate-seal", gateOf(seal, "verify"), 3)
	if err != nil || alone.Outcome.Outcome != mutation.OutcomeCompleted || alone.GateReusedFrom != nil || s.runs(t, "verify") != 2 {
		t.Fatalf("lower attempt alone: %+v %v, runs %d", alone, err, s.runs(t, "verify"))
	}
	if g := s.results(t, seal.AttemptID)["verify"]; g == nil || g.State != "FAILED" {
		t.Fatalf("the joint gate passed the lower tree alone: %+v", g)
	}
	p := alone.GateReusePending
	if p == nil || string(p.Result) != source || p.AttemptID != later.AttemptID || p.Gate != "verify" || p.Branch != "main" || p.Tree != *s.attempt(t, later.AttemptID).CandidateTreeOid {
		t.Fatalf("pending reuse %+v, want %s of %s", p, source, later.AttemptID)
	}
	gitRun(t, s.root, "checkout", "-q", "main")
	gitRun(t, s.root, "merge", "-q", "--ff-only", "side")
	gitRun(t, s.root, "checkout", "-q", "--detach", sCommit)
	rerun, err := s.gate(t, "gate-seal-integrated", gateOf(seal, "verify"), 4)
	if err != nil || rerun.Outcome.Outcome != mutation.OutcomeCompleted || rerun.GateReusePending != nil || s.runs(t, "verify") != 2 {
		t.Fatalf("rerun after integration: %+v %v, runs %d", rerun, err, s.runs(t, "verify"))
	}
	if r := rerun.GateReusedFrom; r == nil || string(r.Result) != source || r.AttemptID != later.AttemptID {
		t.Fatalf("rerun reused %+v, want %s", r, source)
	}
	done := s.lease(t, "complete-seal", completeOf(seal, sCommit), 5, nil)
	if done.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete: %+v", done)
	}
	rec := s.record(t, s.attempt(t, seal.AttemptID).TicketID.Raw)
	if rec.Completion == nil || rec.Completion.Kind != "VERIFIED" {
		t.Fatalf("completion: %+v", rec.Completion)
	}
	cited := false
	for _, d := range rec.Completion.Evidence {
		if g, err := snapshot.DecodeGateResult(s.evidence(t, string(d))); err == nil && g.ReusedFrom != nil && string(*g.ReusedFrom) == source {
			cited = true
		}
	}
	if !cited {
		t.Fatalf("the completion evidence does not name the descendant result %s: %+v", source, rec.Completion.Evidence)
	}
	auditOK(t, s.repo)
}
