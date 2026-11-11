// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package runevidence

import (
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on name, creating it 0600, and returns the release.
// The kernel drops the lock if the process dies, so no stale lock needs recovery (RVD-V1-002).
func lockFile(name string) (func(), error) {
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
