package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func clearLeaseOrphans(ctx context.Context, repo *intent.Repository, operation string) error {
	entries, err := os.ReadDir(filepath.Join(repo.StateDir, "staging"))
	if os.IsNotExist(err) || (err == nil && len(entries) == 0) {
		return nil
	}
	if err != nil {
		return err
	}
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return err
	}
	defer lock.Close()
	if _, err := writerGuards(repo, operation); err != nil {
		return err
	}
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return err
	}
	defer session.Close()
	_, err = session.RemoveOrphanStages()
	return err
}

// redoLeaseProof consumes a pending receipt audited outside the lock. Its
// caller binds the exact head and every physical input with a ChangeGuard.
func redoLeaseProof(repo *intent.Repository, session *authority.Session, proof *journal.Result) error {
	if proof == nil || !proof.Pending || proof.StagingPresent || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "PRE_OR_POST" || proof.IntentError != nil {
		return wire.Errorf(wire.CodeJournalForked, "redo", "missing verified pending receipt")
	}
	head, err := readHead(repo)
	if err != nil {
		return err
	}
	if wire.Sum(wire.EncodeFile(head.Value())) != proof.Identity.HeadSha256 {
		return wire.Errorf(wire.CodeSnapshotMoved, "redo", "audited head changed")
	}
	if err := checkChainBounds(repo, head); err != nil {
		return err
	}
	queue, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return err
	}
	if err := requireBranch(repo, queue.IntentBranch); err != nil {
		return err
	}
	raw, found, err := receiptBytes(repo, head.LastSeq.Uint64()+1)
	if err != nil {
		return err
	}
	if !found || wire.Sum(raw) != proof.LastReceiptSha256 {
		return wire.Errorf(wire.CodeSnapshotMoved, "redo", "audited receipt changed")
	}
	rc, err := snapshot.DecodeReceipt(raw)
	if err != nil {
		return err
	}
	if rc.Seq.Uint64() != head.LastSeq.Uint64()+1 || rc.Prev == nil || head.LastReceiptSha256 == nil || *rc.Prev != *head.LastReceiptSha256 {
		return wire.Errorf(wire.CodeJournalForked, "redo", "pending receipt does not chain to head")
	}
	if err := redoPosts(repo, session, rc); err != nil {
		return err
	}
	return advanceHead(repo, session, head, rc, wire.Sum(raw))
}

// settleLease releases the lock before auditing a pending receipt. The common
// no-recovery path only inspects bounded head/receipt/staging records.
func settleLease(ctx context.Context, repo *intent.Repository) (bool, error) {
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return false, err
	}
	redone := false
	for round := 0; round < 16; round++ {
		pending, err := probeLeaseRecovery(ctx, repo)
		if err != nil || !pending {
			return redone, err
		}
		g, err := authority.WatchChanges(repo)
		if err != nil {
			return redone, err
		}
		err = func() error {
			head, err := readHead(repo)
			if err != nil {
				return err
			}
			proof, err := journalReader(repo, head).AuditForWrite()
			if err == nil {
				return wire.Errorf(wire.CodeSnapshotMoved, "redo", "another writer settled receipt")
			}
			if wire.CodeOf(err) != wire.CodeRedoPending {
				return err
			}
			p := &preparedLease{guard: g, proof: proof, head: wire.EncodeFile(head.Value()), pending: true}
			report := &Report{}
			r := transaction.Request{Operation: transaction.Lease, QueueID: head.QueueID.Raw, Lease: &transaction.LeaseRequest{}}
			if err := commitLease(ctx, repo, r, p, report, nil); err != nil {
				return err
			}
			redone = redone || report.Redone
			return nil
		}()
		closeErr := g.Close()
		if closeErr != nil {
			return redone, errors.Join(err, closeErr)
		}
		if err != nil && wire.CodeOf(err) != wire.CodeSnapshotMoved {
			return redone, err
		}
	}
	return redone, wire.Errorf(wire.CodeSnapshotMoved, "redo", "store changed during all recovery attempts")
}

func probeLeaseRecovery(ctx context.Context, repo *intent.Repository) (_ bool, err error) {
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return false, err
	}
	defer func() {
		if cleanup := lock.Close(); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	head, err := writerGuards(repo, transaction.Lease)
	if err != nil {
		return false, nil
	} // the lease write reports guard refusals
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return false, err
	}
	defer func() {
		if cleanup := session.Close(); cleanup != nil {
			err = errors.Join(err, cleanup)
		}
	}()
	if _, err := session.RemoveOrphanStages(); err != nil {
		return false, err
	}
	if err := checkChainBounds(repo, head); err != nil {
		return false, err
	}
	_, found, err := receiptBytes(repo, head.LastSeq.Uint64()+1)
	return found, err
}
