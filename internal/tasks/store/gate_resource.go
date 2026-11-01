package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// gateResourceDir holds one lock file per gate sharedResource under the Git
// common directory, beside the store's writer lock and outside its state
// directory, so no inventory or audit reads it (LGL-V0-885).
const gateResourceDir = "taskman-gate-resources"

// lockGateResource admits one gate run at a time per policy sharedResource
// (LGL-V0-885). A gate that declares none takes no lock. Otherwise the run
// takes an exclusive, non-blocking lock named for the resource's class and
// key and holds it until the returned release; a second run while it is held
// refuses LOCK_TIMEOUT before its program starts. The kernel drops the lock
// when its holder dies, so a crashed run never leaves the resource taken.
func lockGateResource(repo *intent.Repository, def *intent.GateDefinition) (func(), error) {
	if def.SharedResource == nil {
		return func() {}, nil
	}
	dir := filepath.Join(repo.CommonDir, gateResourceDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(def.SharedResource.Class + "\x00" + def.SharedResource.Key))
	return tryLockGateResource(filepath.Join(dir, hex.EncodeToString(sum[:])+".lock"), def)
}
