package store

import (
	"context"
	"errors"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// readLeaseProof lets a gate's pre-read and eventual write share the same
// audit when its command leaves the store unchanged. It acquires no lock.
func readLeaseProof(ctx context.Context, repo *intent.Repository) (*journal.Result, error) {
	return readLeaseProofWithAudit(ctx, repo, (journal.Reader).AuditForWriteObserved)
}

// Operation-local seam keeps fault injection independent of concurrent reads.
func readLeaseProofWithAudit(ctx context.Context, repo *intent.Repository, audit func(journal.Reader) (*journal.Result, journal.PhysicalObservation, error)) (*journal.Result, error) {
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g, err := authority.WatchChanges(repo)
		if err != nil {
			if wire.CodeOf(err) == wire.CodeSnapshotMoved || os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		proof, err, auditCleanup := func() (*journal.Result, error, error) {
			inv, err := inventory(repo)
			if err != nil {
				return nil, err, nil
			}
			head, _, _, err := journalBytes(repo)
			if err != nil {
				return nil, err, nil
			}
			return leaseAuditObserved(repo, g, inv, head, nil, audit)
		}()
		if moved := g.Check(); moved != nil && (err == nil || wire.CodeOf(err) == wire.CodeSnapshotMoved) {
			err = moved
		}
		// Native close failures are fatal even alongside SNAPSHOT_MOVED.
		// Keep them separate until the retry decision, then retain every error.
		if cleanup := g.Close(); cleanup != nil || auditCleanup != nil {
			return nil, errors.Join(err, auditCleanup, cleanup)
		}
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		if err != nil {
			return nil, err
		}
		return proof, proof.IntentError
	}
	return nil, wire.Errorf(wire.CodeSnapshotMoved, "gate", "store changed during all read attempts")
}
