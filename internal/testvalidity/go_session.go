package testvalidity

// ProjectGoSession recomputes the shared projection for one Go live-session
// state. Identity is the observation's current-input binding; without it the
// freshness axis stays UNKNOWN and no anchor is invented.
func ProjectGoSession(state, identity string) Projection {
	execution := ExecutionFacts{}
	switch state {
	case "passed":
		execution.Outcome = "PASSED"
	case "failed":
		execution.Outcome, execution.Cause = "FAILED", "ASSERTION_OR_TEST"
	case "stale":
		execution.Outcome, execution.Cause = "INCOMPLETE", "STALE"
	case "infrastructure":
		execution.Outcome, execution.Cause = "INCOMPLETE", "INFRASTRUCTURE"
	case "cancelled":
		execution.Outcome, execution.Cause = "INCOMPLETE", "CANCELLATION"
	}
	if identity != "" {
		execution.Anchors = []string{"input-identity:" + identity}
		switch state {
		case "passed", "failed":
			execution.Currency = FreshnessCurrent
		case "stale":
			execution.Currency = FreshnessStale
		}
	}
	return Project(Input{Execution: &execution})
}

// ProjectGoTest recomputes one Go test observation. Action is the retained
// last terminal go test -json action (pass, fail, skip), or another/empty
// value when no terminal action was observed. An absent action therefore
// abstains on execution rather than trusting a carried projection.
func ProjectGoTest(action, identity string, stale bool, anchor string) Projection {
	execution := ExecutionFacts{}
	switch action {
	case "pass":
		execution.Outcome = "PASSED"
	case "fail":
		execution.Outcome, execution.Cause = "FAILED", "ASSERTION_OR_TEST"
	case "skip":
		execution.Outcome = "SKIPPED"
	}
	if identity != "" {
		execution.Currency = FreshnessCurrent
		if stale {
			execution.Currency = FreshnessStale
		}
		execution.Anchors = []string{"input-identity:" + identity}
		if anchor != "" {
			execution.Anchors = []string{anchor, "input-identity:" + identity}
		}
	}
	return Project(Input{Execution: &execution})
}
