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
func PolicyUpdate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice PolicyRequest, now wire.Timestamp) (out *Report, outErr error) {
	defer func() { out, outErr = intentFix(repo, out, outErr) }()
	return policyUpdate(ctx, repo, actor, choice, now, nil)
}

func policyUpdate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice PolicyRequest, now wire.Timestamp, beforeCommit func() error) (*Report, error) {
	request := transaction.Request{Operation: transaction.PolicyUpdate, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, Policy: choice.Policy, ExpectedPolicyVersion: choice.ExpectedPolicyVersion}
	report, proof, err := administrativeWrite(ctx, repo, request, now, beforeCommit)
	if err != nil || proof == nil {
		return report, err
	}
	report.OldPolicySha256 = wire.Sum(proof.Records["intent/policy.json"].Raw)
	report.NewPolicySha256 = wire.Sum(choice.Policy)
	return report, nil
}

// ImportMapRequest is one OWNER amendment of a mapped queue's import map
// (CTS-V0-020): ImportMap is the complete new canonical map and
// ExpectedImportMapSha256 the journaled map the owner reviewed.
type ImportMapRequest struct {
	QueueID, RequestID      string
	ExpectedImportMapSha256 wire.Digest
	ImportMap               []byte
}

// ImportMapAmend commits an amended import map and the queue naming it under
// one IMPORT_PLAN receipt through the §5.2 writer (CTS-V0-020, CTS-V0-021).
func ImportMapAmend(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice ImportMapRequest, now wire.Timestamp) (out *Report, outErr error) {
	defer func() { out, outErr = intentFix(repo, out, outErr) }()
	request := transaction.Request{Operation: transaction.ImportPlan, QueueID: choice.QueueID, RequestID: choice.RequestID, Actor: actor, ImportMap: choice.ImportMap, ExpectedImportMapSha256: choice.ExpectedImportMapSha256}
	report, _, err := administrativeWrite(ctx, repo, request, now, nil)
	return report, err
}

// administrativeWrite commits one whole-store administrative request through
// the §5.2 writer: guards, redo, request-index replay, a fresh audit of every
// intent record, the model, the branch and observation checks, and apply. It
// returns the audit a committed request was planned against.
func administrativeWrite(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error) (*Report, *journal.Result, error) {
	return administrativeWriteWith(ctx, repo, request, now, beforeCommit, nil)
}

// administrativeWriteWith is administrativeWrite for a Lease request: it
// also audits every attempt record, hands the model the head receipt, and
// takes the claim facts observed from the audit under the same lock. A
// write whose context ended before it committed reports the retryable
// LOCK_TIMEOUT, never MALFORMED (ATR-V0-050).
func administrativeWriteWith(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error, facts claimObserver) (*Report, *journal.Result, error) {
	report, proof, err := administrativeWriteRound(ctx, repo, request, now, beforeCommit, facts)
	return report, proof, wire.ContextEnded(err, "store write")
}

func administrativeWriteRound(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error, facts claimObserver) (*Report, *journal.Result, error) {
	if request.Operation == transaction.Lease {
		return leaseWrite(ctx, repo, request, now, beforeCommit, facts)
	}
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
	if _, err := authority.QualifyReused(repo.CommonDir); err != nil {
		return report, nil, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return report, nil, err
	}
	defer lock.Close()
	now = recordedAt(ctx, now)
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return report, nil, err
	}
	defer session.Close()
	// id is the repository identity the journal is read and guarded under:
	// the resolved repository, except that a RELOCATE reads the store under
	// the primary worktree it names explicitly (TRL-V0-001).
	id := repo
	if request.Operation == transaction.Relocate {
		if id, err = relocationIdentity(repo, request.FromPrimaryWorktree); err != nil {
			return report, nil, err
		}
	}
	head, err := writerGuards(id, guardOperation(request))
	if err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	if head.QueueID.Raw != request.QueueID {
		return report, nil, wire.Errorf(wire.CodeOutOfScope, "queueId", "request queue differs")
	}
	if report.Redone, err = redoPending(id, session); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	if request.Operation == transaction.Relocate && report.Redone {
		// A redone RELOCATE re-pinned the head (TRL-V0-004).
		if id, err = relocationIdentity(repo, request.FromPrimaryWorktree); err != nil {
			return report, nil, err
		}
	}
	reader := lockedJournalReader(id, head)
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(request.RequestID)
	if err != nil {
		return report, nil, err
	}
	if found {
		result := replayResult(request, entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil, nil
	}
	inv, err := inventory(repo)
	if err != nil {
		return report, nil, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if file.Path == "programs.json" || file.Path == "pools.json" || strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") || (request.Operation == transaction.Lease && strings.HasPrefix(file.Path, "attempts/")) {
			paths = append(paths, file.Path)
		}
	}
	if request.Operation == transaction.ImportPlan {
		// The amendment is checked against the journaled map (CTS-V0-020).
		paths = append(paths, "intent/import-map.json")
	}
	proof, err := reader.Audit(paths...)
	if err != nil {
		return report, nil, err
	}
	if proof.StagingPresent {
		return report, nil, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	tickets := make([][]byte, 0, len(paths)-2)
	releases := [][]byte{}
	attempts := [][]byte{}
	for _, path := range paths[2:] {
		if path == "pools.json" || path == "programs.json" || path == "intent/import-map.json" {
			continue
		}
		if strings.HasPrefix(path, "attempts/") {
			attempts = append(attempts, proof.Records[path].Raw)
		} else if strings.HasPrefix(path, "intent/releases/") {
			releases = append(releases, proof.Records[path].Raw)
		} else {
			tickets = append(tickets, proof.Records[path].Raw)
		}
	}
	q, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
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
	if wire.Sum(headRaw) != proof.Identity.HeadSha256 {
		return report, nil, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	headRc, err := headReceipt(repo, headRaw)
	if err != nil {
		return report, nil, err
	}
	input := transaction.Input{Inventory: inv, Head: headRaw, HeadReceipt: headRc, Queue: proof.Records["intent/queue.json"].Raw, Policy: proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, Pools: proof.Records["pools.json"].Raw, Programs: proof.Records["programs.json"].Raw, CanonicalTickets: tickets, CanonicalReleases: releases, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now}
	if request.Operation == transaction.Lease {
		if err = leaseInput(proof, attempts, facts, &input); err != nil {
			return report, nil, err
		}
	}
	if request.Operation == transaction.ImportPlan {
		input.ImportMap = proof.Records["intent/import-map.json"].Raw
	}
	result := transaction.Model(request, input)
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	report.AttemptID, report.Generation, report.Expired = result.AttemptID, result.Generation, result.Expired
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil, nil
	}
	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	if err = bindObservation(id, proof.Identity, guardOperation(request)); err != nil {
		return guardFailureAudit(report, request.RequestID, err)
	}
	report.Receipt, err = applyBeforeCommit(repo, session, result.Plan, beforeCommit)
	if err != nil {
		return report, nil, err
	}
	return report, proof, nil
}

func guardFailureAudit(report *Report, requestID string, err error) (*Report, *journal.Result, error) {
	report, err = guardFailure(report, requestID, err)
	return report, nil, err
}
