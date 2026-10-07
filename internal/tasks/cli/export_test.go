package cli

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
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

// SetDetachExecutable makes a detached launcher start program with prefix
// before the CLI arguments; it returns the restore function.
func SetDetachExecutable(program string, prefix ...string) func() {
	was := detachExecutable
	detachExecutable = func() (string, []string, error) { return program, prefix, nil }
	return func() { detachExecutable = was }
}

// SetRunPoll shortens an attach's wait between record reads.
func SetRunPoll(d time.Duration) func() {
	was := runPoll
	runPoll = d
	return func() { runPoll = was }
}

// SetRunSegmentBytes shrinks a detached run's output segment.
func SetRunSegmentBytes(n int64) func() {
	was := runSegmentBytes
	runSegmentBytes = n
	return func() { runSegmentBytes = was }
}

// RunDirForTest is the private directory of an attempt's detached runs.
func RunDirForTest(cwd, attemptID string) (string, error) {
	repo, err := intent.Resolve(cwd)
	if err != nil {
		return "", err
	}
	return runsDir(repo, attemptID), nil
}

// ObserveTickets runs the native dispatcher observation at cwd and returns
// its ticket views (ERG-V0-009 gate exposure).
func ObserveTickets(cwd string) ([]dispatch.Ticket, error) {
	o, err := dispatchQueue{env: Env{Cwd: cwd, Stdout: io.Discard, Stderr: io.Discard}}.Observe(context.Background())
	if err != nil {
		return nil, err
	}
	return o.Tickets, nil
}

// DecodeRunRecord exposes the run-record decoder to the CAL-V0-131 format
// table.
func DecodeRunRecord(raw []byte) error {
	_, err := decodeRunRecord(raw)
	return err
}
