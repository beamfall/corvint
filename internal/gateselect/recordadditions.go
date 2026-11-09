// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// recordAdditionAllows re-observes additions and source-bound scopes; a receipt
// cannot supply its own exclusion. Both gate-affected-select and the planner
// use this same admission rule (AFP-V0-3423). Unknown input only widens.
func recordAdditionAllows(plan Receipt, root, module string) func(string, string) bool {
	allow := func(string, string) bool { return true }
	if plan.Snapshot != nil {
		return allow
	}
	base := plan.Range.Base
	if base == "" {
		base = plan.Revision
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if len(affected.AddedRecordEntries(ctx, "git", root, base, plan.Plan.Dirty)) == 0 {
		return allow
	}
	graph, err := affected.BuildPartial(ctx, root, golang.New())
	if err != nil {
		return allow
	}
	graph = graph.WithRecordEntryBase(ctx, "git", root, base, plan.Plan.Dirty)
	return func(directory, dirty string) bool {
		return graph.RecordEntryAllows("go:"+importPath(module, directory), dirty)
	}
}
