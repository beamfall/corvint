// SPDX-License-Identifier: Apache-2.0
package appflows

import (
	"context"

	"github.com/Beamfall/corvint/internal/extevidence"
)

// E2ERunnerTest is one test from the runner's native list report.
type E2ERunnerTest struct {
	TestKey, Path, Project string
}

// E2ERunnerResult is what the core RSP binding (internal/appflows/e2erunner)
// reports: either Failed with a bounded reason, or the inventoried tests and
// the shared selector's per-runner reasons.
type E2ERunnerResult struct {
	Failed  bool
	Failure string
	Tests   []E2ERunnerTest
	Reasons []string
}

// E2ERunnerInput is an in-process core binding, not a provider/wire extension.
// The core binds Evaluate to admitted T1 records, core qualification and the
// runner's native list report; no old coverage/provider record can substitute
// for a missing RSP component. Keeping the binding behind a callback keeps
// this package off the runner and inventory packages.
type E2ERunnerInput struct {
	Evaluate func(context.Context) E2ERunnerResult
}

func selectE2ERunner(ctx context.Context, in E2EInput) E2ESelection {
	out := E2ESelection{Schema: E2ESafeSchema, Profile: E2ESafeProfile, Base: in.Base, Mandatory: in.Mandatory,
		State: extevidence.SelectionFull, StateReason: "e2e-fallback", Selected: []E2ETest{}, OmittedTests: []E2EOmission{}, Proofs: []E2EProof{}, DisjointFrom: []string{}, OmissionsPerBasis: map[string]int{BasisCoverage: 0, BasisReviewedLinks: 0}, Fallback: []E2EFallback{}, Note: extevidence.SelectionNote + "; " + e2eSafeNote,
		UntrustedTextFields: []string{"test_selection.selected[].test_key", "test_selection.selected[].project", "test_selection.omitted_tests[].test_key", "test_selection.omitted_tests[].project"}}
	fail := func(reason string) E2ESelection {
		out.Discovery = E2EDiscovery{State: "MALFORMED"}
		out.Fallback = append(out.Fallback, E2EFallback{Code: CodeExclusionUnproven, Subject: reason})
		return out
	}
	if ctx.Err() != nil || in.RunnerEvidence.Evaluate == nil {
		return fail("inventory-unavailable")
	}
	res := in.RunnerEvidence.Evaluate(ctx)
	if res.Failed {
		return fail(res.Failure)
	}
	out.Discovery = E2EDiscovery{State: "MATCHED"}
	for _, t := range res.Tests {
		out.Selected = append(out.Selected, E2ETest{TestKey: t.TestKey, Path: t.Path, Project: t.Project, Reasons: []string{CodeExclusionUnproven}})
	}
	// T2 is explicitly shadow-only. Even supplied qualification cannot activate
	// a second enforcing selector here; promotion belongs to the shared rollout.
	for _, reason := range res.Reasons {
		out.Fallback = append(out.Fallback, E2EFallback{Code: CodeExclusionUnproven, Subject: reason})
	}
	if len(out.Fallback) == 0 {
		out.Fallback = append(out.Fallback, E2EFallback{Code: CodeExclusionUnproven, Subject: "runner-selection-shadow"})
	}
	return out
}
