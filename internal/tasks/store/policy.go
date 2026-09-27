package store

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PolicyRequest is one OWNER/OPERATOR policy replacement (ATM-V0-027,
// TM-V0-030). Policy is the complete new canonical policy file.
type PolicyRequest struct {
	QueueID, RequestID    string
	ExpectedPolicyVersion wire.Size
	Policy                []byte
}

// PolicyUpdate commits the next policy version through the §5.2 writer. The
// receipt's pre/post entries for intent/policy.json carry the old and new
// policy digests.
func PolicyUpdate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice PolicyRequest, now wire.Timestamp) (*Report, error) {
	return policyUpdate(ctx, repo, actor, choice, now, nil)
}

func policyUpdate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice PolicyRequest, now wire.Timestamp, beforeCommit func() error) (*Report, error) {
	request := transaction.Request{Operation: transaction.PolicyUpdate, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, Policy: choice.Policy, ExpectedPolicyVersion: choice.ExpectedPolicyVersion}
	report, proof, err := administrativeWrite(ctx, repo, request, now, beforeCommit)
	if err != nil || proof == nil {
		return report, err
	}
	report.OldPolicySha256 = wire.Sum(proof.raw("intent/policy.json"))
	report.NewPolicySha256 = wire.Sum(choice.Policy)
	return report, nil
}

// administrativeWrite commits one whole-store administrative request through
// the §5.2 writer: guards, redo, request-index replay, a fresh audit of every
// intent record, the model, the branch and observation checks, and apply. It
// returns the audit a committed request was planned against.
func administrativeWrite(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error) (*Report, *auditView, error) {
	report := &Report{}
	if repo == nil {
		return report, nil, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	actor := request.Actor
	if actor.Role != "OWNER" && actor.Role != "OPERATOR" {
		result := transaction.Model(request, transaction.Input{})
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil, nil
	}
	if _, err := transaction.Digest(request); err != nil {
		return report, nil, err
	}
	if _, err := os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, nil, wire.Errorf(wire.CodeUninitialized, "state", "no initialized store")
		}
		return report, nil, err
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return report, nil, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return report, nil, err
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return report, nil, err
	}
	defer session.Close()
	head, err := writerGuards(repo, request.Operation)
	if err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	if head.QueueID.Raw != request.QueueID {
		return report, nil, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	if report.Redone, err = redoPending(repo, session); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	proof, replay, err := writerView(repo, head, request.RequestID, false)
	if err != nil {
		return report, nil, err
	}
	if replay != nil {
		result := replayResult(request, replay.entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil, nil
	}
	if proof.staging {
		return report, nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	q, err := intent.DecodeQueue(proof.raw("intent/queue.json"))
	if err != nil {
		return report, nil, err
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, nil, err
	}
	if wire.Sum(headRaw) != proof.identity.HeadSha256 {
		return report, nil, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(request, transaction.Input{Inventory: proof.inv, Head: headRaw, Queue: proof.raw("intent/queue.json"), Policy: proof.raw("intent/policy.json"), Barrier: barrier, Reservations: reservations, CanonicalTickets: proof.tickets, CanonicalReleases: proof.releases, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil, nil
	}
	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	if err = bindObservation(repo, proof.identity, request.Operation); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	report.Receipt, err = applyBeforeCommit(repo, session, result.Plan, beforeCommit)
	if err != nil {
		return report, nil, err
	}
	proof.commit(repo, result.Plan)
	return report, proof, nil
}

func guardFailureAudit(report *Report, requestID string, err error) (*Report, *auditView, error) {
	report, err = guardFailure(report, requestID, err)
	return report, nil, err
}
