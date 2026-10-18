package testvaliditydoc

import (
	"sort"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// Rejection-anchor states (LPCV-V0-081, LPCV-V0-082).
const (
	AnchorAnchored      = "ANCHORED"
	AnchorUnanchored    = "UNANCHORED"
	AnchorFailed        = "ANCHOR-FAILED"
	AnchorUnknown       = "UNKNOWN"
	AnchorNotApplicable = "NOT_APPLICABLE"
	AnchorNotProduced   = "NOT_PRODUCED"
)

// Rejection-anchor reasons (LPCV-V0-081, LPCV-V0-082).
const (
	AnchorRecordAbsent           = "request-outcome-record-absent"
	AnchorRecordInvalid          = "request-outcome-record-invalid"
	AnchorNotRejectionOnly       = "not-rejection-only"
	AnchorTestNotPassed          = "test-not-passed"
	AnchorRouteEvidenceMissing   = "anchor-route-evidence-missing"
	AnchorActorEvidenceMissing   = "anchor-actor-evidence-missing"
	AnchorDeclaredSiblingFailed  = "declared-positive-sibling-failed"
	AnchorObservedSiblingFailed  = "positive-sibling-failed"
	AnchorDeclaredSiblingPassed  = "declared-positive-sibling-passed"
	AnchorSuccessOnRoute         = "success-on-route"
	AnchorNoSuccessOnRoute       = "no-success-on-route"
	AnchorDeclaredSiblingMissing = "declared-positive-sibling-not-completed"
)

// RejectionAnchor states whether a rejection-only test's rejection is anchored by evidence, in the
// same run, that the same method and route template can succeed for the same actor (LPCV-V0-081).
// It is not an axis: it never rewrites the five-axis projection. BlocksCredit marks an
// ANCHOR-FAILED test that no consumer may credit as a witness (LPCV-V0-083). Siblings names, by
// full title, the tests whose evidence decided the state.
type RejectionAnchor struct {
	State        string   `json:"state"`
	Reason       string   `json:"reason"`
	BlocksCredit bool     `json:"blocksCredit,omitempty"`
	Siblings     []string `json:"siblings,omitempty"`
}

type anchorRoute struct {
	method string
	route  string
}

// anchorTest is one receipt test's last-attempt record, or nil when the last attempt carries none.
type anchorTest struct {
	outcome jstestprovider.TestOutcome
	record  *jstestprovider.RequestOutcomeRecord
	valid   bool
}

// rejectionAnchorEvidence returns each test's rejection anchor. It returns nil when no attempt of
// the receipt carries a request-outcome record, or the receipt is profiled, so such a document
// keeps every byte (LPCV-V0-081).
func rejectionAnchorEvidence(receipt jstestprovider.Receipt) []*RejectionAnchor {
	if receipt.Profile != "" || !anyRequestOutcomeRecord(receipt) {
		return nil
	}
	tests := make([]anchorTest, len(receipt.Tests))
	for i, outcome := range receipt.Tests {
		tests[i] = anchorTest{outcome: outcome}
		if len(outcome.AttemptDetails) == 0 {
			continue
		}
		record := outcome.AttemptDetails[len(outcome.AttemptDetails)-1].RequestOutcomes
		if record != nil {
			tests[i].record = record
			tests[i].valid = jstestprovider.ValidRequestOutcomeRecord(*record)
		}
	}
	evidence := make([]*RejectionAnchor, len(tests))
	for i := range tests {
		evidence[i] = rejectionAnchor(tests, i)
	}
	return evidence
}

func anyRequestOutcomeRecord(receipt jstestprovider.Receipt) bool {
	for _, outcome := range receipt.Tests {
		for _, detail := range outcome.AttemptDetails {
			if detail.RequestOutcomes != nil {
				return true
			}
		}
	}
	return false
}

func testPassed(state jstestprovider.ExecutionState) bool {
	return state == jstestprovider.StatePassed || state == jstestprovider.StateFlaky
}

func testFailed(state jstestprovider.ExecutionState) bool {
	return state == jstestprovider.StateFailed || state == jstestprovider.StateTimedOut
}

func testTitle(outcome jstestprovider.TestOutcome) string {
	if outcome.FullName != "" {
		return outcome.FullName
	}
	return outcome.Name
}

// rejectionAnchor classifies one test from its last attempt (LPCV-V0-081, LPCV-V0-082). Evidence
// that is missing never anchors: a rejection-only record without a route or actor is UNKNOWN, and a
// sibling record without the same actor is not a candidate.
func rejectionAnchor(tests []anchorTest, self int) *RejectionAnchor {
	test := tests[self]
	switch {
	case test.record == nil:
		return &RejectionAnchor{State: AnchorNotProduced, Reason: AnchorRecordAbsent}
	case !test.valid:
		return &RejectionAnchor{State: AnchorNotProduced, Reason: AnchorRecordInvalid}
	case test.record.Assertion != jstestprovider.AssertionRejectionOnly:
		return &RejectionAnchor{State: AnchorNotApplicable, Reason: AnchorNotRejectionOnly}
	case !testPassed(test.outcome.State):
		return &RejectionAnchor{State: AnchorNotApplicable, Reason: AnchorTestNotPassed}
	case len(test.record.Requests) == 0:
		return &RejectionAnchor{State: AnchorUnknown, Reason: AnchorRouteEvidenceMissing}
	case test.record.Actor == "":
		return &RejectionAnchor{State: AnchorUnknown, Reason: AnchorActorEvidenceMissing}
	}
	routes := map[anchorRoute]bool{}
	for _, request := range test.record.Requests {
		routes[anchorRoute{request.Method, request.Route}] = true
	}
	failed := map[string]bool{}
	succeeded := map[anchorRoute][]string{}
	for i, other := range tests {
		if i == self || other.record == nil || !other.valid || other.record.Actor != test.record.Actor {
			continue
		}
		for _, request := range other.record.Requests {
			key := anchorRoute{request.Method, request.Route}
			if !routes[key] {
				continue
			}
			if other.record.Assertion == jstestprovider.AssertionPositive && testFailed(other.outcome.State) {
				failed[testTitle(other.outcome)] = true
			}
			if jstestprovider.IsSuccessStatus(request.Status) {
				succeeded[key] = append(succeeded[key], testTitle(other.outcome))
			}
		}
	}
	declared, declaredState := declaredSibling(tests, self)
	if declaredState == AnchorFailed || len(failed) != 0 {
		reason := AnchorObservedSiblingFailed
		if declaredState == AnchorFailed {
			reason = AnchorDeclaredSiblingFailed
			for _, title := range declared {
				failed[title] = true
			}
		}
		return &RejectionAnchor{State: AnchorFailed, Reason: reason, BlocksCredit: true, Siblings: sortedTitles(failed)}
	}
	if declaredState == AnchorAnchored {
		return &RejectionAnchor{State: AnchorAnchored, Reason: AnchorDeclaredSiblingPassed, Siblings: sortedTitles(titleSet(declared))}
	}
	anchors := map[string]bool{}
	for key := range routes {
		if len(succeeded[key]) == 0 {
			reason := AnchorNoSuccessOnRoute
			if test.record.PositiveSibling != "" {
				reason = AnchorDeclaredSiblingMissing
			}
			return &RejectionAnchor{State: AnchorUnanchored, Reason: reason}
		}
		for _, title := range succeeded[key] {
			anchors[title] = true
		}
	}
	return &RejectionAnchor{State: AnchorAnchored, Reason: AnchorSuccessOnRoute, Siblings: sortedTitles(anchors)}
}

// declaredSibling resolves the record's positiveSibling against the other tests of the run, by full
// title or name and, when the test has a project, within that project. Any failed match is
// ANCHOR-FAILED, else any passed match is ANCHORED; no completed match leaves it undecided ("").
func declaredSibling(tests []anchorTest, self int) ([]string, string) {
	test := tests[self]
	name := test.record.PositiveSibling
	if name == "" {
		return nil, ""
	}
	var failed, passed []string
	for i, other := range tests {
		if i == self || (other.outcome.FullName != name && other.outcome.Name != name) {
			continue
		}
		if test.outcome.Project != nil && (other.outcome.Project == nil || other.outcome.Project.Name != test.outcome.Project.Name) {
			continue
		}
		switch {
		case testFailed(other.outcome.State):
			failed = append(failed, testTitle(other.outcome))
		case testPassed(other.outcome.State):
			passed = append(passed, testTitle(other.outcome))
		}
	}
	if len(failed) != 0 {
		return failed, AnchorFailed
	}
	if len(passed) != 0 {
		return passed, AnchorAnchored
	}
	return nil, ""
}

func titleSet(titles []string) map[string]bool {
	set := make(map[string]bool, len(titles))
	for _, title := range titles {
		set[title] = true
	}
	return set
}

func sortedTitles(set map[string]bool) []string {
	titles := make([]string, 0, len(set))
	for title := range set {
		titles = append(titles, title)
	}
	sort.Strings(titles)
	return titles
}
