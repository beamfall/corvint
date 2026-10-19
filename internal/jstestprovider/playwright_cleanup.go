package jstestprovider

import (
	"github.com/Beamfall/corvint/internal/runhygiene"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// ReasonPassedCleanupFailed is the infrastructure reason of a Playwright run whose tests all passed
// (or were skipped) while the report recorded a run-level error, such as a worker fixture teardown
// or cleanup failure (TSA-V0-168). The run is not clean evidence, so the receipt fails closed like
// any other infrastructure failure instead of reading as passed.
const ReasonPassedCleanupFailed = "passed-cleanup-failed"

// playwrightCleanupFailure returns the passed-cleanup-failed failure when the report has a run-level
// error and no outcome failed; a run with a failing outcome already fails and is left unchanged.
func playwrightCleanupFailure(errors []playwrightError, outcomes []TestOutcome) *InfrastructureFailure {
	if len(errors) == 0 {
		return nil
	}
	for _, o := range outcomes {
		if o.State != StatePassed && o.State != StateFlaky && o.State != StateSkipped {
			return nil
		}
	}
	detail, _ := secretscreen.Screen(runhygiene.ScrubFailure(errors[0].Message))
	return &InfrastructureFailure{Reason: ReasonPassedCleanupFailed, Detail: detail}
}
