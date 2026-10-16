package testvaliditydoc

import "github.com/Beamfall/corvint/internal/jstestprovider"

// Route-stub states and codes (LPCV-V0-073).
const (
	RouteStubsRecorded    = "RECORDED"
	RouteStubsNotProduced = "NOT_PRODUCED"
	// StubNeverMatched names a request stub whose handler never ran in a passing attempt.
	StubNeverMatched = "stub-never-matched"
)

// RouteStubs is one test's request-stub evidence from its last attempt. It is not an axis: it
// never rewrites the five-axis projection.
type RouteStubs struct {
	State    string             `json:"state"`
	Reason   string             `json:"reason,omitempty"`
	Findings []RouteStubFinding `json:"findings,omitempty"`
}

// RouteStubFinding names one registered stub and where it was registered.
type RouteStubFinding struct {
	Code    string `json:"code"`
	Target  string `json:"target"`
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// routeStubEvidence returns each test's route-stub evidence, or nil when no attempt of an
// unprofiled receipt carries a record, so a receipt without the producer keeps every byte.
func routeStubEvidence(receipt jstestprovider.Receipt) []*RouteStubs {
	if receipt.Profile != "" || !anyRouteStubRecord(receipt) {
		return nil
	}
	evidence := make([]*RouteStubs, len(receipt.Tests))
	for i, outcome := range receipt.Tests {
		evidence[i] = testRouteStubs(outcome)
	}
	return evidence
}

func anyRouteStubRecord(receipt jstestprovider.Receipt) bool {
	for _, outcome := range receipt.Tests {
		for _, detail := range outcome.AttemptDetails {
			if detail.RouteStubs != nil {
				return true
			}
		}
	}
	return false
}

// testRouteStubs reads the last attempt, whose outcome is the test's: a stub hit in an earlier,
// failed attempt says nothing about the attempt that passed.
func testRouteStubs(outcome jstestprovider.TestOutcome) *RouteStubs {
	if len(outcome.AttemptDetails) == 0 {
		return &RouteStubs{State: RouteStubsNotProduced, Reason: "route-stub-record-absent"}
	}
	last := outcome.AttemptDetails[len(outcome.AttemptDetails)-1]
	if last.RouteStubs == nil {
		return &RouteStubs{State: RouteStubsNotProduced, Reason: "route-stub-record-absent"}
	}
	if !jstestprovider.ValidRouteStubRecord(*last.RouteStubs) {
		return &RouteStubs{State: RouteStubsNotProduced, Reason: "route-stub-record-invalid"}
	}
	if last.State != jstestprovider.StatePassed {
		return &RouteStubs{State: RouteStubsRecorded, Reason: "test-not-passed"}
	}
	record := jstestprovider.ScreenRouteStubRecord(*last.RouteStubs)
	var findings []RouteStubFinding
	for _, stub := range record.Stubs {
		if stub.Hits == 0 {
			findings = append(findings, RouteStubFinding{Code: StubNeverMatched, Target: stub.Target, Kind: stub.Kind, Pattern: stub.Pattern, File: stub.File, Line: stub.Line})
		}
	}
	return &RouteStubs{State: RouteStubsRecorded, Findings: findings}
}
