package store

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ReconcileRequest pins an operator's original choice. File is a preserved copy
// of the offered/discarded projection, not a request to read it again on retry.
// KEEP_JOURNAL also pins the canonical ticket-record digest, not a receipt hash.
type ReconcileRequest struct {
	RequestID, TargetID, Choice string
	File                        []byte
	CanonicalSha256             wire.Digest
}

// Reconcile applies one settled fixture ticket choice under the local-operator
// premise. Pending receipts and active staging require recovery outside this
// delivery; this entrypoint never skips, deletes or redoes them.
func Reconcile(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice ReconcileRequest, now wire.Timestamp) (*Report, error) {
	return reconcile(ctx, repo, actor, choice, now, nil)
}

func reconcile(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice ReconcileRequest, now wire.Timestamp, beforeCommit func() error) (*Report, error) {
	if _, ticketErr := wire.ParseTicketID("targetId", choice.TargetID); ticketErr != nil {
		return reconcileRelease(ctx, repo, actor, choice, now, beforeCommit)
	}
	report := &Report{}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "repository", "missing repository")
	}
	if choice.Choice != transaction.KeepJournal && choice.Choice != transaction.AdoptFile {
		return report, wire.Errorf(wire.CodeMalformed, "choice", "choose KEEP_JOURNAL or ADOPT_FILE")
	}
	if choice.File == nil {
		return report, wire.Errorf(wire.CodeMalformed, "file", "original file bytes required")
	}
	if len(choice.File) > wire.MaxTicketFileBytes {
		return report, wire.Errorf(wire.CodeLimitExceeded, "file", "original file exceeds ticket bound")
	}
	target, err := wire.ParseTicketID("targetId", choice.TargetID)
	if err != nil {
		return report, err
	}
	request := transaction.Request{Operation: choice.Choice, QueueID: target.QueueID(), RequestID: choice.RequestID, TargetID: choice.TargetID, Actor: actor, File: bytes.Clone(choice.File), CanonicalSha256: choice.CanonicalSha256}
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
	reader := journalReader(repo, head)
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(request.RequestID)
	if err != nil {
		return report, err
	}
	if found {
		result := replayResult(request, entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		if result.Kind == "Replay" {
			report.Ticket = index.TicketID
		}
		return report, nil
	}
	inv, err := inventory(repo)
	if err != nil {
		return report, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, f := range inv.Files() {
		if strings.HasPrefix(f.Path, "intent/tickets/") || strings.HasPrefix(f.Path, "intent/releases/") {
			paths = append(paths, f.Path)
		}
	}
	proof, err := reader.Reconciliation(request.TargetID, paths...)
	if err != nil {
		return report, err
	}
	if proof.StagingPresent {
		return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	queue := proof.Records["intent/queue.json"].Raw
	q, err := intent.DecodeQueue(queue)
	if err != nil {
		return report, err
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return guardFailure(report, request.RequestID, err)
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
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	if wire.Sum(headRaw) != proof.Identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(request, transaction.Input{Inventory: inv, Head: headRaw, Queue: queue, Policy: proof.Records["intent/policy.json"].Raw, Barrier: barrier, Reservations: reservations, CanonicalTickets: tickets, CanonicalReleases: releases, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}
	// This second complete audit supports present-empty D without weakening the
	// ordinary intent reader, and binds private inventory as well as head/intent.
	current, err := reader.Reconciliation(request.TargetID)
	if err != nil {
		return report, err
	}
	if current.StagingPresent || current.Identity != proof.Identity {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "reconcile", "validated snapshot changed")
	}
	if err := requireBranch(repo, q.IntentBranch); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	if _, err := writerGuards(repo, request.Operation); err != nil {
		return guardFailure(report, request.RequestID, err)
	}
	report.Receipt, err = applyBeforeCommit(repo, session, result.Plan, beforeCommit)
	if err != nil {
		return report, err
	}
	receipt, err := snapshot.DecodeReceipt(result.Plan.Receipt())
	if err != nil {
		return report, err
	}
	if receipt.TicketID != nil {
		report.Ticket = receipt.TicketID.Raw
	}
	return report, nil
}

func reconcileRelease(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice ReconcileRequest, now wire.Timestamp, beforeCommit func() error) (*Report, error) {
	report := &Report{}
	if _, err := wire.ParseLabel("releaseId", choice.TargetID); err != nil {
		return report, err
	}
	if choice.Choice == transaction.AdoptFile {
		return report, wire.Errorf(wire.CodeUnsupported, "choice", "release ADOPT_FILE is NOT_RUN")
	}
	if choice.Choice != transaction.KeepJournal || choice.File == nil {
		return report, wire.Errorf(wire.CodeMalformed, "choice", "release reconciliation requires KEEP_JOURNAL and original bytes")
	}
	if len(choice.File) > wire.MaxReleaseFileBytes {
		return report, wire.Errorf(wire.CodeLimitExceeded, "file", "release file bound")
	}
	resolved, err := intent.Load(repo.PrimaryWorktree)
	if err == nil && (!resolved.Queue.Fixture || resolved.Queue.ExecutionCutover != nil) {
		return report, wire.Errorf(wire.CodeUnsupported, "queue", "release reconciliation is fixture-only")
	}
	if _, err = authority.Qualify(repo.CommonDir); err != nil {
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
	head, err := writerGuards(repo, transaction.Release)
	if err != nil {
		return report, err
	}
	reader := journalReader(repo, head)
	request := transaction.Request{Operation: transaction.Release, QueueID: head.QueueID.Raw, RequestID: choice.RequestID, TargetID: choice.TargetID, Actor: actor, File: bytes.Clone(choice.File), CanonicalSha256: choice.CanonicalSha256}
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(choice.RequestID)
	if err != nil {
		return report, err
	}
	if found {
		result := replayResult(request, entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		report.Release = choice.TargetID
		return report, nil
	}
	inv, err := inventory(repo)
	if err != nil {
		return report, err
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, f := range inv.Files() {
		if strings.HasPrefix(f.Path, "intent/tickets/") || strings.HasPrefix(f.Path, "intent/releases/") {
			paths = append(paths, f.Path)
		}
	}
	proof, err := reader.ReconciliationRelease(choice.TargetID, paths...)
	if err != nil {
		return report, err
	}
	canonical := proof.Records["intent/releases/"+choice.TargetID+".json"].Raw
	if _, err = release.Decode(canonical); err != nil {
		return report, err
	}
	if wire.Sum(canonical) != choice.CanonicalSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "canonicalSha256", "canonical release changed")
	}
	qraw := proof.Records["intent/queue.json"].Raw
	praw := proof.Records["intent/policy.json"].Raw
	q, err := intent.DecodeQueue(qraw)
	if err != nil {
		return report, err
	}
	if !q.Fixture || q.ExecutionCutover != nil {
		return report, wire.Errorf(wire.CodeUnsupported, "queue", "release reconciliation is fixture-only")
	}
	ticketRaws, releaseRaws := [][]byte{}, [][]byte{}
	for _, path := range paths[2:] {
		if strings.HasPrefix(path, "intent/tickets/") {
			ticketRaws = append(ticketRaws, proof.Records[path].Raw)
		} else {
			releaseRaws = append(releaseRaws, proof.Records[path].Raw)
		}
	}
	headRaw, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return report, err
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return report, err
	}
	modeled := transaction.Model(request, transaction.Input{Inventory: inv, Head: headRaw, Queue: qraw, Policy: praw, Barrier: barrier, Reservations: reservations, CanonicalTickets: ticketRaws, CanonicalReleases: releaseRaws, Premise: transaction.LocalOperator, Branch: branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now})
	report.Outcome, report.Coverage, report.Detail, report.Kind = modeled.Outcome, modeled.Coverage, modeled.Detail, modeled.Kind
	report.Release = choice.TargetID
	if modeled.Kind != "Transaction" || modeled.Plan == nil {
		return report, nil
	}
	report.Receipt, err = applyBeforeCommit(repo, session, modeled.Plan, beforeCommit)
	return report, err
}
