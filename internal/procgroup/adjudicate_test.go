package procgroup

import "testing"

func completedObservation() Observation {
	return Observation{Started: true, ExitObserved: true, PipesDrained: true, OwnedProcessGroupCleanup: true}
}
func TestAdjudicateRefusalsAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		refusal PrelaunchRefusal
		state   CaseInconclusiveState
		reason  string
	}{
		{RefusalNone, InconclusiveSetup, ReasonNoLaunchObserved},
		{RefusalMissingSandbox, InconclusiveSandbox, ReasonMissingSandbox},
		{RefusalDescriptorInvalid, InconclusiveSetup, ReasonDescriptorInvalid},
		{RefusalDigestMismatch, InconclusiveSetup, ReasonDigestMismatch},
		{RefusalFixtureNotRegistered, InconclusiveSetup, ReasonFixtureNotRegistered},
		{RefusalResourceBound, InconclusiveResource, ReasonResourceBound},
		{RefusalBudgetExpired, InconclusiveTimeout, ReasonBudgetExpired},
	} {
		t.Run(string(tc.refusal), func(t *testing.T) {
			a := Adjudicate(6, tc.refusal, nil)
			if a.Status != StatusNotRun || a.Outcome != OutcomeWithheld || a.CaseInconclusiveState != tc.state || a.Reason != tc.reason || a.RecordCount != 0 || a.Launches != 0 || a.UnobservedLaunches != 0 || a.FullyObserved {
				t.Fatalf("%+v", a)
			}
		})
	}
}
func TestAdjudicateObservationTruthTable(t *testing.T) {
	done := completedObservation()
	overflow := done
	overflow.StderrOverflow = true
	timeout := done
	timeout.TimedOut = true
	unobserved := done
	unobserved.ExitObserved = false
	undrained := done
	undrained.PipesDrained = false
	unclean := done
	unclean.OwnedProcessGroupCleanup = false
	cancelled := Observation{Cancelled: true}
	cases := []struct {
		name                          string
		obs                           []Observation
		state                         CaseInconclusiveState
		status                        Status
		records, launches, unobserved int
		full                          bool
	}{
		{"fully-observed", []Observation{done, done, done, done, done, done}, InconclusiveNone, "", 6, 6, 0, true},
		{"started-unobserved", []Observation{unobserved, unobserved, unobserved, unobserved, unobserved, unobserved}, InconclusiveInconsistent, StatusFail, 0, 6, 6, false},
		{"mixed-counts", []Observation{done, done, unobserved, {}, {}, {}}, InconclusiveSetup, StatusFail, 2, 3, 1, false},
		{"planned-not-slice-length", []Observation{done, done, done}, InconclusiveInconsistent, StatusFail, 3, 3, 0, false},
		{"overflow-beats-other-setup", []Observation{overflow, {}}, InconclusiveResource, StatusFail, 1, 1, 0, false},
		{"timeout-beats-other-setup", []Observation{timeout, {}}, InconclusiveTimeout, StatusFail, 1, 1, 0, false},
		{"overflow-beats-timeout", []Observation{timeout, overflow}, InconclusiveResource, StatusFail, 2, 2, 0, false},
		{"later-start-failure", []Observation{done, {}}, InconclusiveSetup, StatusFail, 1, 1, 0, false},
		{"later-budget-expiry", []Observation{done, cancelled}, InconclusiveTimeout, StatusFail, 1, 1, 0, false},
		{"no-launch-budget-expiry", []Observation{cancelled}, InconclusiveTimeout, StatusNotRun, 0, 0, 0, false},
		{"no-launch-start-failure", []Observation{{}}, InconclusiveSetup, StatusNotRun, 0, 0, 0, false},
		{"undrained", []Observation{done, undrained}, InconclusiveInconsistent, StatusFail, 2, 2, 0, false},
		{"unclean", []Observation{done, unclean}, InconclusiveInconsistent, StatusFail, 2, 2, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := Adjudicate(6, RefusalNone, tc.obs)
			if a.Status != tc.status || a.CaseInconclusiveState != tc.state || a.RecordCount != tc.records || a.Launches != tc.launches || a.UnobservedLaunches != tc.unobserved || a.FullyObserved != tc.full {
				t.Fatalf("%+v", a)
			}
			if !tc.full && a.Outcome != OutcomeWithheld {
				t.Fatalf("outcome=%s", a.Outcome)
			}
			if tc.full && a.Outcome != "" {
				t.Fatal(a)
			}
		})
	}
}
