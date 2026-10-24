package store

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// recordedProcessIdentity reads a recorded PID's start identity; tests
// replace it to model a Linux btime read one second off (PSR-V0-227).
var recordedProcessIdentity = supervisor.ProcessIdentity

// recordedMatch compares fresh, read now for a recorded PID, with the
// identity another process recorded for it (PSR-V0-227). Exact equality is
// the same process, as before the btime tolerance; otherwise it is
// supervisor.CompareIdentity, so a Linux btime one second off is the same
// process and a matching start tick with btime further off is unknown.
func recordedMatch(fresh, recorded string) supervisor.IdentityMatch {
	if fresh == recorded {
		return supervisor.IdentitySame
	}
	return supervisor.CompareIdentity(fresh, recorded)
}

// ownerNotLive refuses while p's unreleased owner, whose identity read now is
// fresh, still runs (ATTEMPT_LIVE) or may still run: a matching start tick
// with a Linux btime more than a second off is QUIESCENCE_UNPROVED, never a
// gone owner (PSR-V0-227).
func ownerNotLive(fresh string, p snapshot.Program) error {
	if p.OwnerReleased {
		return nil
	}
	switch recordedMatch(fresh, p.OwnerStarted) {
	case supervisor.IdentitySame:
		return wire.Errorf(wire.CodeAttemptLive, "program", "program still has a live owner")
	case supervisor.IdentityUnknown:
		return wire.Errorf(wire.CodeQuiescenceUnproved, "program", "program owner identity is uncertain: its start tick matches but its boot time differs")
	}
	return nil
}
