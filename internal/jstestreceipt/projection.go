package jstestreceipt

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tcq"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ReceiptTestProjection carries lifecycle and identity uncertainty into every
// qualified row, including retained/MCP readers that recompute projections.
func ReceiptTestProjection(r Receipt, t TestOutcome) testvalidity.Projection {
	if r.Profile == FreshnessProfile {
		return freshTestProjection(r, t)
	}
	if IsExternalProfile(r.Profile) {
		if r.Cancelled {
			t.State = StateInterrupted
		} else if r.Infrastructure != nil || QualifiedUnknown(r, t) {
			t.State = StateInfrastructure
		}
	}
	return ToTestProjection(t)
}

// EncodeQualified emits the frozen canonical envelope used by retention and
// the strict consumer. The projections are derived, never caller-supplied.
func EncodeQualified(r Receipt) ([]byte, error) {
	if err := QualifiedProfileShapeError(r); err != nil {
		return nil, err
	}
	if r.Profile == SensitiveExternalProfile {
		if findings := ValidateSensitiveInputEvidence(r); len(findings) != 0 {
			return nil, &SensitiveInputValidationError{Findings: findings}
		}
	}
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
	if len(data) >= ExternalOutputLimit {
		return nil, errors.New("qualified-document-output-overflow")
	}
	identifiers := qualifiedIdentifiers(r)
	identifiers.Schema = SchemaNames(reflect.TypeOf(document))
	if err := ScreenQualifiedDocument(data, identifiers); err != nil {
		return nil, err
	}
	return append(data, '\n'), err
}

func QualifiedProfileShapeError(r Receipt) error {
	for _, row := range r.Tests {
		if len(row.ImportedFiles) != 0 {
			return errors.New("freshness-imports-require-separate-codec")
		}
	}
	if r.Freshness != nil || r.Profile == FreshnessProfile {
		return errors.New("freshness-requires-separate-codec")
	}
	if err := ProjectReportersShapeError(r); err != nil {
		return err
	}
	if err := RunnerProcessShapeError(r); err != nil {
		return err
	}
	if err := PreflightShapeError(r); err != nil {
		return err
	}
	if r.Profile != "" && r.Profile != AttemptExternalProfile && HasAttemptDetails(r) {
		return errors.New("external-profile-has-attempt-details")
	}
	if r.Profile != "" && hasRouteStubs(r) {
		return errors.New("external-profile-has-route-stubs")
	}
	if r.Profile != "" && hasRequestOutcomes(r) {
		return errors.New("external-profile-has-request-outcomes")
	}
	switch r.Profile {
	case AttemptExternalProfile:
		if r.ApplicationAttestation != nil || r.TestRepositoryAtStart != nil || r.TestRepositoryAtPublish != nil || HasSensitiveInputEvidence(r) {
			return errors.New("external-attempt-details-composition-unsupported")
		}
		if err := ValidateAttemptDetails(r.Tests); err != nil {
			return err
		}
	case ExternalProfile:
		if r.ApplicationAttestation != nil || r.TestRepositoryAtStart != nil || r.TestRepositoryAtPublish != nil {
			return errors.New("legacy-external-profile-has-attested-fields")
		}
		if HasSensitiveInputEvidence(r) {
			return errors.New("legacy-external-profile-has-sensitive-input-fields")
		}
	case AttestedExternalProfile:
		if r.External != nil && strings.TrimSpace(r.External.DeclaredAppIdentity) != "" {
			return errors.New("attested-external-profile-has-declared-identity")
		}
		if HasSensitiveInputEvidence(r) {
			return errors.New("attested-external-profile-has-sensitive-input-fields")
		}
	case SensitiveExternalProfile:
		if r.SensitiveInputPolicy == nil {
			return errors.New("sensitive-input-policy-required")
		}
		if r.ApplicationAttestation == nil {
			if r.TestRepositoryAtStart != nil || r.TestRepositoryAtPublish != nil {
				return errors.New("sensitive-external-profile-has-partial-attested-fields")
			}
		} else if r.External != nil && strings.TrimSpace(r.External.DeclaredAppIdentity) != "" {
			return errors.New("sensitive-attested-profile-has-declared-identity")
		}
	default:
		if r.SensitiveInputPolicy != nil {
			return errors.New("sensitive-input-policy-requires-profile-2")
		}
	}
	return bunMemberShapeError(r, runRecordProfileError(r))
}

