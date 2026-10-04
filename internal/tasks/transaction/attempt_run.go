package transaction

import (
	"bytes"
	"encoding/json"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LeaseRunOutcome records one attempt-runner command outcome (ATR-V0-005).
// It is an observation: it posts only the outcome document under evidence/
// and never changes the attempt, its lease or the reservation set.
const LeaseRunOutcome = "RUN_OUTCOME"

// RunOutcomeProfile names the outcome document a RUN_OUTCOME posts.
const RunOutcomeProfile = "taskman-attempt-run-outcome/0"

// MaxRunOutcomeBytes bounds one outcome document.
const MaxRunOutcomeBytes = 4096

// Run outcome classes (ATR-V0-004).
const (
	RunExit        = "EXIT"
	RunSignal      = "SIGNAL"
	RunTimeout     = "TIMEOUT"
	RunLostLease   = "LOST_LEASE"
	RunInterrupted = "INTERRUPTED"
	RunSpawnFailed = "SPAWN_FAILED"
	RunCleanupHold = "CLEANUP_HOLD"
)

// Cleanup states of the owned process group (ATR-V0-003).
const (
	CleanupNotStarted = "NOT_STARTED"
	CleanupReleased   = "RELEASED"
	CleanupHold       = "HOLD"
)

// RunOutcome is the closed outcome document of one `corvint-tasks run
// --attempt` invocation. It carries the argv digest, never the argv.
type RunOutcome struct {
	Profile        string  `json:"profile"`
	RunID          string  `json:"runId"`
	AttemptID      string  `json:"attemptId"`
	Generation     string  `json:"generation"`
	ArgvSha256     string  `json:"argvSha256"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
	StartedAt      *string `json:"startedAt"`
	EndedAt        string  `json:"endedAt"`
	Class          string  `json:"class"`
	ExitCode       *int    `json:"exitCode"`
	Signal         *int    `json:"signal"`
	Cleanup        string  `json:"cleanup"`
	Heartbeats     int     `json:"heartbeats"`
	Renewals       int     `json:"renewals"`
	LostLease      *string `json:"lostLease"`
}

var runClasses = map[string]bool{RunExit: true, RunSignal: true, RunTimeout: true, RunLostLease: true, RunInterrupted: true, RunSpawnFailed: true, RunCleanupHold: true}

var cleanupStates = map[string]bool{CleanupNotStarted: true, CleanupReleased: true, CleanupHold: true}

// EncodeRunOutcome is the canonical encoding: compact JSON with one
// trailing LF.
func EncodeRunOutcome(o RunOutcome) ([]byte, error) {
	raw, e := json.Marshal(o)
	if e != nil {
		return nil, e
	}
	raw = append(raw, '\n')
	if _, e = DecodeRunOutcome(raw); e != nil {
		return nil, e
	}
	return raw, nil
}

// DecodeRunOutcome accepts only a canonical, closed, internally consistent
// outcome document.
func DecodeRunOutcome(raw []byte) (*RunOutcome, error) {
	if len(raw) > MaxRunOutcomeBytes {
		return nil, limit("run outcome larger than 4096 bytes")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var o RunOutcome
	if e := d.Decode(&o); e != nil {
		return nil, malformed("run outcome: " + e.Error())
	}
	again, e := json.Marshal(o)
	if e != nil || !bytes.Equal(append(again, '\n'), raw) {
		return nil, malformed("run outcome is not canonical")
	}
	if o.Profile != RunOutcomeProfile || !runClasses[o.Class] || !cleanupStates[o.Cleanup] {
		return nil, malformed("run outcome profile, class or cleanup is not in the closed set")
	}
	if _, e = wire.ParseIdentifier("runId", o.RunID); e != nil {
		return nil, e
	}
	if _, e = wire.ParseDigest("argvSha256", o.ArgvSha256); e != nil {
		return nil, e
	}
	if _, e = wire.ParseSize("generation", o.Generation); e != nil {
		return nil, e
	}
	if _, e = wire.ParseTimestamp("endedAt", o.EndedAt); e != nil {
		return nil, e
	}
	if o.StartedAt != nil {
		if _, e = wire.ParseTimestamp("startedAt", *o.StartedAt); e != nil {
			return nil, e
		}
	}
	if o.LostLease != nil && !wire.IsCode(*o.LostLease) {
		return nil, malformed("run outcome lostLease is not a closed code")
	}
	started := o.StartedAt != nil
	if started == (o.Class == RunSpawnFailed) || started == (o.Cleanup == CleanupNotStarted) {
		return nil, malformed("run outcome start, class and cleanup disagree")
	}
	if o.TimeoutSeconds < 1 || o.Heartbeats < 0 || o.Renewals < 0 || o.ExitCode != nil && o.Signal != nil {
		return nil, malformed("run outcome counters or status are out of range")
	}
	return &o, nil
}

// planRunOutcome posts one outcome document for an attempt generation that
// has existed. It does not require a live or unexpired lease, so the outcome
// of a fenced run is retained, and it renews nothing (ATR-V0-005).
func planRunOutcome(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	g := c.l.Generation.Uint64()
	if g == 0 || g > a.Generation.Uint64() {
		return c.fail(malformed("run outcome names a generation the attempt never had"))
	}
	raw := c.in.LeaseFacts.RunOutcome
	if string(wire.Sum(raw)) != c.l.Evidence {
		return c.fail(malformed("run outcome binding"))
	}
	o, e := DecodeRunOutcome(raw)
	if e != nil {
		return c.fail(e)
	}
	if o.AttemptID != a.AttemptID || o.Generation != string(c.l.Generation) {
		return c.fail(malformed("run outcome names another attempt generation"))
	}
	posts := map[string][]byte{"evidence/" + c.l.Evidence: bytes.Clone(raw)}
	eff := &leaseEffect{kind: "TRANSITION", attemptID: a.AttemptID, generation: c.l.Generation, outcome: mutation.OutcomeCompleted, codes: []string{}}
	return leaseOutcome{posts: posts, effect: eff, detail: "run outcome " + o.Class}
}
