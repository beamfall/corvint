// SPDX-License-Identifier: Apache-2.0
package affected

import "testing"

// AFP-V0-9500: a partial observation is not a negative runtime-edge witness;
// observing the target (or a non-holder) does not create an unknown reader.
func TestSelectionContractUnobservedRuntimeReaders_AFPV09500(t *testing.T) {
	graph := &Graph{
		order: []string{"go:m/a", "go:m/b"},
		units: map[string]Unit{
			"go:m/a": {ID: "go:m/a", Tests: []string{"a/a_test.go"}},
			"go:m/b": {ID: "go:m/b", Tests: []string{"b/b_test.go"}},
		},
		runtimeEdges: []RuntimeEdge{{Tests: "a/", Runs: "other:server", Reason: "drives server"}},
	}
	readers := graph.UnobservedRuntimeReaders()
	if len(readers) != 1 || readers["go:m/a"] == "" {
		t.Fatalf("missing unobserved reader: %v", readers)
	}
	graph.units["other:server"] = Unit{ID: "other:server"}
	if readers := graph.UnobservedRuntimeReaders(); len(readers) != 0 {
		t.Fatalf("observed edge is unknown: %v", readers)
	}
}
