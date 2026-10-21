package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// GateReuseForTest records a gate run whose facts are record and source,
// as GateRun does once it has chosen a reuse, so a test can hand the model
// a reuse GateRun itself would never choose (CAL-V0-240..242).
func GateReuseForTest(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, record, source []byte, now wire.Timestamp) (*Report, error) {
	choice.gate = &gateRun{record: record, source: source}
	return Lease(ctx, repo, actor, choice, now)
}

// SetDescentRootForTest points the receipt audit's ancestry reads for repo
// at root, the test's separate checkout (TCD-V0-153).
func SetDescentRootForTest(repo *intent.Repository, root string) {
	descentRoots.Store(repo.StateDir, root)
}
