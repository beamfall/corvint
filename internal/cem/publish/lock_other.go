//go:build !darwin && !linux

package publish

import (
	"errors"
	"os"
)

// openLockFile refuses: serialized CEM map updates are unsupported on this
// platform, and an unserialized update could be lost.
func openLockFile(string) (*os.File, error) {
	return nil, errors.New("serialized CEM map updates are unsupported on this platform")
}

func tryLockFile(*os.File) (bool, error) { return false, nil }

func unlockFile(*os.File) {}

// syncDirectory is a no-op where directory handles cannot be synced.
func syncDirectory(string) error { return nil }
