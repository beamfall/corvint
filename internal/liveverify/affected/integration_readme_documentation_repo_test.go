package affected_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestIntegrationReadmesAreDocumentation_AFP_V0_830: a host README edit is
// declared documentation, so it raises no UNOWNED_DIRTY_PATH entry and no
// frontier of a plugin that neither owns, reads nor reaches it, while the
// units that read it by literal stay selected (V1-3509).
func TestIntegrationReadmesAreDocumentation_AFP_V0_830(t *testing.T) {
	graph, module := repositoryNarrowingGraph(t)
	for _, test := range []struct {
		dirty   string
		readers []string
	}{
		{"integrations/opencode/README.md", []string{"internal/opencodequalification"}},
		{"integrations/claude-code/README.md", nil},
		{"integrations/README.md", nil},
	} {
		plan := affected.Select(graph, []string{test.dirty})
		declared := false
		for _, group := range plan.NonSource {
			if group.Class == "DECLARED_DOCUMENTATION" && len(group.Paths) == 1 && group.Paths[0] == test.dirty {
				declared = true
			}
		}
		if !declared {
			t.Errorf("%s: not classified DECLARED_DOCUMENTATION: %+v", test.dirty, plan.NonSource)
		}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == "UNOWNED_DIRTY_PATH" || strings.HasPrefix(unknown.Detail, "cfamily:") || strings.HasPrefix(unknown.Detail, "dotnet:") {
				t.Errorf("%s: unknown %s %s; a README reaches no such plugin", test.dirty, unknown.Reason, unknown.Detail)
			}
		}
		units := selectedUnits(graph, module, test.dirty)
		for _, reader := range test.readers {
			if _, selected := units[reader]; !selected {
				t.Errorf("%s: reader %s not selected", test.dirty, reader)
			}
		}
	}
	// A source file in the same host directory stays source: the declaration
	// names only the README.
	plan := affected.Select(graph, []string{"integrations/opencode/src/index.js"})
	for _, group := range plan.NonSource {
		if group.Class == "DECLARED_DOCUMENTATION" {
			t.Errorf("integrations/opencode/src/index.js classified as documentation")
		}
	}
	if len(plan.Selected) == 0 {
		t.Errorf("integrations/opencode/src/index.js selects nothing")
	}
}
