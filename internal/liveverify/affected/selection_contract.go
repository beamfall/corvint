// SPDX-License-Identifier: Apache-2.0
package affected

// UnobservedRuntimeReaders names test holders whose declared runtime target
// this graph did not observe. A partial graph cannot certify their exclusion
// (AFP-V0-9500): qualification must retain them for any dirty path, even if a
// supplied receipt omitted them. The declaration itself supplies the reason.
func (graph *Graph) UnobservedRuntimeReaders() map[string]string {
	readers := map[string]string{}
	for _, edge := range graph.runtimeEdges {
		if _, observed := graph.units[edge.Runs]; observed {
			continue
		}
		for _, id := range graph.order {
			for _, test := range graph.units[id].Tests {
				if runtimeEdgeHolds(edge.Tests, test) {
					if _, exists := readers[id]; !exists {
						readers[id] = RuntimeEdgesPath + " names an unobserved runtime target " + edge.Runs
					}
				}
			}
		}
	}
	return readers
}
