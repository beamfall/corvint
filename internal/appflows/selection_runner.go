// SPDX-License-Identifier: Apache-2.0
package appflows

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/webdialect"
)

// E2ERunnerInput is an in-process core binding, not a provider/wire extension.
// Selector carries admitted T1 records and core qualification; discovery is
// exclusively the runner's native list report. No old coverage/provider record
// can substitute for a missing RSP component.
type E2ERunnerInput struct {
	Discovery jslinecov.Discovery
	Report    string
	Selector  runnerselection.Input
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
	if ctx.Err() != nil {
		return fail("inventory-unavailable")
	}
	rsp := in.RunnerEvidence
	inv, err := webdialect.E2EInventory(rsp.Discovery, rsp.Report)
	if err != nil {
		return fail(inv.Reason)
	}
	out.Discovery = E2EDiscovery{State: "MATCHED"}
	for _, t := range inv.Tests {
		var tuple []string
		if json.Unmarshal([]byte(strings.TrimPrefix(t.ID, "playwright:")), &tuple) != nil || len(tuple) != 3 {
			return fail("inventory-unavailable")
		}
		out.Selected = append(out.Selected, E2ETest{TestKey: t.ID, Path: t.Path, Project: tuple[1], Reasons: []string{CodeExclusionUnproven}})
	}
	input := rsp.Selector
	input.Runners = append([]runnerselection.Runner{}, input.Runners...)
	if len(input.Runners) != 1 || input.Runners[0].Name != "playwright" || input.Head != inv.Key.Checkout || input.Runners[0].Identity.ConfigurationDigest != inv.Key.ConfigDigest || input.Runners[0].Identity.RunnerVersion != rsp.Discovery.Version || input.Runners[0].Execution.OS != inv.Key.OS || input.Runners[0].Execution.Architecture != inv.Key.Arch {
		return fail("inventory-key-mismatch")
	}
	input.Runners[0].Inventory = inv
	plan := runnerselection.Select(input)
	// T2 is explicitly shadow-only. Even supplied qualification cannot activate
	// a second enforcing selector here; promotion belongs to the shared rollout.
	for _, p := range plan.Runners {
		for _, reason := range p.Reasons {
			out.Fallback = append(out.Fallback, E2EFallback{Code: CodeExclusionUnproven, Subject: reason})
		}
	}
	if len(out.Fallback) == 0 {
		out.Fallback = append(out.Fallback, E2EFallback{Code: CodeExclusionUnproven, Subject: "runner-selection-shadow"})
	}
	return out
}
