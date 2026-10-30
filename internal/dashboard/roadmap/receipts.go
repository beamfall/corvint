package roadmap

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	receiptStateCurrent = "CURRENT"
	receiptStateStale   = "STALE"
)

// receiptFile is the on-disk shape this join reads from the configured
// receipts directory: one JSON file per requirement id, naming the tree
// identity the projection was measured against alongside the shared
// testvalidity.Projection itself. No repository convention for a persisted
// projection receipt exists yet; this is a proposed minimal envelope (see
// the IPR-10 dashboard evidence note and the spec addition it proposes for
// docs/specs/local-observability-dashboard-v0.md).
type receiptFile struct {
	InputIdentity string                  `json:"inputIdentity"`
	Projection    testvalidity.Projection `json:"projection"`
}

// readReceipt opens name beneath dir through os.Root, so a requirement id
// taken from the planning store ("../x", or a symlink leaving dir) cannot
// name a file outside the receipts directory.
func readReceipt(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.OpenFile(name, inputOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readRegularBounded(file)
}

// LoadReceipt reads "<dir>/<requirementID>.json" and classifies it against
// currentTreeDigest. A missing directory, missing file, unreadable/
// malformed file, or absent current-tree digest is NOT_OBSERVED with a
// reason, as is a name that escapes dir, a non-regular file, or one past
// maxInputFileBytes — this never reports a receipt as current when it cannot verify
// that. worktreeDirty is whatever WorktreeDirty last observed for the same
// root currentTreeDigest was measured against: a dirty worktree means
// currentTreeDigest no longer reflects everything on disk, so CURRENT/STALE
// would be an invented certainty (AGENTS.md invariant 2) — the
// classification renders NOT_OBSERVED with reason "worktree-dirty" instead,
// on both a would-be match and a would-be mismatch.
func LoadReceipt(dir, requirementID, currentTreeDigest string, worktreeDirty bool) TestReceipt {
	receipt := TestReceipt{RequirementID: requirementID, State: notObserved}
	if dir == "" {
		receipt.Reason = "no receipts directory configured"
		return receipt
	}
	data, err := readReceipt(dir, requirementID+".json")
	if errors.Is(err, fs.ErrNotExist) {
		receipt.Reason = "no receipt file for " + requirementID
		return receipt
	}
	if err != nil {
		receipt.Reason = "receipt file for " + requirementID + " is not a contained, regular, bounded file"
		return receipt
	}
	var parsed receiptFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		receipt.Reason = "receipt file did not parse as JSON"
		return receipt
	}
	receipt.InputIdentity = parsed.InputIdentity
	projection := parsed.Projection
	receipt.Projection = &projection
	if currentTreeDigest == "" {
		receipt.Reason = "current tree digest not observed"
		return receipt
	}
	if worktreeDirty {
		receipt.Reason = "worktree-dirty"
		return receipt
	}
	if parsed.InputIdentity == "" || parsed.InputIdentity != currentTreeDigest {
		receipt.State = receiptStateStale
		receipt.Reason = "receipt inputIdentity does not match the current tree digest"
		return receipt
	}
	receipt.State = receiptStateCurrent
	return receipt
}
