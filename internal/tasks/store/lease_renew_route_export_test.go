package store

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// LowerWriterCheckpointForTest lets external lease tests run the same
// sequence on the writer-checkpoint route (CAL-V0-115) as on the complete route.
func LowerWriterCheckpointForTest(t *testing.T, repo *intent.Repository) {
	t.Helper()
	lowerWriterCheckpoint(t, repo)
}
