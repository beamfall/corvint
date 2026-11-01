package store

import (
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// plainCheckpointStores names, by state directory, the stores whose
// complete-route writes retain the writer checkpoint at the audited head with
// no base at the sealed sequence, as before JSG-V0-780. It is a test seam for
// the fixtures that exercise a seal validating from a checkpoint inside its
// range (JSG-V0-009, JSG-V0-010); it is empty in use.
var plainCheckpointStores sync.Map

// plainCheckpoint reports whether repo's complete-route writes keep the plain
// derivation.
func plainCheckpoint(repo *intent.Repository) bool {
	_, ok := plainCheckpointStores.Load(repo.StateDir)
	return ok
}
