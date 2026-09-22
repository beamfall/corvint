//go:build !darwin && !linux

package unplannedread

import (
	"fmt"
	"os"
)

func lockLedgerFile(*os.File) error {
	return fmt.Errorf("serialized unplanned-read writes are unsupported on this platform")
}

func unlockLedgerFile(*os.File) {}
