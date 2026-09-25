package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
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
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(request.RequestID)
	if err != nil {
		return report, err
	}
	if found {
		result := replayResult(request, entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil
	}
	inv, err := inventory(repo)
	if err != nil {
		return report, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	audit := reader.Audit
	if request.Operation == transaction.Unpause {
		audit = reader.BarrierRemoval
	}
	proof, err := audit(paths...)
	if err != nil {
		return report, err
	}
	if proof.StagingPresent {
		return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	tickets := make([][]byte, 0, len(paths)-2)
	releases := [][]byte{}
	for _, path := range paths[2:] {
		if strings.HasPrefix(path, "intent/releases/") {
			releases = append(releases, proof.Records[path].Raw)
		} else {
			tickets = append(tickets, proof.Records[path].Raw)
		}
	}
	headRaw, currentBarrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	if wire.Sum(headRaw) != proof.Identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(request, transaction.Input{Inventory: inv, Head: headRaw, Queue: proof.Records["intent/queue.json"].Raw, Policy: proof.Records["intent/policy.json"].Raw, Barrier: currentBarrier, Reservations: reservations, CanonicalTickets: tickets, CanonicalReleases: releases, Premise: transaction.LocalOperator, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	current, err := audit()
	if err != nil {
		return report, err
	}
	if current.StagingPresent || current.Identity != proof.Identity {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "barrier", "validated snapshot changed")
	}
	if _, err := writerGuards(repo, request.Operation); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	report.Receipt, err = applyWithFaults(repo, session, result.Plan, beforeCommit, beforeDelete)
	return report, err
}
