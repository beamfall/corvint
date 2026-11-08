// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedReadScopeChanges binds AFP-V0-3300/3301 selection inputs to the
// immutable measured commit and the planned tree. Git admission stays at the
// CLI boundary, so merely using the affected graph needs no Git object reader.
func affectedReadScopeChanges(ctx context.Context, in affectedPlanInputs, graph *affected.Graph, dirty []string) *affected.Graph {
	needed := false
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if len(unit.ReadScopeTrees) != 0 {
			needed = true
			break
		}
	}
	if !needed {
		return graph
	}
	unknown := func() *affected.Graph { return graph.WithReadScopeSources(nil, nil, dirty) }
	base := affectedMeasuredCommit(ctx, in.gitExecutable, in.repository, in.base, in.revision)
	if base == "" {
		return unknown()
	}
	repository, err := gitauth.Open(in.repository, gitrun.NewBudget(gitrun.DefaultOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		return unknown()
	}
	defer repository.BeginObjectSession()()
	source, err := repository.RevisionFS(ctx, base, affected.MaxSourceBytes)
	if err != nil {
		return unknown()
	}
	return graph.WithReadScopeSources(affected.FSSource(source), affected.DiskSourceContext(ctx, in.tree), dirty)
}
