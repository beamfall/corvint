package cli

import (
	"errors"
	"time"
)

// SetAttemptBeatInterval shortens the attempt runner's heartbeat interval for
// a test and returns the restore function.
func SetAttemptBeatInterval(d time.Duration) func() {
	was := attemptBeatInterval
	attemptBeatInterval = d
	return func() { attemptBeatInterval = was }
}

// SetAttemptWriteFault makes fault decide, before each lease write of the
// runner, whether that write fails; it returns the restore function.
func SetAttemptWriteFault(fault func(verb string) error) func() {
	was := attemptWriteFault
	attemptWriteFault = fault
	return func() { attemptWriteFault = was }
}

// SetAttemptGroupSignalFault makes the owned group's SIGKILL fail without
// being sent, so retirement cannot be proved and cleanup HOLDs; it returns
// the restore function. Use it only with a command that exits by itself.
func SetAttemptGroupSignalFault() func() {
	was := attemptGroupPrimitives
	attemptGroupPrimitives.KillGroup = func(int) error { return errors.New("injected group signal failure") }
	return func() { attemptGroupPrimitives = was }
}
