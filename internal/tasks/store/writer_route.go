package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutateWriter is mutateLocked's writer-checkpoint route (CAL-V0-116,
// proposed). It runs under the caller's lock and session, after the writer
// guards and §5.2 redo, models the envelope against the summarized inventory
// and commits it as the complete route does. handled is false, with nothing
// written, whenever the route declines: a request that may replay, a REOPEN
// or review operation (they read attempt history), a model that needed an
// elided path, and every refusal, which the complete route derives itself.
func mutateWriter(ctx context.Context, repo *intent.Repository, session *authority.Session, headState *snapshot.Head, request transaction.Request, env *mutation.Envelope, now wire.Timestamp, report *Report, refresh *bool) (handled bool, out *Report, err error) {
	if env.Operation == mutation.OpReopen || mutation.IsReviewOperation(env.Operation) {
		return false, nil, nil
	}
	start := time.Now()
	w, decline, terminal := observeWriter(repo, headState, env.RequestID, true)
	if terminal != nil {
		out, err = guardFailure(report, env.RequestID, terminal)
		return true, out, err
	}
	declined := func(why string) (bool, *Report, error) {
		mutationStage(ctx, "fast.declined: "+why)
		return false, nil, nil
	}
	if decline != nil {
		if headState.LastSeq.Uint64() < minWriterCheckpointSeq {
			return false, nil, nil
		}
		return declined(decline.Error())
	}
	at := writerStage(ctx, "observe", start)
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range w.inv.FilesUnder("intent/") {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	tickets := make([][]byte, 0, len(paths)-2)
	releases := [][]byte{}
	for _, path := range paths {
		record, ok := w.proof.Records[path]
		if !ok || record.Raw == nil {
			return declined(path + " was not selected")
		}
		switch {
		case path == "intent/queue.json" || path == "intent/policy.json":
		case strings.HasPrefix(path, "intent/releases/"):
			releases = append(releases, record.Raw)
		default:
			tickets = append(tickets, record.Raw)
		}
	}
	queue := w.proof.Records["intent/queue.json"].Raw
	q, err := intent.DecodeQueue(queue)
	if err != nil {
		return declined(err.Error())
	}
	priorNote, err := priorNoteEvent(repo, env, w.proof.Records)
	if err != nil {
		return declined(err.Error())
	}
	result := transaction.Model(request, transaction.Input{
		Inventory:         w.inv,
		Head:              w.head,
		HeadReceipt:       w.headReceipt,
		Queue:             queue,
		Policy:            w.proof.Records["intent/policy.json"].Raw,
		Barrier:           w.barrier,
		Reservations:      w.reservations,
		CanonicalTickets:  tickets,
		CanonicalReleases: releases,
		Attempts:          [][]byte{},
		Premise:           transaction.LocalOperator,
		Branch:            w.branch,
		Replay:            transaction.ReplayObservation{State: "ABSENT"},
		RecordedAt:        now,
		PriorNoteEvent:    priorNote,
	})
	at = writerStage(ctx, "model", at)
	if w.inv.Incomplete() {
		return declined("model needed an elided path")
	}
	if result.Kind != "Transaction" || result.Plan == nil {
		return declined("model did not plan a transaction")
	}
	if err = requireBranch(repo, q.IntentBranch); err != nil {
		return declined(err.Error())
	}
	if err = bindObservation(repo, w.proof.Identity, transaction.Mutate); err != nil {
		return declined(err.Error())
	}
	at = writerStage(ctx, "bind", at)
	report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
	report.Receipt, err = apply(repo, session, result.Plan)
	if err != nil {
		out, err = guardFailure(report, env.RequestID, err)
		return true, out, err
	}
	at = writerStage(ctx, "apply", at)
	receipt, err := snapshot.DecodeReceipt(result.Plan.Receipt())
	if err != nil {
		out, err = guardFailure(report, env.RequestID, err)
		return true, out, err
	}
	if receipt.TicketID != nil {
		report.Ticket = receipt.TicketID.Raw
	}
	if advanceWriterCheckpoint(repo, w) {
		*refresh = true
	}
	writerStage(ctx, "checkpoint", at)
	return true, report, nil
}

// writerLeaseVerb reports whether the writer-checkpoint route serves the
// request: the liveness and claim writes that dominate queue traffic
// (V1-0887). Every other lease verb keeps the complete route.
func writerLeaseVerb(request transaction.Request) bool {
	if request.Operation != transaction.Lease || request.Lease == nil {
		return false
	}
	switch request.Lease.Verb {
	case transaction.LeaseClaim, transaction.LeaseClaimNext, transaction.LeaseRenew, transaction.LeaseHeartbeat, transaction.LeaseRelease:
		return true
	}
	return false
}

// leaseWriter is leaseWrite's writer-checkpoint route (CAL-V0-116, proposed):
// it observes, models and commits under one writer lock, with no change guard
// and no complete inventory. The lock excludes cooperating writers, and
// bindObservation rechecks the head and the intent tree before effects, as
// Mutate does. handled is false, with nothing written, whenever the route
// declines; leaseWrite then prepares the complete route as before. refresh
// reports that the scheduled complete audit is due.
func leaseWriter(ctx context.Context, repo *intent.Repository, request transaction.Request, now wire.Timestamp, beforeCommit func() error, facts claimObserver, report *Report) (handled, refresh bool, proof *journal.Result, err error) {
	if !writerLeaseVerb(request) {
		return false, false, nil, nil
	}
	if head, err := readHead(repo); err != nil || head.LastSeq.Uint64() < minWriterCheckpointSeq {
		return false, false, nil, nil
	}
	if _, err := os.Lstat(journal.WriterCheckpointPath(repo.StateDir)); err != nil {
		return false, false, nil, nil
	}
	timing, wait := leaseTimingOf(ctx), time.Now()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{CallerWait: leaseLockWaitOf(ctx)})
	timing.LockWait += time.Since(wait)
	if err != nil {
		// The complete route would wait for the same lock and fail alike.
		return true, false, nil, err
	}
	held := time.Now()
	defer func() {
		cleanup := lock.Close()
		timing.LockHold += time.Since(held)
		if cleanup != nil {
			handled, err = true, errors.Join(err, cleanup)
		}
	}()
	headState, err := writerGuards(repo, guardOperation(request))
	if err != nil || headState.QueueID.Raw != request.QueueID {
		return false, false, nil, nil
	}
	read := time.Now()
	w, decline, terminal := observeWriter(repo, headState, request.RequestID, false)
	timing.SnapshotRead += time.Since(read)
	if terminal != nil {
		return true, false, nil, terminal
	}
	if decline != nil {
		return false, false, nil, nil
	}
	validate := time.Now()
	now = recordedAt(ctx, now)
	input := transaction.Input{Inventory: w.inv, Head: w.head, HeadReceipt: w.headReceipt, Queue: w.proof.Records["intent/queue.json"].Raw, Policy: w.proof.Records["intent/policy.json"].Raw, Barrier: w.barrier, Reservations: w.reservations, Pools: w.proof.Records["pools.json"].Raw, Programs: w.proof.Records["programs.json"].Raw, Premise: transaction.LocalOperator, Branch: w.branch, Replay: transaction.ReplayObservation{State: "ABSENT"}, RecordedAt: now}
	for path, record := range w.proof.Records {
		if record.Raw == nil {
			continue
		}
		switch {
		case strings.HasPrefix(path, "intent/tickets/"):
			input.CanonicalTickets = append(input.CanonicalTickets, record.Raw)
		case strings.HasPrefix(path, "intent/releases/"):
			input.CanonicalReleases = append(input.CanonicalReleases, record.Raw)
		case strings.HasPrefix(path, "attempts/"):
			input.Attempts = append(input.Attempts, record.Raw)
		}
	}
	if err := ctx.Err(); err != nil {
		return true, false, nil, err
	}
	if err := leaseInput(w.proof, input.Attempts, facts, &input); err != nil {
		return false, false, nil, nil
	}
	result := transaction.Model(request, input)
	timing.Validation += time.Since(validate)
	if w.inv.Incomplete() || result.Kind != "Transaction" || result.Plan == nil {
		return false, false, nil, nil
	}
	if err := bindObservation(repo, w.proof.Identity, guardOperation(request)); err != nil {
		return false, false, nil, nil
	}
	write := time.Now()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		timing.wrote(write, 0)
		return true, false, nil, err
	}
	defer func() {
		cleanup := session.Close()
		timing.wrote(write, session.SyncDuration())
		if cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	setLeaseReport(report, result)
	if report.Receipt, err = applyBeforeCommit(repo, session, result.Plan, beforeCommit); err != nil {
		return true, false, nil, err
	}
	return true, advanceWriterCheckpoint(repo, w), w.proof, nil
}
