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

// BarrierRequest identifies a PAUSE or UNPAUSE choice with the existing frozen
// administrative request digest. PAUSE is always ADMISSION/OPERATOR.
type BarrierRequest struct{ QueueID, RequestID, Operation string }

// Barrier applies a settled fixture pause/unpause under the recorded local
// operator premise. Pending receipts and active staging require separate recovery.
func Barrier(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice BarrierRequest, now wire.Timestamp) (*Report, error) {
	return barrier(ctx, repo, actor, choice, now, nil, nil)
}

func barrier(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice BarrierRequest, now wire.Timestamp, beforeCommit, beforeDelete func() error) (*Report, error) {
	report := &Report{}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	if choice.Operation != transaction.Pause && choice.Operation != transaction.Unpause {
		return report, wire.Errorf(wire.CodeMalformed, "operation", "choose PAUSE or UNPAUSE")
	}
	request := transaction.Request{Operation: choice.Operation, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor}
	if actor.Role != "OWNER" && actor.Role != "OPERATOR" {
		result := transaction.Model(request, transaction.Input{})
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil
	}
	if _, err := transaction.Digest(request); err != nil {
		return report, err
	}
	if _, err := os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, wire.Errorf(wire.CodeUninitialized, "state", "no initialized store")
		}
		return report, err
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return report, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return report, err
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return report, err
	}
	defer session.Close()
	head, err := writerGuards(repo, request.Operation)
	if err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	if head.QueueID.Raw != request.QueueID {
		return report, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	reader := journalReader(repo, head)
	audit := reader.Audit
	if request.Operation == transaction.Unpause {
		audit = reader.BarrierRemoval
	}
	proof, replay, err := writerView(repo, head, request.RequestID, request.Operation == transaction.Unpause)
	if err != nil {
		return report, err
	}
	if replay != nil {
		result := replayResult(request, replay.entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil
	}
	if proof.staging {
		return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	headRaw, currentBarrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	if wire.Sum(headRaw) != proof.identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(request, transaction.Input{Inventory: proof.inv, Head: headRaw, Queue: proof.raw("intent/queue.json"), Policy: proof.raw("intent/policy.json"), Barrier: currentBarrier, Reservations: reservations, CanonicalTickets: proof.tickets, CanonicalReleases: proof.releases, Premise: transaction.LocalOperator, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	if err = proof.recheck(repo, head, audit); err != nil {
		return report, err
	}
	if _, err := writerGuards(repo, request.Operation); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	report.Receipt, err = applyWithFaults(repo, session, result.Plan, beforeCommit, beforeDelete)
	if err == nil {
		proof.commit(repo, result.Plan)
	}
	return report, err
}
