//go:build !darwin && !linux

package cli

import (
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Detached claims refuse UNSUPPORTED before any lock here (ATR-V0-020).
func lockLeaseDetach(string, time.Duration) (func(), error) {
	return nil, wire.Errorf(wire.CodeUnsupported, "detach", "detached claims need Darwin or Linux")
}

func leaseDetachLockHeld(string) bool { return false }

func leaseDetachShareLaunch(string) (func(), bool, error) { return func() {}, false, nil }
