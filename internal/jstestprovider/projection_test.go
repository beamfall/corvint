package jstestprovider

import "testing"

func TestToTestProjection_NeverCollapsesToBoolean(t *testing.T) {
	cases := []struct {
		state         ExecutionState
		wantExecution string
		wantAssociate string
	}{
		{StatePassed, "PASSED", "ASSOCIATED"},
		{StateFailed, "FAILED", "ASSOCIATED"},
		{StateSkipped, "SKIPPED", "ASSOCIATED"},
		{StateFlaky, "PASSED", "ASSOCIATED"},
	}
	for _, c := range cases {
		outcome := TestOutcome{Name: "t", State: c.state, Anchor: &Anchor{File: "x.test.js", Line: 3}}
		p := ToTestProjection(outcome)
		if p.Execution.State != c.wantExecution {
			t.Fatalf("%s: want execution %s, got %+v", c.state, c.wantExecution, p.Execution)
		}
		if p.Association.State != c.wantAssociate {
			t.Fatalf("%s: want association %s, got %+v", c.state, c.wantAssociate, p.Association)
		}
		if len(p.Association.Anchors) == 0 {
			t.Fatalf("%s: want a source anchor carried through, got none", c.state)
		}
	}
}

func TestToTestProjection_HarnessIncompleteStates(t *testing.T) {
	cases := []struct {
		state     ExecutionState
		wantState string
		wantCause string
	}{
		{StateTimedOut, "INFRASTRUCTURE", "TIMEOUT"},
		{StateInterrupted, "CANCELLED", "CANCELLATION"},
		{StateInfrastructure, "INFRASTRUCTURE", "INFRASTRUCTURE"},
	}
	for _, c := range cases {
		p := ToTestProjection(TestOutcome{Name: "t", State: c.state})
		if p.Execution.State != c.wantState {
			t.Fatalf("%s: want execution state %s, got %+v", c.state, c.wantState, p.Execution)
		}
		if p.Execution.Reason != c.wantCause {
			t.Fatalf("%s: want cause %s, got %+v", c.state, c.wantCause, p.Execution)
		}
	}
}

// TestReceiptRunProjection_StaleAppBuild confirms a stale app build surfaces
// on the freshness axis independent of whether any test failed - the axis
// never collapses into an aggregate pass/fail.
func TestReceiptRunProjection_StaleAppBuild(t *testing.T) {
	p := ReceiptRunProjection(Receipt{StaleAppBuild: true})
	if p.Freshness.State != "STALE" {
		t.Fatalf("want STALE freshness, got %+v", p.Freshness)
	}
}

func TestReceiptRunProjection_Cancelled(t *testing.T) {
	p := ReceiptRunProjection(Receipt{Cancelled: true})
	if p.Execution.State != "CANCELLED" {
		t.Fatalf("want CANCELLED execution, got %+v", p.Execution)
	}
}

func TestReceiptRunProjection_Infrastructure(t *testing.T) {
	p := ReceiptRunProjection(Receipt{Infrastructure: &InfrastructureFailure{Reason: "timeout"}})
	if p.Execution.State != "INFRASTRUCTURE" {
		t.Fatalf("want INFRASTRUCTURE execution, got %+v", p.Execution)
	}
}
