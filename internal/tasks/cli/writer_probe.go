package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// writerProbe is the one pre-write probe of a command whose store writer
// settles a pending receipt under the store lock before it models the
// request (§5.2 redo). The probe only names the queue, so a committed receipt
// that head.json does not name yet (REDO_PENDING) with a readable head is not
// refused here: the writer settles it, or refuses with the audit's own error
// when it cannot (CAL-V0-222, CAL-V0-260). pending reports that case. A nil
// probe is snapshot.Probe.
func writerProbe(probe func(string) (*snapshot.Snapshot, error), stateDir string) (observed *snapshot.Snapshot, pending bool, err error) {
	if probe == nil {
		probe = snapshot.Probe
	}
	observed, err = probe(stateDir)
	if err == nil {
		return observed, false, nil
	}
	if wire.CodeOf(err) == wire.CodeRedoPending && observed != nil && observed.Head != nil {
		return observed, true, nil
	}
	return observed, false, err
}
