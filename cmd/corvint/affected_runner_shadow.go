// SPDX-License-Identifier: Apache-2.0
package main

import (
	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/runnerpolicy"
	"github.com/Beamfall/corvint/internal/runnerselection"
)

// affectedRunnerShadow reports declarations without discovery or execution.
// All installed pieces are currently absent. Their inventories, qualifications
// and indexes remain unknown rather than treating static test-file counts as
// per-test inventories. Only advice receives this plan; execution keeps using
// the existing affected plan and providers (RSP-V0-040 step 1).
func affectedRunnerShadow(base, head string, changed []string) runnerselection.Plan {
	in := runnerselection.Input{Base: base, Head: head, Repository: "root", Repositories: map[string]bool{"root": true}, DiffReason: "static-path-list-only"}
	for _, p := range changed {
		in.Changes = append(in.Changes, runnerselection.Change{Repository: "root", Path: p})
	}
	seen := map[string]bool{}
	for _, manifest := range langplugin.All() {
		for name, pieces := range manifest.RunnerPieces {
			if seen[name] {
				continue
			}
			seen[name] = true
			in.Runners = append(in.Runners, runnerselection.Runner{Name: name, Pieces: pieces})
		}
	}
	// Both CI and local affected consumers use the same policy. No rollout or
	// protected sampling configuration is admitted by this declaration-only path.
	request := runnerpolicy.Request{Input: in}
	plan := runnerpolicy.ResolveLocal(request, request).Shadow
	// Declaration-only reporting has no retained inventory/loading/selection
	// measurement. Null preserves that uncertainty and keeps replay byte-stable.
	plan.SelectionNS = nil
	return plan
}
