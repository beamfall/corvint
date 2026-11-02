package affected

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// AFP-V0-987 (V1-3898): a dirty set of declared runtime-readers data alone
// changes no plugin's code, so the terminal reader it selects leaves its
// cross-language runtime entry out (AFP-V0-560) instead of naming it; the
// same data changed beside a source path of another plugin still names it
// (AFP-V0-818).
func TestDeclaredDataAloneCarriesNoRuntimeEdge_AFP_V0_987(t *testing.T) {
	g := fake{name: "g", units: []Unit{
		{ID: "g:server", Sources: []string{"server.g"}, Tests: []string{"server_test.g"}},
	}}
	s := fake{name: "s", frontier: []string{runtimeEdgeSite}, crossLanguage: []string{"s:e2e-runtime"}, units: []Unit{
		{ID: "s:reader", Sources: []string{"reader.s"}, Tests: []string{"reader_test.s"}, PathTokens: []string{"data.json"}},
		{ID: "s:spec", Tests: []string{"spec.s"}},
	}}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	readers := `{"profile":"corvint-runtime-readers/0","entries":[{"path":"data.json","reason":"read only by s:reader"}]}`
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(RuntimeReadersPath)), []byte(readers), 0o644); err != nil {
		t.Fatal(err)
	}
	graph, err := Build(context.Background(), root, g, s)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"data.json"})
	want := []FrontierExclusion{{Language: "s", Reason: FrontierExclusionReason, Frontier: []string{runtimeEdgeSite}, Units: []string{"s:reader"}}}
	if got := frontierOf(plan); got != "BOUNDED []" || !reflect.DeepEqual(plan.FrontierExcluded, want) {
		t.Fatalf("declared data alone: plan %s frontierExcluded=%+v", got, plan.FrontierExcluded)
	}
	if got := frontierOf(Select(graph, []string{"data.json", "server.g"})); got != "UNKNOWN ["+runtimeEdgeSite+"]" {
		t.Fatalf("declared data beside a server change: plan %s, want the cross-language entry named", got)
	}
}
