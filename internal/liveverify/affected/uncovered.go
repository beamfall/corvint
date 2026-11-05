// SPDX-License-Identifier: Apache-2.0
package affected

import "strings"

// coverageGap inspects modelled and declared callers before claiming that no
// selectable test can observe a changed unit (AZU-V0-006). It does not narrow
// the ordinary reach: callers and path readers have already been selected.
func (graph *Graph) coverageGap(id string, seed Witness, reached map[string]Witness) (gap, uncertain bool) {
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
		for _, reasons := range [][]string{unit.Frontier, graph.frontierBy[language]} {
			for _, reason := range reasons {
				if !declared || direction.FrontierDirection(reason) != "OUTBOUND" {
					uncertain = true
				}
			}
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

	// Literal, scoped, glob and unbounded readers can observe the path without
	// an import edge. Retain their selection and do not call that path uncovered.
	for unit, witness := range reached {
		if witness.DirtyPath == seed.DirtyPath && len(graph.units[unit].Tests) != 0 {
			observer = true
		}
	}
	return !observer, uncertain
}
