// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

package runevidence

import "errors"

// lockFile refuses on a host without the advisory lock the append needs; the caller prints
// run-evidence-write-failed and keeps its exit code (RVD-V1-010).
func lockFile(string) (func(), error) {
	return nil, errors.New("the run evidence store needs an advisory file lock this platform does not provide")
}
