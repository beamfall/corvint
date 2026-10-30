package store

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// GateRunCI satisfies one policy command gate from a retained green CI run
// instead of executing it (TCD-V0-155). ciRaw is the operator's
// taskman-ci-run/0 record; it is decoded and re-encoded canonically, and
// nothing is fetched: the run's head commit must already be in the local
// object store. The CI-backed result is built here and the model rechecks
// the policy's ciEvidence entry, the run's conclusion and checks, the bytes
// and the head commit's ancestry on the protected intent branch under the
// lock. A stale or unsubmitted attempt is answered by the model with no
// gate facts, as GateRun does.
func GateRunCI(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, ciRaw []byte, clock func() time.Time) (*Report, error) {
	ci, err := snapshot.DecodeCIRun(ciRaw)
	if err != nil {
		return &Report{}, err
	}
	raw, err := ci.Encode()
	if err != nil {
		return &Report{}, err
	}
	redone, err := settleLease(ctx, repo)
	if err != nil {
		return &Report{}, err
	}
	a, policy, _, err := unlockedAttemptProof(ctx, repo, choice.Lease.AttemptID)
	if err != nil {
		return &Report{}, err
	}
	if runnable(a, choice.Lease, clock()) {
		if _, err := commandGate(policy, choice.Lease.Gate); err != nil {
			return &Report{}, err
		}
		tree, err := resolveObject(leaseRoot(repo, choice), ci.HeadSha, "tree")
		if err != nil {
			return &Report{}, wire.Errorf(wire.CodeStaleTree, "ciRun", "the CI run's head commit %s is not in the local repository; fetch it first", ci.HeadSha)
		}
		g := transaction.CIGateResult(policy, choice.Lease.Gate, ci, raw, a, tree)
		record, err := g.Encode()
		if err != nil {
			return &Report{}, err
		}
		choice.gate = &gateRun{record: record, source: raw}
	}
	now, err := wire.ParseTimestamp("recordedAt", clock().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return &Report{}, err
	}
	report, err := Lease(ctx, repo, actor, choice, now)
	if report != nil {
		report.Redone = report.Redone || redone
		switch {
		case err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted:
		case report.Kind == "Transaction" && choice.gate != nil:
			report.GateReusedFrom = gateReuseOf(choice.gate.source)
		case report.Kind == "Replay" && report.Outcome.ReceiptSeq != nil:
			report.GateReusedFrom = recordedReuse(repo, report.Outcome.ReceiptSeq.Uint64(), choice.Lease.AttemptID, choice.Lease.Gate)
		}
	}
	return report, err
}

// ciRunFacts are a CI-backed gate run's facts under the lock: the head
// commit's tree and its commit-level ancestry verdict, re-read from git now
// (TCD-V0-155). An unresolvable head commit yields no tree, which the model
// refuses.
func ciRunFacts(root string, proof *journal.Result, a *snapshot.Attempt, ci *snapshot.CIRun, run *gateRun, results map[wire.Digest][]byte) (transaction.LeaseFacts, error) {
	tree, err := resolveObject(root, ci.HeadSha, "tree")
	if err != nil || a == nil || a.CandidateTreeOid == nil {
		return transaction.GateCIFacts(run.record, run.source, "", "", results), nil
	}
	h, err := loadProtectedHistory(root, intentBranch(proof))
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	why, err := h.commitDescent(*a.CandidateTreeOid, ci.HeadSha, tree)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	return transaction.GateCIFacts(run.record, run.source, tree, why, results), nil
}
