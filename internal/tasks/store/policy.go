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
	report := &Report{}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	request := transaction.Request{Operation: transaction.PolicyUpdate, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, Policy: choice.Policy, ExpectedPolicyVersion: choice.ExpectedPolicyVersion}
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
	if report.Redone, err = redoPending(repo, session); err != nil {
		return guardFailure(report, request.RequestID, err)
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
	proof, err := reader.Audit(paths...)
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
	q, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return report, err
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	if wire.Sum(headRaw) != proof.Identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(request, transaction.Input{Inventory: inv, Head: headRaw, Queue: proof.Records["intent/queue.json"].Raw, Policy: proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, CanonicalTickets: tickets, CanonicalReleases: releases, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	if err = bindObservation(repo, proof.Identity, request.Operation); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	report.Receipt, err = applyBeforeCommit(repo, session, result.Plan, beforeCommit)
	if err != nil {
		return report, err
	}
	report.OldPolicySha256 = wire.Sum(proof.Records["intent/policy.json"].Raw)
	report.NewPolicySha256 = wire.Sum(choice.Policy)
	return report, nil
}
