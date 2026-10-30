package transaction

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// QualificationPackage and QualificationSuite name the CAL-V0-019 suite a
// QUALIFICATION run must pass: every test passes and none fails.
const QualificationPackage = "github.com/Beamfall/corvint/internal/tasks/store"

var QualificationSuite = []string{
	"TestCALV0019_ConcurrentCollidingClaimsAdmitOne",
	"TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead",
	"TestCALV0019_FencedGenerationCannotMoveOrComplete",
	"TestCALV0019_FaultAtEveryArtifactIsAllOrNothing",
	"TestCALV0019_KilledWriterRecovers",
}

// testEvent is the part of a `go test -json` event the verdict reads.
type testEvent struct {
	Action, Package, Test string
}

// qualificationPosts plans the execution cutover (CAL-V0-020). The run is
// `go test -json` output; it is posted as evidence under its own digest, which
// the queue's executionCutover names in gateEvidence.
func qualificationPosts(r Request, in Input, state inputState, posts map[string][]byte) *Result {
	if r.Actor.Role != "OWNER" {
		result := refused(r.RequestID, mutation.OutcomeUnauthorized, "", "an execution cutover needs an OWNER binding")
		return &result
	}
	if state.queue.Fixture {
		result := refused(r.RequestID, mutation.OutcomeBlocked, "", "a fixture queue keeps mode DEVELOPMENT and takes no execution cutover")
		return &result
	}
	if state.queue.CanonicalWriter != "NATIVE" {
		result := refused(r.RequestID, mutation.OutcomeBlocked, wire.CodeCutoverMissing, "the authority switch precedes the execution cutover")
		return &result
	}
	if state.queue.ExecutionCutover != nil {
		result := refused(r.RequestID, mutation.OutcomeBlocked, "", "executionCutover is already recorded")
		return &result
	}
	if refusal := runVerdict(r.RequestID, r.File); refusal != nil {
		return refusal
	}
	digest := wire.Sum(r.File)
	next := *state.queue
	next.ExecutionCutover = &intent.ExecutionCutover{EnabledBy: r.Actor.ID, DecisionRef: r.RequestID, GateEvidence: []wire.Digest{digest}}
	posts["intent/queue.json"] = wire.EncodeFile(next.Value())
	posts["evidence/"+string(digest)] = bytes.Clone(r.File)
	return nil
}

// runVerdict requires the suite's complete package run, including each test's
// run/pass pair and the final package pass. Evidence remains owner supplied.
func runVerdict(requestID string, run []byte) *Result {
	started, finished := false, false
	passed := map[string]bool{}
	running := map[string]bool{}
	for _, line := range bytes.Split(run, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ev, ok := qualificationEvent(line)
		if !ok {
			result := failed(requestID, malformed("qualification run is not go test -json output"))
			return &result
		}
		if ev.Action == "fail" || ev.Action == "build-fail" {
			result := refused(requestID, mutation.OutcomeBlocked, wire.CodeGateFailed, "qualification run has a failure: "+ev.Package+" "+ev.Test)
			return &result
		}
		if ev.Package != QualificationPackage {
			continue
		}
		if finished || (!started && ev.Action != "start") {
			result := refused(requestID, mutation.OutcomeBlocked, wire.CodeMissingEvidence, "qualification run lacks a complete package lifecycle")
			return &result
		}
		switch ev.Action {
		case "start":
			if started || ev.Test != "" {
				result := failed(requestID, malformed("qualification package start is repeated or names a test"))
				return &result
			}
			started = true
		case "run":
			running[ev.Test], passed[ev.Test] = true, false
		case "pass":
			if ev.Test == "" {
				finished = true
			} else {
				passed[ev.Test], running[ev.Test] = running[ev.Test], false
			}
		case "skip":
			passed[ev.Test], running[ev.Test] = false, false
		}
	}
	for _, name := range QualificationSuite {
		if !passed[name] {
			result := refused(requestID, mutation.OutcomeBlocked, wire.CodeMissingEvidence, "qualification run lacks a passing "+name)
			return &result
		}
	}
	if !finished {
		result := refused(requestID, mutation.OutcomeBlocked, wire.CodeMissingEvidence, "qualification run lacks the final package pass")
		return &result
	}
	return nil
}

// Go test events are flat objects. Decode their exact keys so duplicate or
// case-folded Action fields cannot hide a failure behind a later pass.
func qualificationEvent(line []byte) (testEvent, bool) {
	var ev testEvent
	var importPath string
	d := json.NewDecoder(bytes.NewReader(line))
	if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
		return ev, false
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return ev, false
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return ev, false
		}
		seen[name] = true
		value, err := d.Token()
		if err != nil {
			return ev, false
		}
		if name == "Elapsed" {
			if n, ok := value.(float64); !ok || n < 0 {
				return ev, false
			}
			continue
		}
		text, ok := value.(string)
		if !ok {
			return ev, false
		}
		switch name {
		case "Action":
			ev.Action = text
		case "Package":
			ev.Package = text
		case "Test":
			ev.Test = text
		case "ImportPath":
			importPath = text
		case "Time", "Output", "OutputType", "FailedBuild":
		default:
			return ev, false
		}
	}
	if tok, err := d.Token(); err != nil || tok != json.Delim('}') {
		return ev, false
	}
	if (ev.Action == "build-output" || ev.Action == "build-fail") && ev.Package == "" {
		ev.Package = importPath
	}
	if _, err := d.Token(); err != io.EOF || ev.Package == "" {
		return ev, false
	}
	switch ev.Action {
	case "start", "pass", "fail", "skip", "output", "build-output", "build-fail":
		return ev, true
	case "run", "pause", "cont", "bench":
		return ev, ev.Test != ""
	default:
		return ev, false
	}
}
