//go:build !(darwin && (arm64 || amd64))

package groupreap

import (
	"errors"
	"syscall"
	"time"
)

// RetirementSupported is false: no process-table or environment read has
// been qualified here, so callers must refuse before launch.
const RetirementSupported = false

// Never sent: NewRetirer refuses before any signal on this platform.
const sigStop, sigKill = syscall.Signal(-1), syscall.Signal(-1)

func platformPrimitives() retirePrimitives {
	unsupported := errors.ErrUnsupported
	return retirePrimitives{
		rows:     func() ([]procRow, error) { return nil, unsupported },
		identity: func(int) (procRow, bool, error) { return procRow{}, false, unsupported },
		environ:  func(int, string) (bool, error) { return false, unsupported },
		signal:   func(int, syscall.Signal) error { return unsupported },
		sleep:    time.Sleep,
	}
}
