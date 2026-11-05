// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"strings"
)

// inboundRuntimeSite accepts only a located site whose declared runtime
// caller resolves to a test in this graph. Unlocated and truncated entries
// remain unknown. AZU-V0-004, AZU-V0-8902.
func (graph *Graph) inboundRuntimeSite(entry string) string {
	const prefix = "typescript:e2e-runtime-dependency:"
	if !strings.HasPrefix(entry, prefix) {
		return ""
	}
	file, _, _ := strings.Cut(strings.TrimPrefix(entry, prefix), ":")
	owner := graph.owner[file]
	if owner == "" || !slices.Contains(graph.units[owner].Tests, file) {
		return ""
	}
	for _, edge := range graph.runtimeEdges {
		if runtimeEdgeHolds(edge.Tests, file) {
			if _, exists := graph.units[edge.Runs]; exists {
				return owner
			}
		}
	}
	return ""
}

// Select the declared caller conservatively on every nonempty change. A
// runtime dependency can start outside the statically visible import graph.
// Existing reader selections are kept, and the caller's dependents are walked.
func (graph *Graph) inboundRuntimeCallers(reached map[string]Witness, followed map[string]bool, dirty []string) {
	if len(dirty) == 0 || !graph.inboundChangesCode(dirty) {
		return
	}
	seeds := map[string]Witness{}
	for _, entry := range graph.frontierBy["typescript"] {
		if id := graph.inboundRuntimeSite(entry); id != "" {
			seeds[id] = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{id}}
		}
	}
	for id, unit := range graph.units {
		for _, entry := range unit.Frontier {
			if graph.inboundRuntimeSite(entry) == id {
				seeds[id] = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{id}}
			}
		}
	}
	closure := graph.traverse(seeds, nil)
	graph.testUsersOf(closure, nil)
	for id, witness := range closure {
		if _, exists := reached[id]; !exists {
			reached[id] = witness
		}
		followed[id] = true
	}
}

// Reuse the selector's data classification before adding runtime callers.
// A declared-data-only change cannot start a hidden runtime code edge.
func (graph *Graph) inboundChangesCode(dirty []string) bool {
	_, unknown := graph.seed(dirty)
	task, unknown := graph.taskState(unknown)
	docs, unknown := graph.documentationPaths(unknown)
	readers, _ := graph.runtimeReaderPaths(unknown)
	return changesCode(dirty, append(append(task, docs...), readers...))
}
