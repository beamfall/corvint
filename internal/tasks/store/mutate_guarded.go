// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TicketGuard is a precondition over one other ticket's canonical record
// (TLF-V0-884): Check runs on the audited record after §5.2 redo, under the
// writer lock that then commits the envelope, so no writer can change that
// ticket between the check and the commit. A recorded request replays
// without the check. A Check error refuses with nothing written.
type TicketGuard struct {
	TicketID wire.TicketID
	Check    func(*ticket.Record) error
}

// MutateGuarded is Mutate with guard checked under its writer lock.
func MutateGuarded(ctx context.Context, repo *intent.Repository, actor mutation.Binding, envelope []byte, now wire.Timestamp, guard *TicketGuard) (*Report, error) {
	return mutate(ctx, repo, actor, envelope, now, guard)
}

func (g *TicketGuard) check(repo *intent.Repository, head *snapshot.Head, requestID string) error {
	if g == nil {
		return nil
	}
	reader := lockedJournalReader(repo, head)
	idx := journal.RequestIndex{Reader: reader}
	if _, found, err := idx.Lookup(requestID); err != nil || found {
		return err
	}
	path := "intent/tickets/" + g.TicketID.Local + ".json"
	canonical, err := reader.Audit("intent/queue.json", "intent/policy.json", path)
	if err != nil {
		return err
	}
	record, ok := canonical.Records[path]
	if !ok {
		return wire.Errorf(wire.CodeStaleTicket, "ticket", "ticket %s is not in the queue", g.TicketID.Raw)
	}
	rec, err := ticket.Decode(record.Raw)
	if err != nil {
		return err
	}
	return g.Check(rec)
}

// Settle completes a §5.2 pending receipt under the writer lock and writes
// nothing else, so a command whose pre-write read met REDO_PENDING can settle
// the journal and read again. It reports whether a receipt was redone.
func Settle(ctx context.Context, repo *intent.Repository) (bool, error) {
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return false, err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return false, err
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return false, err
	}
	defer session.Close()
	if _, err = writerGuards(repo, transaction.Mutate); err != nil {
		return false, err
	}
	return redoPending(repo, session)
}
