package golang_test

import (
	"context"
	json "encoding/json/v2"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const consumerSealed = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"

// consumerHead adds, to the additive fixture, the new Fresh in core, a
// declared documentation file with one resolving reader and one lone
// file-name token, and a sealed CEM with one reader that enumerates its
// directory and one holder of a `.corvint` token beside a bare "/".
var consumerHead = map[string]string{
	"core/core.go":                      additiveCore + "\nfunc Fresh() string { return Old() }\n",
	".corvint/documentation-paths.json": `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}` + "\n",
	"docs/specs/INDEX.json":             "{}\n",
	"reader/reader_test.go":             "package reader\n\nvar index = \"../docs/specs/INDEX.json\"\n",
	"named/named_test.go":               "package named\n\nvar fixture = \"INDEX.json\"\n",
	consumerSealed:                      "{}\n",
	"enum/enum_test.go":                 "package enum\n\nvar sealed = \"../.corvint/changes\"\n",
	"loose/loose.go":                    "package loose\n\nvar sep, base = \"/\", \".corvint\"\n",
	"loose/loose_test.go":               "package loose\n",
	".corvint/changes/forged.cem.json":  "{}\n",
}

// consumerReceipt runs the real planner over the head tree and returns the
// plan as `corvint test` hands it to SelectStructural: the JSON plan with the
// provider's Go packages taken from its selection.
func consumerReceipt(t *testing.T, root string, dirty []string) gateselect.Receipt {
	t.Helper()
	baseFS := fstest.MapFS{}
	for relative, body := range additiveBase {
		baseFS[relative] = &fstest.MapFile{Data: []byte(body)}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	records := golang.AdditiveRecords(baseFS, os.DirFS(root), graph, dirty)
	plan := affected.Select(graph.WithGoAdditive(records), dirty)
	var packages []string
	for _, selection := range plan.Selected {
		if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok {
			packages = append(packages, pkg)
		}
	}
	encoded, err := json.Marshal(map[string]any{
		"plan":     plan,
		"provider": map[string]any{"go": map[string]any{"packages": packages, "state": "RUNNABLE"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var receipt gateselect.Receipt
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func consumerRun(t *testing.T, lines []string) []string {
	t.Helper()
	verdict, packages, _ := strings.Cut(lines[len(lines)-1], " ")
	if verdict != "run" {
		t.Fatalf("structural verdict %q", strings.Join(lines, "\n"))
	}
	return strings.Fields(packages)
}

// TestADNV0010And0011ProducerToConsumer replays ADN-V0-006, ADN-V0-010 and
// ADN-V0-011 from a real planner run to SelectStructural through the JSON plan
// `corvint test` builds: the narrowed record drops its unreferenced importers,
// the documentation and sealed CEM paths take the plan's readers, and a stale
// or forged plan (a dropped reader, a removed declaration, a CEM name that is
// no commit) keeps the consumer's whole rule (c) for that path.
func TestADNV0010And0011ProducerToConsumer(t *testing.T) {
	const module = "example.test/m"
	root := t.TempDir()
	files := maps.Clone(additiveBase)
	maps.Copy(files, consumerHead)
	writeFiles(t, root, files)
	dirty := []string{"core/core.go", "docs/specs/INDEX.json", consumerSealed}
	receipt := consumerReceipt(t, root, dirty)
	has := func(packages []string, name string) bool { return slices.Contains(packages, module+"/"+name) }

	lines := gateselect.SelectStructural(receipt, module, root)
	selected := consumerRun(t, lines)
	for _, want := range []string{"core", "ref", "alias", "reader", "enum"} {
		if !has(selected, want) {
			t.Errorf("%s is not selected: %q", want, strings.Join(lines, "\n"))
		}
	}
	for _, unwanted := range []string{"unref", "leaf", "named"} {
		if has(selected, unwanted) {
			t.Errorf("%s is selected: %q", unwanted, strings.Join(lines, "\n"))
		}
	}
	if !slices.Contains(lines, "documentation docs/specs/INDEX.json") || !slices.Contains(lines, "evidence "+consumerSealed) {
		t.Fatalf("no documentation or evidence line: %q", strings.Join(lines, "\n"))
	}

	// A stale or forged reader selection: the plan no longer selects the
	// documentation reader or the CEM's enumerating reader.
	stale := receipt
	stale.Provider.Go.Packages = slices.DeleteFunc(slices.Clone(receipt.Provider.Go.Packages), func(pkg string) bool {
		return pkg == module+"/reader" || pkg == module+"/enum"
	})
	lines = gateselect.SelectStructural(stale, module, root)
	selected = consumerRun(t, lines)
	if !has(selected, "reader") || !has(selected, "named") || !has(selected, "enum") {
		t.Errorf("a stale reader selection narrowed rule (c): %q", strings.Join(lines, "\n"))
	}
	if slices.ContainsFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, "documentation ") || strings.HasPrefix(line, "evidence ")
	}) {
		t.Errorf("a stale reader selection was taken: %q", strings.Join(lines, "\n"))
	}

	// A forged CEM record: a dirty path under .corvint/changes/ whose name is
	// no commit takes no plan readers.
	forged := receipt
	forged.Plan.Dirty = []string{".corvint/changes/forged.cem.json"}
	forged.Plan.NonSource = nil
	forged.Plan.GoAdditive = nil
	if lines := gateselect.SelectStructural(forged, module, root); slices.ContainsFunc(lines, func(line string) bool { return strings.HasPrefix(line, "evidence ") }) {
		t.Errorf("a forged CEM name took the plan's readers: %q", strings.Join(lines, "\n"))
	}

	// A stale classification: the declaration no longer declares the path.
	if err := os.Remove(filepath.Join(root, ".corvint", "documentation-paths.json")); err != nil {
		t.Fatal(err)
	}
	lines = gateselect.SelectStructural(receipt, module, root)
	selected = consumerRun(t, lines)
	if !has(selected, "named") || slices.Contains(lines, "documentation docs/specs/INDEX.json") {
		t.Errorf("an undeclared documentation path took the plan's readers: %q", strings.Join(lines, "\n"))
	}
}
