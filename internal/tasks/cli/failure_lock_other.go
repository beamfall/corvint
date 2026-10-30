//go:build !darwin && !linux

package cli

import (
	"errors"
	"time"
)

// Without a ledger lock the guard runs unguarded (TOL-V0-054).
func lockFailureLedger(string, time.Duration) (func(), error) {
	return nil, errors.New("the ledger lock needs Darwin or Linux")
}
