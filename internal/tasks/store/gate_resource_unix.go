//go:build darwin || linux

package store

import (
	"errors"
	"os"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tryLockGateResource takes the flock on path without waiting (LGL-V0-885).
// The descriptor is close-on-exec, so the gate program never inherits it.
func tryLockGateResource(path string, def *intent.GateDefinition) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			r := def.SharedResource
			return nil, wire.Errorf(wire.CodeLockTimeout, "gate", "gate %s declares sharedResource %s %s, which another gate run holds; nothing ran or was recorded, so the same gate run can succeed once that run ends (LGL-V0-885)", def.GateID, r.Class, r.Key)
		}
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
