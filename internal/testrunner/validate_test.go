package testrunner

import "testing"

func TestNormalizeBoundaryAndExitProfiles(t *testing.T) {
	observed := func() Observation {
		return Observation{Complete: true, RetryInformation: NotApplicable, Tests: []Test{{ID: "test", State: Passed, Attempts: []Attempt{{State: Passed}}}}}
	}
	for _, in := range []Input{
		{ExitCode: 0, Expected: []string{"absent"}},
		{ExitCode: 0, SuccessExitCodes: []int{0}, FailureExitCodes: []int{0}},
		{ExitCode: 0, OutcomeNeutralExitCodes: []int{0, 0}},
		{ExitCode: 0, SuccessExitCodes: []int{256}},
		{ExitCode: 0, ExecutionProblems: []Problem{{"binding", "changed"}}},
		{ExitCode: 0, TimedOut: true},
	} {
		o := Normalize(in, observed())
		if o.Complete || o.Tests[0].State != Unknown || o.Tests[0].Attempts[0].State != Unknown {
			t.Fatalf("unsafe observation: %+v", o)
		}
	}
	for _, in := range []Input{{ExitCode: 1}, {ExitCode: 0, OutcomeNeutralExitCodes: []int{0}}} {
		o := observed()
		o.Tests[0].State = Failed
		o.Tests[0].Attempts[0].State = Failed
		got := Normalize(in, o)
		if !got.Complete || got.Tests[0].State != Failed {
			t.Fatalf("ordinary failure invalidated: %+v", got)
		}
	}
}

func TestNormalizeFullNativeExitRange(t *testing.T) {
	failure := make([]int, 255)
	for i := range failure {
		failure[i] = i + 1
	}
	in := Input{ExitCode: 255, SuccessExitCodes: []int{0}, FailureExitCodes: failure}
	o := Normalize(in, Observation{Complete: true, RetryInformation: NotApplicable, Tests: []Test{{ID: "case", State: Failed}}})
	if !o.Complete || o.Tests[0].State != Failed {
		t.Fatalf("native bounded range refused: %+v", o)
	}
}

func TestAggregateInventoryDoesNotInventExecutedCases(t *testing.T) {
	zero, one := 0, 1
	for _, c := range []struct {
		name, state       string
		executed, skipped *int
		complete          bool
	}{{"skip-only", Skipped, &zero, &one, true}, {"empty", Passed, &zero, &zero, false}, {"skip-as-pass", Passed, &zero, &one, false}, {"execution-as-skip", Skipped, &one, &zero, false}, {"missing-count", Passed, nil, &zero, false}} {
		t.Run(c.name, func(t *testing.T) {
			o := Normalize(Input{Runner: "suite", ExitCode: 0}, Observation{Complete: true, RetryInformation: NotReported, Tests: []Test{{ID: "suite:a.slt", State: c.state, Granularity: "SUITE_ONLY", ExecutedCount: c.executed, SkippedCount: c.skipped}}})
			if o.Complete != c.complete {
				t.Fatalf("unexpected aggregate classification: %+v", o)
			}
		})
	}
}
