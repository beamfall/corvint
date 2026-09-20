package jstestprovider

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tcq"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ReceiptTestProjection carries lifecycle and identity uncertainty into every
// qualified row, including retained/MCP readers that recompute projections.
func ReceiptTestProjection(r Receipt, t TestOutcome) testvalidity.Projection {
	if r.Profile == ExternalProfile {
		if r.Cancelled {
			t.State = StateInterrupted
		} else if r.Infrastructure != nil || qualifiedUnknown(r, t) {
			t.State = StateInfrastructure
		}
	}
	return ToTestProjection(t)
}

// EncodeQualified emits the frozen canonical envelope used by retention and
// the strict consumer. The projections are derived, never caller-supplied.
func EncodeQualified(r Receipt) ([]byte, error) {
	type row struct {
		Name       string                  `json:"name"`
		State      ExecutionState          `json:"state"`
		Projection testvalidity.Projection `json:"projection"`
	}
	tests := make([]row, 0, len(r.Tests))
	for _, t := range r.Tests {
		tests = append(tests, row{t.Name, t.State, ReceiptTestProjection(r, t)})
	}
	document := struct {
		Receipt Receipt                 `json:"receipt"`
		Tests   []row                   `json:"testProjections"`
		Run     testvalidity.Projection `json:"runProjection"`
	}{r, tests, ReceiptRunProjection(r)}
	data, err := json.Marshal(document)
	if len(data) >= externalOutputLimit {
		return nil, errors.New("qualified-document-output-overflow")
	}
	if secretscreen.MatchString(string(data)) {
		return nil, errors.New("qualified-document-secret-shaped")
	}
	return append(data, '\n'), err
}

// ToTestProjection projects one TestOutcome through the shared
// testvalidity.Project (internal/testvalidity/projection.go). Ordinary
// per-test states (passed/failed/skipped/flaky) carry ClaimFacts, since they
// are TCQ-shaped per-test report rows with a known association anchor.
// Harness-level incomplete states (timedOut/interrupted/infrastructure) that
// mean the test itself never produced a real report instead carry
// ExecutionFacts with an INCOMPLETE outcome, matching the LPCV/GLTP cause
// vocabulary Project already understands. No branch invents a "valid"
// verdict; every branch states exactly the fact this provider observed.
func ToTestProjection(outcome TestOutcome) testvalidity.Projection {
	anchors := anchorStrings(outcome.Anchor)
	switch outcome.State {
	case StatePassed, StateFailed, StateSkipped, StateFlaky:
		claim := testvalidity.ClaimFacts{
			AssociationState: tcq.AssociationAssociated,
			HygieneState:     tcq.HygieneEligible,
			ReportState:      reportStateFor(outcome.State),
			Anchors:          anchors,
		}
		if outcome.State == StateFlaky {
			claim.Reasons = []string{"flaky-retry"}
		}
		return testvalidity.Project(testvalidity.Input{Claim: &claim})
	case StateTimedOut, StateInterrupted, StateInfrastructure:
		execution := testvalidity.ExecutionFacts{
			Outcome: "INCOMPLETE",
			Cause:   causeFor(outcome.State),
			Anchors: anchors,
		}
		return testvalidity.Project(testvalidity.Input{Execution: &execution})
	default:
		return testvalidity.Project(testvalidity.Input{})
	}
}

func reportStateFor(state ExecutionState) string {
	switch state {
	case StatePassed, StateFlaky:
		return tcq.ReportPassed
	case StateFailed:
		return tcq.ReportFailed
	case StateSkipped:
		return tcq.ReportSkipped
	default:
		return tcq.ReportError
	}
}

func causeFor(state ExecutionState) string {
	switch state {
	case StateTimedOut:
		return "TIMEOUT"
	case StateInterrupted:
		return "CANCELLATION"
	case StateInfrastructure:
		return "INFRASTRUCTURE"
	default:
		return ""
	}
}

func anchorStrings(a *Anchor) []string {
	if a == nil || a.File == "" {
		return nil
	}
	return []string{anchorString(*a)}
}

func anchorString(a Anchor) string {
	if a.Line == 0 {
		return a.File
	}
	return a.File + ":" + strconv.Itoa(a.Line)
}

// ReceiptRunProjection projects the receipt's own run-level facts: a
// run-level InfrastructureFailure (bad command, no tests found, missing
// browser at collection time) and stale-app-build freshness. It is separate
// from per-test projections because a run-level infrastructure failure may
// exist with zero test outcomes, and staleness is a fact about the whole
// run's served build, not any one test.
func ReceiptRunProjection(r Receipt) testvalidity.Projection {
	var execution *testvalidity.ExecutionFacts
	switch {
	case r.Cancelled:
		execution = &testvalidity.ExecutionFacts{Outcome: "INCOMPLETE", Cause: "CANCELLATION"}
	case r.Infrastructure != nil:
		execution = &testvalidity.ExecutionFacts{Outcome: "INCOMPLETE", Cause: "INFRASTRUCTURE"}
	}
	if execution == nil && !r.StaleAppBuild {
		return testvalidity.Project(testvalidity.Input{})
	}
	if execution == nil {
		execution = &testvalidity.ExecutionFacts{}
	}
	if r.StaleAppBuild {
		execution.Currency = "STALE"
	} else if execution.Outcome != "" && r.Profile != ExternalProfile {
		execution.Currency = "CURRENT"
	}
	return testvalidity.Project(testvalidity.Input{Execution: execution})
}
