// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-1061: new dated records retain declared readers and the root-locator
// floor, including contextindex, whose self-corpus scanner reads every tracked file.
func TestNewBuildLogKeepsDeclaredReaders_TSPV01061(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	dirty := "docs/build-log/2099-01-01-new-record.md"
	plan := affected.Select(graph, []string{dirty})
	for _, s := range plan.Selected {
		if s.UnitID == "go:"+module+"/internal/contextindex" && s.Witness.Kind != affected.WitnessUnboundedReader {
			t.Errorf("self-corpus reader lost its root-locator witness: %+v", s.Witness)
		}
	}
	units := selectedUnits(graph, module, dirty)
	for _, reader := range []string{"cmd/corvint", "internal/contextindex"} {
		if _, ok := units[reader]; !ok {
			t.Errorf("reader %s omitted", reader)
		}
	}
	// Unrelated data retains the same conservative floor.
	wider := affected.Select(graph, []string{"undeclared/data.json"})
	if !slices.ContainsFunc(wider.Selected, func(s affected.Selection) bool { return s.Witness.Kind == affected.WitnessUnboundedReader }) {
		t.Fatal("undeclared input lost its conservative fallback")
	}
}
