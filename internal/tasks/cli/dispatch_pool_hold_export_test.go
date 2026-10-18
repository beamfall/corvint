package cli

import (
	"context"
	"io"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// PoolClaimDispatch is the dispatcher's ATR-V0-120 pre-launch claim against
// the store at root.
func PoolClaimDispatch(root string, a dispatch.Assignment, holder, requestID string) (dispatch.PoolHold, error) {
	return dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}.PoolClaim(context.Background(), a, holder, 0, requestID)
}

// PoolReleaseDispatch is the dispatcher's ATR-V0-122 return against the
// store at root.
func PoolReleaseDispatch(root string, a dispatch.Attempt, allocation, requestID string) error {
	return dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}.PoolRelease(context.Background(), a, allocation, requestID)
}
