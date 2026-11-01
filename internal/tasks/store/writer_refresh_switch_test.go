package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// completeRefreshOnly makes repo's scheduled refresh the complete audit until
// t ends: for tests of what only the complete refresh, the PSR-V0-891
// backstop, detects or retains.
func completeRefreshOnly(t testing.TB, repo *intent.Repository) {
	completeRefreshes.Store(repo.StateDir, struct{}{})
	t.Cleanup(func() { completeRefreshes.Delete(repo.StateDir) })
}
