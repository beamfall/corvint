package affected

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exclusionsGraph builds chain() after edit, beside the y plugin, over a root holding the given
// runtime-readers and unbounded-reader-exclusions declarations (none when empty).
func exclusionsGraph(t *testing.T, readers, exclusions string, edit func(*fake)) (*Graph, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{RuntimeReadersPath: readers, UnboundedReaderExclusionsPath: exclusions} {
		if body != "" {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	language := chain()
	edit(&language)
	other := fake{name: "y", units: []Unit{{ID: "y:app", Sources: []string{"app.y"}, Tests: []string{"app_test.y"}}}, frontier: []string{"y:dynamic-import"}}
	return Build(context.Background(), root, language, other)
}

// AFP-V0-772 (V1-3571): a runtime-readers entry the exclusions declaration names selects its
// literal and read-scope readers but no unbounded reader, and follows no root locator, on a
// change of only its paths, the shape of a CEM seal commit; the same unbounded reader is still
// selected for an entry it does not name, and for any other dirty path. The runtime-readers file
// keeps its closed grammar, so a planner that predates the exclusions reads it unchanged, and an
// exclusion naming no entry refuses the graph.
func TestUnboundedReaderExclusionsDropOnlyUnboundedReaders_AFPV0772(t *testing.T) {
	readers := `{"profile":"corvint-runtime-readers/0","entries":[` +
		`{"path":".corvint/change.cem.json","reason":"shared CEM bind path"},` +
		`{"path":".corvint/changes/","reason":"sealed CEMs"},` +
		`{"path":"data/catalog.json","reason":"embedded catalog"}]}`
	exclusions := `{"profile":"corvint-unbounded-reader-exclusions/0","paths":[".corvint/change.cem.json",".corvint/changes/"]}`
	seal := []string{".corvint/change.cem.json", ".corvint/changes/0123.cem.json"}
	edit := func(language *fake) {
		language.units[3].UnboundedReads = "root-locator"
		language.units[0].PathTokens = []string{".corvint/changes/"}
	}
	graph, err := exclusionsGraph(t, readers, exclusions, edit)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, seal)
	kinds := witnesses(plan)
	if _, selected := kinds["x:solo"]; selected {
		t.Fatalf("an unbounded reader is selected for an excluded entry: %+v", plan.Selected)
	}
	if kinds["x:core"].Kind != WitnessPathLiteralReader {
		t.Fatalf("the literal reader is not selected: %+v", plan.Selected)
	}
	if len(groupPaths(plan, ClassDeclaredRuntimeReaders)) != 2 || len(unknownDetails(plan)) != 0 {
		t.Fatalf("seal paths: unknown=%v nonSource=%v", unknownDetails(plan), plan.NonSource)
	}
	if got := witnesses(Select(graph, []string{"data/catalog.json"}))["x:solo"]; got.Kind != WitnessUnboundedReader {
		t.Fatalf("an entry the exclusions do not name dropped the unbounded reader: %+v", got)
	}
	if got := witnesses(Select(graph, append([]string{"core.x"}, seal...)))["x:solo"]; got.Kind != WitnessUnboundedReader {
		t.Fatalf("another dirty path dropped the unbounded reader: %+v", got)
	}
	plain, err := exclusionsGraph(t, readers, "", edit)
	if err != nil {
		t.Fatal(err)
	}
	if got := witnesses(Select(plain, seal))["x:solo"]; got.Kind != WitnessUnboundedReader {
		t.Fatalf("without the exclusions the unbounded reader is not selected: %+v", got)
	}
	for name, bad := range map[string]string{
		"unknown path":      `{"profile":"corvint-unbounded-reader-exclusions/0","paths":["other/"]}`,
		"member in readers": "",
	} {
		body := readers
		if name == "member in readers" {
			body = strings.Replace(readers, `"reason":"sealed CEMs"`, `"reason":"sealed CEMs","noUnboundedReaders":true`, 1)
		}
		if _, err := exclusionsGraph(t, body, bad, edit); err == nil {
			t.Errorf("%s: the graph build accepted it", name)
		}
	}
}
