//go:build !unix

package main

import (
	"errors"
	"os"
	"os/exec"
)

// The ledger records only where it can verify ownership and serialise runs,
// so a non-Unix host refuses its directory and runs every step (GL-V0-005).

func ownedByInvokingUser(os.FileInfo) bool { return false }

func lockExclusive(*os.File, bool) error {
	return errors.New("file locking is unsupported on this platform")
}

func unlock(*os.File) {}

func signalExit(*exec.ExitError) (int, bool) { return 0, false }
