// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

package runevidence

import (
	"errors"
	"os"
)

// noFollow is zero where the platform has no O_NOFOLLOW; readBounded's Lstat check still refuses
// a linked file on read.
const noFollow = 0

// openLock refuses on a host without the advisory lock the append needs; the caller prints
// run-evidence-write-failed and keeps its exit code (RVD-V1-010). Nothing is created.
func openLock(string) (*os.File, error) {
	return nil, errors.New("the run evidence store needs an advisory file lock this platform does not provide")
}

func tryLock(*os.File) (bool, error) {
	return false, errors.New("the run evidence store needs an advisory file lock this platform does not provide")
}

func unlockFile(*os.File) {}
