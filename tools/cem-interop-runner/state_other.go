//go:build !unix

package main

import (
	"errors"
	"os"
)

// lockOwnerUID has no owning-UID stat on this platform, so the caller fails
// closed.
func lockOwnerUID(os.FileInfo) (int, bool) { return 0, false }

// flockExclusive has no advisory-lock equivalent on this platform, so the
// caller fails closed instead of writing unguarded.
func flockExclusive(int) (bool, error) {
	return false, errors.New("flock-unsupported")
}

func flockUnlock(int) {}
