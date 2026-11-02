//go:build !darwin && !linux

package store

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tryLockGateResource has no cross-process lock on this platform, so a gate
// that declares a sharedResource refuses before it runs (LGL-V0-885).
func tryLockGateResource(_ string, def *intent.GateDefinition) (func(), error) {
	return nil, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s declares sharedResource, which gate run serializes only on darwin and linux; run it there, or remove sharedResource from the gate (LGL-V0-885)", def.GateID)
}
