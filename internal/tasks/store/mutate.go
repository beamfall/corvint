package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Mutate commits one §3.3 ticket mutation to the journal (TCP-02b).
//
// The envelope is the caller's; the trusted binding is the caller's; this
// package supplies only the facts a mutation depends on and the §5.2 ordering
// that makes the result durable. It never authenticates, never edits the
// envelope. Entry guards refuse unsupported states before the model or recovery runs.
func Mutate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, envelope []byte, now wire.Timestamp) (*Report, error) {
	report := &Report{}
	if repo == nil {
		return report, wire.Errorf(wire.CodeMalformed, "", "no repository authority was resolved")
	}
	env, err := mutation.Decode(envelope)
	if err != nil {
		return report, err
	}
	request := transaction.Request{Operation: transaction.Mutate, QueueID: env.QueueID.Raw, RequestID: env.RequestID, Actor: actor, Envelope: envelope}
	if env.Actor.ID != actor.ID || env.Actor.Role != actor.Role || (actor.Role != "OWNER" && actor.Role != "OPERATOR") {
		result := transaction.Model(request, transaction.Input{})
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		return report, nil
	}
	if _, err = os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return report, wire.Errorf(wire.CodeUninitialized, repo.StateDir, "no store: run `corvint-tasks init` first")
		}
		return guardFailure(report, env.RequestID, err)
	}
	if _, err = authority.Qualify(repo.CommonDir); err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	defer lock.Close()

	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	defer session.Close()
	headState, err := writerGuards(repo, transaction.Mutate)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	// §5.2: a receipt that was linked in but whose posts or head were not
	// written is completed before anything new is modelled, so the inventory
	// below describes a settled journal.
	redone, err := redoPending(repo, session)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	report.Redone = redone

	reader := journalReader(repo, headState)
	index := journal.RequestIndex{Reader: reader}
	entry, found, err := index.Lookup(env.RequestID)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
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
		return guardFailure(report, env.RequestID, err)
	}
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	canonical, err := reader.Audit(paths...)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	if canonical.StagingPresent {
		return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	queue := canonical.Records["intent/queue.json"].Raw
	policy := canonical.Records["intent/policy.json"].Raw
	q, err := intent.DecodeQueue(queue)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	branch, err := primaryBranch(repo)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	tickets := make([][]byte, 0, len(paths)-2)
	releases := [][]byte{}
	for _, path := range paths[2:] {
		if strings.HasPrefix(path, "intent/releases/") {
			releases = append(releases, canonical.Records[path].Raw)
		} else {
			tickets = append(tickets, canonical.Records[path].Raw)
		}
	}
	head, barrier, reservations, err := journalBytes(repo)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	if wire.Sum(head) != canonical.Identity.HeadSha256 {
		return report, wire.Errorf(wire.CodeSnapshotMoved, "head.json", "validated head changed")
	}
	result := transaction.Model(
		request,
		transaction.Input{
			Inventory:         inv,
			Head:              head,
			Queue:             queue,
			Policy:            policy,
			Barrier:           barrier,
			Reservations:      reservations,
			CanonicalTickets:  tickets,
			CanonicalReleases: releases,
			Premise:           transaction.LocalOperator,
			Branch:            branch,
			Replay:            transaction.ReplayObservation{State: "ABSENT"},
			RecordedAt:        now,
		},
	)
	report.Outcome = result.Outcome
	report.Coverage = result.Coverage
	report.Detail = result.Detail
	report.Kind = result.Kind
	if result.Kind != "Transaction" || result.Plan == nil {
		return report, nil
	}

	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	if err = bindObservation(repo, canonical.Identity, transaction.Mutate); err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	report.Receipt, err = apply(repo, session, result.Plan)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	receipt, err := snapshot.DecodeReceipt(result.Plan.Receipt())
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	if receipt.TicketID != nil {
		report.Ticket = receipt.TicketID.Raw
	}
	return report, nil
}

// journalBytes reads the three §5.2 mutable state files. Only the head is
// required; an absent barrier means no barrier is in force.
func journalBytes(repo *intent.Repository) (head, barrier, reservations []byte, err error) {
	head, err = intent.ReadFile(filepath.Join(repo.StateDir, "head.json"), 4096)
	if err != nil {
		return nil, nil, nil, err
	}
	barrier, err = optional(filepath.Join(repo.StateDir, "barrier.json"), 4096)
	if err != nil {
		return nil, nil, nil, err
	}
	reservations, err = optional(filepath.Join(repo.StateDir, "reservations.json"), wire.MaxReservationSetBytes)
	if err != nil {
		return nil, nil, nil, err
	}
	return head, barrier, reservations, nil
}

func optional(path string, bound int) ([]byte, error) {
	raw, err := intent.ReadFile(path, bound)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}

func replayResult(request transaction.Request, entry mutation.IndexEntry) transaction.Result {
	record := wire.NewObject()
	record.Set("requestId", wire.String(entry.RequestID))
	record.Set("seq", wire.String(string(*entry.Outcome.ReceiptSeq)))
	record.Set("mutationSha256", wire.String(string(entry.MutationSha256)))
	record.Set("outcome", entry.Outcome.Value())
	return transaction.Model(request, transaction.Input{Replay: transaction.ReplayObservation{State: "FOUND", Record: wire.EncodeFile(wire.ObjectValue(record))}})
}
