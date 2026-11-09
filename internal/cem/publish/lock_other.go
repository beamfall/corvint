//go:build !darwin && !linux

package publish

import (
	"errors"
	"os"
)

// errLockUnsupported is the platform refusal: serialized CEM map updates are
// unsupported on this platform, and an unserialized update could be lost.
var errLockUnsupported = errors.New("serialized CEM map updates are unsupported on this platform")

// LockRefusal returns the platform refusal Lock makes before it creates any
// directory or lock file (CEM-PILOT-033).
func LockRefusal() error { return errLockUnsupported }

func openLockFile(string) (*os.File, error) { return nil, errLockUnsupported }

func tryLockFile(*os.File) (bool, error) { return false, nil }

func unlockFile(*os.File) {}

// syncDirectory is a no-op where directory handles cannot be synced.
func syncDirectory(string) error { return nil }

func syncHeldDirectory(*os.Root) error { return nil }
