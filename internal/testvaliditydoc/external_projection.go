package testvaliditydoc

import (
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// ReasonExternalCurrencyUnverified is the freshness reason of an external
// Playwright receipt: the application is owned and built outside the
// provider, so no execution identity binds the tested build to the current
// source (PWP-V0-154).
const ReasonExternalCurrencyUnverified = "external-application-currency-unverified"

func externalProfile(profile string) bool {
	return profile == jstestprovider.ExternalProfile || profile == jstestprovider.AttestedExternalProfile || profile == jstestprovider.SensitiveExternalProfile || profile == jstestprovider.AttemptExternalProfile
}

// externalTestProjection names why an external test's freshness is unknown.
// The qualified envelope's own projections are frozen wire and unchanged.
func externalTestProjection(p testvalidity.Projection) testvalidity.Projection {
	p.Freshness = externalFreshness(p.Freshness)
	return p
}

// ReasonExternalRunPerTest is the run execution reason of an external
// receipt with tests and no run-level infrastructure or cancellation: the
// run stays UNSUPPORTED, because DCP-V1-007 forbids promoting a retained
// external witness's run to PASSED, and each test's own execution axis is
// the verdict (PWP-V0-153).
const ReasonExternalRunPerTest = "external-run-execution-per-test"

// externalRunProjection names why a complete external run has no run-level
// execution state and why its freshness is unknown (PWP-V0-153, PWP-V0-154).
func externalRunProjection(run testvalidity.Projection, tests []Test) testvalidity.Projection {
	if run.Execution.State == testvalidity.ExecutionUnsupported && len(tests) != 0 {
		run.Execution = testvalidity.Axis{State: testvalidity.ExecutionUnsupported, Reason: ReasonExternalRunPerTest, Anchors: run.Execution.Anchors}
	}
	run.Freshness = externalFreshness(run.Freshness)
	return run
}

func externalFreshness(axis testvalidity.Axis) testvalidity.Axis {
	if axis.State == testvalidity.FreshnessStale || axis.State == testvalidity.FreshnessCurrent {
		return axis
	}
	return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: ReasonExternalCurrencyUnverified, Anchors: axis.Anchors}
}