// HasAttemptDetails reports the unprofiled-only attemptDetails member: an external profile gains a
// wire field only through another profile revision.
func HasAttemptDetails(r Receipt) bool {
	for _, test := range r.Tests {
		if len(test.AttemptDetails) != 0 {
			return true
		}
	}
	return false
}

func HasSensitiveInputEvidence(r Receipt) bool {
	if r.SensitiveInputPolicy != nil {
		return true
	}
	for _, test := range r.Tests {
		for _, attempt := range test.Attempts {
			if len(attempt.Steps) != 0 {
				return true
			}
		}
	}
	return false
}

// TcqStatus maps a per-attempt execution state onto the TCQ report vocabulary
// that the shared flake rule (tcq.Flaky, TCQ-V0-049) judges. Harness states
// that produced no test result map to error or skipped; a state outside the
// table maps to "" and is ignored by the rule.
var TcqStatus = map[ExecutionState]string{
	StatePassed:         tcq.ReportPassed,
	StateFailed:         tcq.ReportFailed,
	StateTimedOut:       tcq.ReportError,
	StateInfrastructure: tcq.ReportError,
	StateSkipped:        tcq.ReportSkipped,
	StateInterrupted:    tcq.ReportSkipped,
}

// FlakyOutcome applies the shared TCQ-V0-049 rule to the recorded attempts. A
// reporter's own flaky label stands on its own, so missing attempt evidence
// never upgrades a labelled flake to a clean pass; recorded attempts can only
// add the qualification.
func FlakyOutcome(outcome TestOutcome) bool {
	statuses := make([]string, 0, len(outcome.Attempts))
	for _, attempt := range outcome.Attempts {
		statuses = append(statuses, TcqStatus[attempt.State])
	}
	return outcome.State == StateFlaky || tcq.Flaky(statuses)
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
		if FlakyOutcome(outcome) {
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
	if r.Profile == FreshnessProfile {
		currency := testvalidity.FreshnessCurrent
		if len(r.Tests) == 0 {
			currency = FreshnessCurrency(r, TestOutcome{})
		}
		for _, t := range r.Tests {
			observed := FreshnessCurrency(r, t)
			if observed == testvalidity.FreshnessStale {
				currency = observed
			} else if observed != testvalidity.FreshnessCurrent && currency != testvalidity.FreshnessStale {
				currency = testvalidity.FreshnessUnknown
			}
		}
		facts := testvalidity.ExecutionFacts{Currency: currency}
		if r.Cancelled {
			facts.Outcome = "INCOMPLETE"
			facts.Cause = "CANCELLATION"
		} else if r.Infrastructure != nil {
			facts.Outcome = "INCOMPLETE"
			facts.Cause = "INFRASTRUCTURE"
		}
		return testvalidity.Project(testvalidity.Input{Execution: &facts})
	}

	var execution *testvalidity.ExecutionFacts
	switch {
	case r.Cancelled:
		execution = &testvalidity.ExecutionFacts{Outcome: "INCOMPLETE", Cause: "CANCELLATION"}
	case r.Infrastructure != nil:
		execution = &testvalidity.ExecutionFacts{Outcome: "INCOMPLETE", Cause: "INFRASTRUCTURE", Anchors: preflightAnchors(r.Infrastructure)}
	}
	if execution == nil && !r.StaleAppBuild {
		return testvalidity.Project(testvalidity.Input{})
	}
	if execution == nil {
		execution = &testvalidity.ExecutionFacts{}
	}
	if r.StaleAppBuild {
		execution.Currency = "STALE"
	} else if execution.Outcome != "" && !IsExternalProfile(r.Profile) {
		execution.Currency = "CURRENT"
	}
	return testvalidity.Project(testvalidity.Input{Execution: execution})
}
