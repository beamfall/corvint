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
		proof, err := func() (*journal.Result, error) {
			inv, err := inventory(repo)
			if err != nil {
				return nil, err
			}
			head, _, _, err := journalBytes(repo)
			if err != nil {
				return nil, err
			}
			return leaseAudit(repo, g, inv, head)
		}()
		if moved := g.Check(); moved != nil {
			err = moved
		}
		if cleanup := g.Close(); cleanup != nil {
			return nil, errors.Join(err, cleanup)
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
