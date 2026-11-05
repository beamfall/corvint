// SPDX-License-Identifier: Apache-2.0
package affected

import "strings"

// coverageGap inspects modelled and declared callers before claiming that no
// selectable test can observe a changed unit (AZU-V0-006). It does not narrow
// the ordinary reach: callers and path readers have already been selected.
func (graph *Graph) coverageGap(id string, seed Witness, reached map[string]Witness, dirty []string, readersByPath map[string]map[string]Witness) (gap, uncertain bool) {
	// A seed is only the smallest witness for a unit, not its complete dirty
	// input. Reader membership must be recomputed for every path it owns.
	paths := []string{seed.DirtyPath}
	for _, file := range dirty {
		if graph.owner[file] == id && file != seed.DirtyPath {
			paths = append(paths, file)
		}
	}
	observer := graph.testReach[id]
	callers := map[string]bool{id: true}
	pending := []string{id}
	// AFP-V0-811: embedding source bytes does not model execution of that
	// language. Keep its coverage uncertainty while retaining all Go readers.
	if !strings.HasPrefix(id, "go:") && len(graph.embedded[seed.DirtyPath]) != 0 {
		uncertain = true
	}
	// RUBY-AFFECTED-V0-761: a template without a route-dispatched spec has
	// unresolved runtime dispatch, not proof that no test can observe it.
	if strings.HasPrefix(id, "ruby:view:") && !graph.testReach[id] {
		uncertain = true
	}
	inspect := func(caller string) {
		unit := graph.units[caller]
		observer = observer || len(unit.Tests) != 0 || len(unit.ExecConsumerRuns) != 0
		for _, file := range unit.Callers {
			if graph.owner[file] == "" {
				uncertain = true
			}
		}
		language := strings.SplitN(caller, ":", 2)[0]
		// A missing direction declaration is INBOUND. Plugins that have verified
		// caller declarations remove their closed inbound reasons from the unit.
		direction, declared := graph.claimants[language].(interface{ FrontierDirection(string) string })
		frontier := unit.Frontier
		// TSP-V0-001: a reader's build variants bear only on changes to its
		// own files or build inputs. Reading another package does not qualify.
		if !graph.ownFilesBear(caller, dirty) {
			frontier = nil
		}
		for _, reasons := range [][]string{frontier, graph.frontierBy[language]} {
			for _, reason := range reasons {
				if !declared || direction.FrontierDirection(reason) != "OUTBOUND" {
					uncertain = true
				}
			}
		}
	}
	// Literal, scoped, glob and unbounded readers can observe the path without
	// an import edge. Their caller closures must satisfy the same absence proof
	// as the changed owner's: a declared caller with no owner stays uncertain.
	add := func(unit string) {
		if !callers[unit] {
			callers[unit] = true
			pending = append(pending, unit)
		}
	}
	// With one dirty path every reached witness already names that input.
	// Multiple inputs can hide relevant readers behind another witness, so
	// only that case needs independent membership discovery.
	if len(dirty) > 1 {
		for _, file := range paths {
			readers, cached := readersByPath[file]
			if !cached {
				// Structural changes can seed many owners with the same path.
				// Cache membership within this plan, never across dirty inputs.
				readers = make(map[string]Witness)
				input := []string{file}
				graph.readers(readers, input)
				graph.unboundedReadersOf(readers, graph.unboundedDirty(input))
				graph.scopedReadersOf(readers, input)
				readersByPath[file] = readers
			}
			for unit := range readers {
				add(unit)
			}
		}
	}
	for unit, witness := range reached {
		if witness.DirtyPath == seed.DirtyPath || graph.owner[witness.DirtyPath] == id {
			add(unit)
		}
	}
	// This proof needs membership only, not sorted witness chains. Reuse the
	// graph's reverse edges without building or ranking a second selection.
	for len(pending) != 0 {
		caller := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		inspect(caller)
		for _, edges := range []map[string][]string{graph.dependents, graph.advisory, graph.routed} {
			for _, user := range edges[caller] {
				if !callers[user] {
					callers[user] = true
					pending = append(pending, user)
				}
			}
		}
		for _, edges := range []map[string][]string{graph.testUsers, graph.execUsers} {
			for _, user := range edges[caller] {
				inspect(user)
			}
		}
		if observer && uncertain {
			return false, true
		}
	}

	return !observer, uncertain
}
