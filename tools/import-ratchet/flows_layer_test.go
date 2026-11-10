//corvint:read-kinds .go go.mod

package importratchet

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The packages decision 0595 names. Each literal is compared against the manifest, the baseline
// and the scanned graph and names no read of its own, so it carries the fixture marker; the one
// read is scanModule's, bounded by the read-kinds directive above.
const (
	flowsAppflows     = "internal/appflows"      //corvint:fixture-path
	flowsAppmap       = "internal/appmap"        //corvint:fixture-path
	flowsCoverage     = "internal/flowcoverage"  //corvint:fixture-path
	flowsCorvint      = "cmd/corvint"            //corvint:fixture-path
	flowsTestplan     = "internal/testplan"      //corvint:fixture-path
	flowsTestsource   = "internal/testsource"    //corvint:fixture-path
	flowsCorvintDocs  = "cmd/corvint-docs"       //corvint:fixture-path
	flowsCommandGroup = "internal/command/flows" //corvint:fixture-path
)

// TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs pins the layering decision 0595 rests on.
// The flow data model and its read-only readers are keyed `shared`, so the tests layer reads flows
// in process (code -> flow -> E2E test selection) and never through the docs product; the `flows`
// and `how` commands are keyed `docs`; and no baselined edge lets corvint, or any tests-layer
// package, import the flows command package again (CMS-V0-916). TestImportRatchet enforces the
// same rule on every edge; this test names the flows ones so a re-key or a re-baselined edge
// fails with the decision it breaks.
func TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs(t *testing.T) {
	m, b := loadEmbedded(t)
	keyed := map[string]string{
		flowsAppflows:     "shared",
		flowsAppmap:       "shared",
		flowsCoverage:     "shared",
		flowsCorvint:      "tests",
		flowsTestplan:     "tests",
		flowsTestsource:   "tests",
		flowsCorvintDocs:  "docs",
		flowsCommandGroup: "docs",
	}
	for pkg, want := range keyed {
		if a, _, ok := m.areaOf(pkg); !ok || a.Name != want {
			t.Errorf("FLOW_LAYER_REKEYED %s: area %v, want %s (decision 0595)", pkg, a, want)
		}
	}
	for _, edge := range b.Edges {
		from, to, _ := strings.Cut(edge, " -> ")
		if from == flowsCorvint || to == flowsCommandGroup || to == flowsCorvintDocs {
			t.Errorf("FLOW_COMMAND_EDGE_BASELINED %s: corvint forwards flows and how to corvint-docs and holds no flow command code (decision 0595); delete the import, not the rule", edge)
		}
	}
	graph, err := scanModule(filepath.Join("..", ".."), m.Module)
	if err != nil {
		t.Fatal(err)
	}
	for from, imports := range graph {
		if a, _, ok := m.areaOf(from); ok && a.Name == "tests" && slices.Contains(imports, flowsCommandGroup) {
			t.Errorf("FLOW_COMMAND_IMPORTED %s imports %s in a production file", from, flowsCommandGroup)
		}
	}
	for _, reader := range []string{flowsCorvint, flowsTestplan, flowsTestsource} {
		if !slices.Contains(graph[reader], flowsAppflows) && !slices.Contains(graph[reader], flowsAppmap) {
			t.Errorf("FLOW_READER_UNREACHED %s imports neither %s nor %s: the tests layer no longer reads flows in process", reader, flowsAppflows, flowsAppmap)
		}
	}
	regained := copyGraph(graph)
	regained[flowsCorvint] = append(regained[flowsCorvint], flowsCommandGroup)
	want := "NEW_WRONG_WAY_EDGE " + edgeKey(flowsCorvint, flowsCommandGroup) + ":"
	if got := check(m, regained, b); len(got) != 1 || !strings.HasPrefix(got[0], want) {
		t.Errorf("SPY_INEFFECTIVE corvint importing the flows commands: got %q, want one problem starting %q", got, want)
	}
}
