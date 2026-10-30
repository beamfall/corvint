//go:build !darwin && !linux

package defects

import (
	"fmt"
	"os"
)

func lockFile(*os.File) error {
	return fmt.Errorf("serialized defect-register writes are unsupported on this platform")
}

func unlockFile(*os.File) {}
