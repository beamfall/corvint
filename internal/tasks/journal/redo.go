package journal

import "github.com/Beamfall/corvint/internal/tasks/wire"

// DeleteRedo classifies a validated UNPAUSE deletion. It never unlinks.
// The caller must propagate any read error before supplying current=nil.
func DeleteRedo(pre wire.Digest, current *wire.Digest) (string, error) {
	if _, err := wire.ParseDigest("/pre", string(pre)); err != nil {
		return "", err
	}
	if current == nil {
		return "ALREADY_APPLIED", nil
	}
	if _, err := wire.ParseDigest("/current", string(*current)); err != nil {
		return "", err
	}
	if *current == pre {
		return "DELETE_ELIGIBLE", nil
	}
	return "", wire.Errorf(wire.CodeJournalForked, "barrier.json", "deletion encountered a third digest")
}
