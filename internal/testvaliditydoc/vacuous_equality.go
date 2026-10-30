package testvaliditydoc

import "github.com/Beamfall/corvint/internal/jstestprovider"

// Run-time vacuous-equality states, reasons and code (LPCV-V0-121, LPCV-V0-122).
const (
	VacuousRecorded      = "RECORDED"
	VacuousNotApplicable = "NOT_APPLICABLE"
	VacuousNotProduced   = "NOT_PRODUCED"

	VacuousRecordAbsent  = "assertion-operand-record-absent"
	VacuousRecordInvalid = "assertion-operand-record-invalid"
	VacuousTestNotPassed = "test-not-passed"

	// VacuousEmptyEquality is the run-time code of the static TSA-V0-172 rule: an evaluated count
	// or equality assertion whose two operands were both empty or zero.
	VacuousEmptyEquality = "VACUOUS-EMPTY-EQUALITY"
)

// VacuousEquality states whether a passed test's count or equality assertions held only because
// both operands were empty (LPCV-V0-121). It is not an axis: it never rewrites the five-axis
// projection. BlocksCredit marks a test that no consumer may credit as a witness (LPCV-V0-122).
type VacuousEquality struct {
	State        string                   `json:"state"`
	Reason       string                   `json:"reason,omitempty"`
	BlocksCredit bool                     `json:"blocksCredit,omitempty"`
	Findings     []VacuousEqualityFinding `json:"findings,omitempty"`
}

// VacuousEqualityFinding names one assertion whose actual and expected operands were both empty.
type VacuousEqualityFinding struct {
	Code    string `json:"code"`
	Matcher string `json:"matcher"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// vacuousEqualityEvidence returns each test's run-time vacuous-equality reading. It returns nil when
// the receipt is profiled or no attempt carries an assertion-operand record, so such a document
// keeps every byte (LPCV-V0-121).
func vacuousEqualityEvidence(receipt jstestprovider.Receipt) []*VacuousEquality {
	if receipt.Profile != "" || !anyAssertionOperandRecord(receipt) {
		return nil
	}
	evidence := make([]*VacuousEquality, len(receipt.Tests))
	for i, outcome := range receipt.Tests {
		evidence[i] = testVacuousEquality(outcome)
	}
	return evidence
}

func anyAssertionOperandRecord(receipt jstestprovider.Receipt) bool {
	for _, outcome := range receipt.Tests {
		for _, detail := range outcome.AttemptDetails {
			if detail.AssertionOperands != nil {
				return true
			}
		}
	}
	return false
}

// testVacuousEquality reads the last attempt, whose outcome is the test's. Operands are read only
// for a passed test: a failed test is never a witness, and an operand record that is absent or
// invalid is never read as empty.
func testVacuousEquality(outcome jstestprovider.TestOutcome) *VacuousEquality {
	if len(outcome.AttemptDetails) == 0 {
		return &VacuousEquality{State: VacuousNotProduced, Reason: VacuousRecordAbsent}
	}
	record := outcome.AttemptDetails[len(outcome.AttemptDetails)-1].AssertionOperands
	switch {
	case record == nil:
		return &VacuousEquality{State: VacuousNotProduced, Reason: VacuousRecordAbsent}
	case !jstestprovider.ValidAssertionOperandRecord(*record):
		return &VacuousEquality{State: VacuousNotProduced, Reason: VacuousRecordInvalid}
	case !testPassed(outcome.State):
		return &VacuousEquality{State: VacuousNotApplicable, Reason: VacuousTestNotPassed}
	}
	screened := jstestprovider.ScreenAssertionOperandRecord(*record)
	var findings []VacuousEqualityFinding
	for _, assertion := range screened.Assertions {
		if assertion.Actual == 0 && assertion.Expected == 0 {
			findings = append(findings, VacuousEqualityFinding{Code: VacuousEmptyEquality, Matcher: assertion.Matcher, File: assertion.File, Line: assertion.Line})
		}
	}
	return &VacuousEquality{State: VacuousRecorded, BlocksCredit: len(findings) > 0, Findings: findings}
}
