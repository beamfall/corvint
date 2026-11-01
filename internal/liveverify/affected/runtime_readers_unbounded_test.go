package affected

import "testing"

// AFP-V0-772 (V1-3571): a runtime-readers entry with noUnboundedReaders selects its literal and
// read-scope readers but no unbounded reader, and follows no root locator, on a change of only
// its paths, the shape of a CEM seal commit; the same unbounded reader is still selected for
// such a path when the entry makes no such statement, and for any other dirty path.
func TestNoUnboundedReadersEntryDropsOnlyUnboundedReaders_AFPV0772(t *testing.T) {
	declaration := `{"profile":"corvint-runtime-readers/0","entries":[` +
		`{"path":".corvint/change.cem.json","reason":"shared CEM bind path","noUnboundedReaders":true},` +
		`{"path":".corvint/changes/","reason":"sealed CEMs","noUnboundedReaders":true},` +
		`{"path":"data/catalog.json","reason":"embedded catalog"}]}`
	seal := []string{".corvint/change.cem.json", ".corvint/changes/0123.cem.json"}
	edit := func(language *fake) {
		language.units[3].UnboundedReads = "root-locator"
		language.units[0].PathTokens = []string{".corvint/changes/"}
	}
	graph, err := runtimeReadersGraph(t, declaration, edit)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, seal)
	kinds := witnesses(plan)
	if _, selected := kinds["x:solo"]; selected {
		t.Fatalf("an unbounded reader is selected for a noUnboundedReaders entry: %+v", plan.Selected)
	}
	if kinds["x:core"].Kind != WitnessPathLiteralReader {
		t.Fatalf("the literal reader is not selected: %+v", plan.Selected)
	}
	if len(groupPaths(plan, ClassDeclaredRuntimeReaders)) != 2 || len(unknownDetails(plan)) != 0 {
		t.Fatalf("seal paths: unknown=%v nonSource=%v", unknownDetails(plan), plan.NonSource)
	}
	if got := witnesses(Select(graph, []string{"data/catalog.json"}))["x:solo"]; got.Kind != WitnessUnboundedReader {
		t.Fatalf("an entry without the statement dropped the unbounded reader: %+v", got)
	}
	if got := witnesses(Select(graph, append([]string{"core.x"}, seal...)))["x:solo"]; got.Kind != WitnessUnboundedReader {
		t.Fatalf("another dirty path dropped the unbounded reader: %+v", got)
	}
}
