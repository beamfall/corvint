// SPDX-License-Identifier: AGPL-3.0-or-later
package gitstatus

import "sync/atomic"

var ownedWorker atomic.Bool

// EnableOwnedWorker is startup-only: call before Git resolution, Core work, or
// goroutines. It permanently selects inheritance of the enclosing native worker
// group. It is not an environment/configuration option or hostile-code sandbox.
func EnableOwnedWorker() error {
	if err := verifyWorkerGroup(); err != nil {
		return err
	}
	ownedWorker.Store(true)
	return nil
}

// OwnedWorker reports the immutable startup choice for Core launchers.
func OwnedWorker() bool { return ownedWorker.Load() }
