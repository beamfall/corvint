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
	"github.com/Beamfall/corvint/internal/tasks/ticket"
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
	// The CAL-V0-070 change watch holds a descriptor per watched path on some
	// platforms. Like lease preparation's, it closes after the lock is released.
	var watch *authority.ChangeGuard
	defer func() { _ = watch.Close() }()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	defer lock.Close()
	now = recordedAt(ctx, now)

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

	// CAL-V0-070: one audit answers the request lookup and supplies the
	// canonical intent records, with the same refusals as Lookup and Audit made
	// apart. A watch registered before that audit reads anything, and a fresh
	// inventory that matches every digest it read, decide whether the second
	// Audit may reuse it; without a watch, or after a refusal, the inventory
	// and Audit run fresh, as before.
	reader := journalReader(repo, headState)
	if watch, err = authority.WatchChanges(repo); err != nil {
		watch = nil
	} else {
		mutationStage(ctx, "watched")
	}
	audit, err := reader.AuditForMutation(env.RequestID)
	if err != nil && watch != nil {
		// The watch's descriptors share the process limit with the audit's
		// reads, so a refusal is taken again without them, as Lookup took it.
		_ = watch.Close()
		watch = nil
		mutationStage(ctx, "retry: "+err.Error())
		audit, err = reader.AuditForMutation(env.RequestID)
	}
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	if audit.Found {
		result := replayResult(request, audit.Entry)
		report.Outcome, report.Coverage, report.Detail, report.Kind = result.Outcome, result.Coverage, result.Detail, result.Kind
		if result.Kind == "Replay" {
			report.Ticket = audit.TicketID
		}
		return report, nil
	}
	inv, paths, canonical, err := observeMutation(ctx, repo, reader, audit, watch)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	retainCheckpoint(repo, canonical)
	if canonical.StagingPresent {
		return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
	}
	// Recovery uses journal-authoritative attempt bytes, not unaudited physical
	// files. Ordinary completed REOPEN keeps its existing read boundary.
	if env.Operation == mutation.OpReopen && env.TargetID != nil {
		path := "intent/tickets/" + env.TargetID.Local + ".json"
		if record, ok := canonical.Records[path]; ok {
			rec, decodeErr := ticket.Decode(record.Raw)
			if decodeErr != nil {
				return guardFailure(report, env.RequestID, decodeErr)
			}
			if rec.Status == ticket.StatusOpen {
				for _, file := range inv.Files() {
					if strings.HasPrefix(file.Path, "attempts/") {
						paths = append(paths, file.Path)
					}
				}
				canonical, err = reader.Audit(paths...)
				if err != nil {
					return guardFailure(report, env.RequestID, err)
				}
				if canonical.StagingPresent {
					return report, wire.Errorf(wire.CodeUnsupported, "staging", "active staging recovery is not implemented")
				}
			}
		}
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
	attempts := [][]byte{}
	for _, path := range paths[2:] {
		if strings.HasPrefix(path, "attempts/") {
			attempts = append(attempts, canonical.Records[path].Raw)
		} else if strings.HasPrefix(path, "intent/releases/") {
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
	headRc, err := headReceipt(repo, head)
	if err != nil {
		return report, err
	}
	priorNote, err := priorNoteEvent(repo, env, canonical.Records)
	if err != nil {
		return guardFailure(report, env.RequestID, err)
	}
	result := transaction.Model(
		request,
		transaction.Input{
			Inventory:         inv,
			Head:              head,
			HeadReceipt:       headRc,
			Queue:             queue,
			Policy:            policy,
			Barrier:           barrier,
			Reservations:      reservations,
			CanonicalTickets:  tickets,
			CanonicalReleases: releases,
			Attempts:          attempts,
			Premise:           transaction.LocalOperator,
			Branch:            branch,
			Replay:            transaction.ReplayObservation{State: "ABSENT"},
			RecordedAt:        now,
			PriorNoteEvent:    priorNote,
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

// headReceipt reads the receipt head.json names, so the model can refuse a
// transaction recorded earlier than it (CAL-V0-012).
func headReceipt(repo *intent.Repository, headRaw []byte) ([]byte, error) {
	head, err := snapshot.DecodeHead(headRaw)
	if err != nil {
		return nil, err
	}
	return readReceiptBytes(repo, head.LastSeq.Uint64())
}

func optional(path string, bound int) ([]byte, error) {
	raw, err := intent.ReadFile(path, bound)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}

// mutationStageKey carries a test-only callback that Mutate and
// observeMutation run at their named stages, so a test can change the store,
// or the descriptor limit, at a fixed point.
type mutationStageKey struct{}

func mutationStage(ctx context.Context, stage string) {
	if hook, ok := ctx.Value(mutationStageKey{}).(func(string)); ok {
		hook(stage)
	}
}

// observeMutation returns Mutate's inventory, its selection and the canonical
// audit of that selection (CAL-V0-070). The inventory is always read fresh.
// The merged audit's records stand in for a second Audit only when every file
// that audit read has the same digest and size in the inventory, and watch,
// registered before that audit read anything, has seen nothing change in the
// state directory or the intent tree once both are taken. The digests vouch
// for content: a write through a shared mapping changes what a read returns
// with no inotify event at all, and no kqueue event unless msync is called.
// The watch vouches for names, types and modes, which change only through
// calls it reports.
// Otherwise, and without a watch, both run fresh after the watch is closed,
// so a refusal and any change between the merged audit and the inventory are
// refused as the separate passes refused them.
func observeMutation(ctx context.Context, repo *intent.Repository, reader journal.Reader, audit *journal.MutationAudit, watch *authority.ChangeGuard) (*transaction.Inventory, []string, *journal.Result, error) {
	mutationStage(ctx, "audited")
	if watch != nil {
		inv, err := inventory(repo)
		reuse := err == nil && readUnchanged(inv, audit.Physical.Files)
		var paths []string
		var canonical *journal.Result
		if reuse {
			paths = mutationSelection(inv)
			var reused bool
			if canonical, reused, err = audit.Canonical(paths...); !reused {
				canonical, err = reader.Audit(paths...)
			}
		}
		mutationStage(ctx, "observed")
		if reuse && err == nil && watch.Check() == nil {
			return inv, paths, canonical, nil
		}
		_ = watch.Close()
	}
	mutationStage(ctx, "fresh")
	inv, err := inventory(repo)
	if err != nil {
		return nil, nil, nil, err
	}
	paths := mutationSelection(inv)
	canonical, err := reader.Audit(paths...)
	return inv, paths, canonical, err
}

// readUnchanged reports whether every file the merged audit read is listed
// in inv with the same digest and size. read is nil unless that audit settled
// with no deferred refusal.
func readUnchanged(inv *transaction.Inventory, read map[string]journal.PhysicalFile) bool {
	if read == nil {
		return false
	}
	matched := 0
	for _, file := range inv.Files() {
		if r, ok := read[file.Path]; ok {
			if r.Bytes < 0 || r.Sha256 != file.Sha256 || wire.SizeOf(uint64(r.Bytes)) != file.Bytes {
				return false
			}
			matched++
		}
	}
	return matched == len(read)
}

// mutationSelection is the queue, the policy and every ticket and release
// file the inventory lists, in inventory order.
func mutationSelection(inv *transaction.Inventory) []string {
	paths := []string{"intent/queue.json", "intent/policy.json"}
	for _, file := range inv.Files() {
		if strings.HasPrefix(file.Path, "intent/tickets/") || strings.HasPrefix(file.Path, "intent/releases/") {
			paths = append(paths, file.Path)
		}
	}
	return paths
}

func replayResult(request transaction.Request, entry mutation.IndexEntry) transaction.Result {
	record := wire.NewObject()
	record.Set("requestId", wire.String(entry.RequestID))
	record.Set("seq", wire.String(string(*entry.Outcome.ReceiptSeq)))
	record.Set("mutationSha256", wire.String(string(entry.MutationSha256)))
	record.Set("outcome", entry.Outcome.Value())
	return transaction.Model(request, transaction.Input{Replay: transaction.ReplayObservation{State: "FOUND", Record: wire.EncodeFile(wire.ObjectValue(record))}})
}

// priorNoteEvent reads the target's current operator-note event at the
// audited reference head for NOTE_SET/NOTE_CLEAR (ON-V0-004). A missing file
// returns nil, and the pure transition refuses MISSING_EVIDENCE; the bytes are
// re-hashed against the reference there, so this read is never trusted alone.
func priorNoteEvent(repo *intent.Repository, env *mutation.Envelope, records map[string]journal.Record) ([]byte, error) {
	if !mutation.IsNoteOperation(env.Operation) || env.TargetID == nil {
		return nil, nil
	}
	record, ok := records["intent/tickets/"+env.TargetID.Local+".json"]
	if !ok {
		return nil, nil
	}
	rec, err := ticket.Decode(record.Raw)
	if err != nil || rec.OperatorNote == nil {
		return nil, err
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(rec.OperatorNote.Head)), ticket.MaxOperatorNoteBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}
