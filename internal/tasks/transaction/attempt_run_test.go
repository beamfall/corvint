package transaction

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0005_OutcomeFactsAgree: every shape the runner can produce encodes,
// and each contradiction in the spec's class consistency table refuses as
// MALFORMED.
func TestATRV0005_OutcomeFactsAgree(t *testing.T) {
	ip := func(n int) *int { return &n }
	sp := func(s string) *string { return &s }
	shape := func(class, cleanup string, exit, sig *int, lost *string) RunOutcome {
		o := RunOutcome{Profile: RunOutcomeProfile, RunID: "r1", AttemptID: "a", Generation: "1", ArgvSha256: strings.Repeat("a", 64), TimeoutSeconds: 1, StartedAt: sp("2026-10-04T00:00:00Z"), EndedAt: "2026-10-04T00:00:01Z", Class: class, ExitCode: exit, Signal: sig, Cleanup: cleanup, Heartbeats: 1, LostLease: lost}
		if class == RunSpawnFailed {
			o.StartedAt = nil
		}
		return o
	}
	fenced := sp(wire.CodeFenced)
	for name, o := range map[string]RunOutcome{
		"exit":                 shape(RunExit, CleanupReleased, ip(0), nil, nil),
		"signal":               shape(RunSignal, CleanupReleased, nil, ip(9), nil),
		"timeout killed":       shape(RunTimeout, CleanupReleased, nil, ip(9), nil),
		"timeout exited":       shape(RunTimeout, CleanupReleased, ip(3), nil, nil),
		"lost lease killed":    shape(RunLostLease, CleanupReleased, nil, ip(9), fenced),
		"lost lease at finish": shape(RunLostLease, CleanupReleased, ip(0), nil, sp(wire.CodeTicketState)),
		"interrupted":          shape(RunInterrupted, CleanupReleased, nil, ip(9), nil),
		"spawn failed":         shape(RunSpawnFailed, CleanupNotStarted, nil, nil, nil),
		"hold":                 shape(RunCleanupHold, CleanupHold, nil, nil, nil),
		"hold after lost":      shape(RunCleanupHold, CleanupHold, nil, nil, fenced),
	} {
		if _, err := EncodeRunOutcome(o); err != nil {
			t.Errorf("%s refused: %v", name, err)
		}
	}
	early := shape(RunExit, CleanupReleased, ip(0), nil, nil)
	early.EndedAt = "2026-10-03T23:59:59Z"
	for name, o := range map[string]RunOutcome{
		"hold with released cleanup":   shape(RunCleanupHold, CleanupReleased, nil, nil, nil),
		"exit with hold cleanup":       shape(RunExit, CleanupHold, ip(0), nil, nil),
		"lost lease without code":      shape(RunLostLease, CleanupReleased, nil, ip(9), nil),
		"signal without signal":        shape(RunSignal, CleanupReleased, ip(0), nil, nil),
		"exit with signal":             shape(RunExit, CleanupReleased, nil, ip(9), nil),
		"exit with exit and signal":    shape(RunExit, CleanupReleased, ip(0), ip(9), nil),
		"exit with lost lease":         shape(RunExit, CleanupReleased, ip(0), nil, fenced),
		"negative exit code":           shape(RunExit, CleanupReleased, ip(-1), nil, nil),
		"ended before started":         early,
		"released without status":      shape(RunTimeout, CleanupReleased, nil, nil, nil),
		"hold with status":             shape(RunCleanupHold, CleanupHold, nil, ip(9), nil),
		"spawn failed with exit":       shape(RunSpawnFailed, CleanupNotStarted, ip(1), nil, nil),
		"interrupted with lost lease":  shape(RunInterrupted, CleanupReleased, nil, ip(9), fenced),
		"signal zero":                  shape(RunSignal, CleanupReleased, nil, ip(0), nil),
		"timeout with released status": shape(RunTimeout, CleanupNotStarted, nil, ip(9), nil),
	} {
		_, err := EncodeRunOutcome(o)
		if wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("%s: %v, want MALFORMED", name, err)
		}
	}
}
