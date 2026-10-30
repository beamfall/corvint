package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const runtimeReadersDeclaration = `{"profile":"corvint-runtime-readers/0","entries":[` +
	`{"path":".corvint/changes/","reason":"sealed CEMs; read by the CEM tools' literals and read scopes"},` +
	`{"path":"data/catalog.json","reason":"embedded catalog"}]}`

// runtimeReadersGraph builds chain() after edit, with a path-open frontier on
// the x plugin, beside a second plugin y that owns no dirty path and carries
// its own path-open frontier, over a root holding declaration as
// .corvint/runtime-readers.json (none when empty).
func runtimeReadersGraph(t *testing.T, declaration string, edit func(*fake)) (*Graph, error) {
	t.Helper()
	root := t.TempDir()
	if declaration != "" {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(RuntimeReadersPath)), []byte(declaration), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	language := chain()
	language.frontier = []string{"x:dynamic-dispatch"}
	if edit != nil {
		edit(&language)
	}
	other := fake{name: "y", units: []Unit{{ID: "y:app", Sources: []string{"app.y"}, Tests: []string{"app_test.y"}}}, frontier: []string{"y:dynamic-import"}}
	return Build(context.Background(), root, language, other)
}

func runtimeReadersPlan(t *testing.T, edit func(*fake), dirty ...string) Plan {
	t.Helper()
	graph, err := runtimeReadersGraph(t, runtimeReadersDeclaration, edit)
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

func frontierDetails(plan Plan) []string {
	details := []string{}
	for _, entry := range plan.Unknown {
		if entry.Reason == UnknownLanguageFrontier {
			details = append(details, entry.Detail)
		}
	}
	return details
}

// AFP-V0-602 (2), (3): a dirty path the repository declares as read only by
// the units the reader rules name raises neither UNOWNED_DIRTY_PATH nor, from
// a plugin that owns no dirty path and reaches no unit, LANGUAGE_FRONTIER; it
// is listed under DECLARED_RUNTIME_READERS with the entry's own reason. The
// same path undeclared keeps both (V1-2941, V1-2726).
func TestDeclaredRuntimeReadersIsNonSourceNotUnknown_AFPV0602(t *testing.T) {
	dirty := []string{".corvint/changes/0123.cem.json", "data/catalog.json"}
	plan := runtimeReadersPlan(t, nil, dirty...)
	if plan.Scope != ScopeBounded || len(plan.Unknown) != 0 {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, unknownDetails(plan))
	}
	if !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), dirty) || len(plan.NonSource) != 2 {
		t.Fatalf("nonSource=%+v", plan.NonSource)
	}
	if !strings.Contains(plan.NonSource[0].Reason, "sealed CEMs") || !strings.Contains(plan.NonSource[1].Reason, "embedded catalog") || !strings.HasPrefix(plan.NonSource[0].Reason, runtimeReadersReason) {
		t.Fatalf("reasons=%+v", plan.NonSource)
	}
	graph, err := runtimeReadersGraph(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	undeclared := Select(graph, dirty)
	if undeclared.Scope != ScopeUnknown || !slices.Contains(unknownDetails(undeclared), "UNOWNED_DIRTY_PATH data/catalog.json") || !slices.Contains(frontierDetails(undeclared), "y:dynamic-import") {
		t.Fatalf("undeclared: scope=%s unknown=%v", undeclared.Scope, unknownDetails(undeclared))
	}
	for _, outside := range []string{".corvint/changesx/a.cem.json", "data/catalog.json.bak", "data/other.json"} {
		plan := runtimeReadersPlan(t, nil, outside)
		if plan.Scope != ScopeUnknown || len(plan.NonSource) != 0 || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+outside) {
			t.Fatalf("%s: scope=%s unknown=%v nonSource=%v", outside, plan.Scope, unknownDetails(plan), plan.NonSource)
		}
	}
}

// AFP-V0-602 (3): every unit a reader rule names is still selected, and its
// own plugin's frontier still bears; only the plugin that neither reads nor
// is reached loses its fan-out. A path a plugin claims as source and a
// genuinely unowned path beside a declared one keep their entries.
func TestDeclaredRuntimeReadersKeepsEveryReader_AFPV0602(t *testing.T) {
	entry := ".corvint/changes/0123.cem.json"
	cases := []struct {
		name    string
		edit    func(*fake)
		witness string
	}{
		{"file literal", func(language *fake) { language.units[3].PathTokens = []string{entry} }, WitnessPathLiteralReader},
		{"declared read scope", func(language *fake) {
			language.units[3].Sources = []string{"solo/solo.x"}
			language.units[3].Tests = []string{"solo/solo_test.x"}
			language.units[3].ReadScoped = true
			language.units[3].ReadScope = []string{".corvint/changes/"}
		}, WitnessDeclaredReadScope},
		{"unbounded reader", func(language *fake) { language.units[3].UnboundedReads = "root-locator" }, WitnessUnboundedReader},
	}
	for _, tc := range cases {
		plan := runtimeReadersPlan(t, tc.edit, entry)
		if got := witnesses(plan)["x:solo"]; got.Kind != tc.witness || got.DirtyPath != entry {
			t.Fatalf("%s: selected=%+v", tc.name, plan.Selected)
		}
		if slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+entry) || slices.Contains(frontierDetails(plan), "y:dynamic-import") || !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), []string{entry}) {
			t.Fatalf("%s: unknown=%v nonSource=%v", tc.name, unknownDetails(plan), plan.NonSource)
		}
	}
	claimed := ".corvint/changes/tool.x"
	if plan := runtimeReadersPlan(t, nil, claimed); !slices.Contains(unknownDetails(plan), "UNINDEXED_SOURCE_PATH "+claimed) || len(plan.NonSource) != 0 {
		t.Fatalf("claimed source: unknown=%v nonSource=%v", unknownDetails(plan), plan.NonSource)
	}
	mixed := runtimeReadersPlan(t, nil, entry, "conformance/ledger.json")
	if mixed.Scope != ScopeUnknown || !slices.Contains(unknownDetails(mixed), "UNOWNED_DIRTY_PATH conformance/ledger.json") || slices.Contains(unknownDetails(mixed), "UNOWNED_DIRTY_PATH "+entry) || !slices.Contains(frontierDetails(mixed), "y:dynamic-import") {
		t.Fatalf("mixed: scope=%s unknown=%v", mixed.Scope, unknownDetails(mixed))
	}
}

// AFP-V0-602 (1): the declaration is closed and validated; an invalid one
// refuses the graph rather than classifying anything.
func TestRuntimeReadersDeclarationIsValidated_AFPV0602(t *testing.T) {
	for name, declaration := range map[string]string{
		"profile":          `{"profile":"corvint-runtime-readers/1","entries":[{"path":"a.json","reason":"r"}]}`,
		"unknown member":   `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"r","x":1}]}`,
		"missing entries":  `{"profile":"corvint-runtime-readers/0"}`,
		"empty reason":     `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":""}]}`,
		"control reason":   `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"a\u0007"}]}`,
		"unsorted":         `{"profile":"corvint-runtime-readers/0","entries":[{"path":"b.json","reason":"r"},{"path":"a.json","reason":"r"}]}`,
		"duplicate":        `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a.json","reason":"r"},{"path":"a.json","reason":"r"}]}`,
		"escaping":         `{"profile":"corvint-runtime-readers/0","entries":[{"path":"../a.json","reason":"r"}]}`,
		"itself":           `{"profile":"corvint-runtime-readers/0","entries":[{"path":".corvint/runtime-readers.json","reason":"r"}]}`,
		"declarations dir": `{"profile":"corvint-runtime-readers/0","entries":[{"path":".corvint/","reason":"r"}]}`,
		"documentation":    `{"profile":"corvint-runtime-readers/0","entries":[{"path":".corvint/documentation-paths.json","reason":"r"}]}`,
		"workflow":         `{"profile":"corvint-runtime-readers/0","entries":[{"path":".github/workflows/","reason":"r"}]}`,
		"github":           `{"profile":"corvint-runtime-readers/0","entries":[{"path":".github/","reason":"r"}]}`,
		"make file":        `{"profile":"corvint-runtime-readers/0","entries":[{"path":"build/rules.mk","reason":"r"}]}`,
		"root":             `{"profile":"corvint-runtime-readers/0","entries":[{"path":"/","reason":"r"}]}`,
	} {
		if _, err := runtimeReadersGraph(t, declaration, nil); err == nil || !strings.Contains(err.Error(), RuntimeReadersPath) {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if _, err := runtimeReadersGraph(t, `{"profile":"corvint-runtime-readers/0","entries":[]}`, nil); err != nil {
		t.Fatalf("empty entries: %v", err)
	}
}

// AFP-V0-602 (1): the declaration is in the graph digest when present and
// absent from the canonical form otherwise.
func TestRuntimeReadersDeclarationIsInTheDigest_AFPV0602(t *testing.T) {
	without, err := runtimeReadersGraph(t, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	with, err := runtimeReadersGraph(t, runtimeReadersDeclaration, nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := without.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), "runtimeReaders") || without.Digest() == with.Digest() {
		t.Fatalf("canonical=%s digests %s %s", canonical, without.Digest(), with.Digest())
	}
}

// AFP-V0-602 (4): a dirty declaration that no unit names or reaches is a
// CORVINT_DECLARATION, as AFP-V0-443 has it for the documentation declaration.
func TestRuntimeReadersDeclarationIsCorvintDeclaration_AFPV0602(t *testing.T) {
	plan := runtimeReadersPlan(t, nil, RuntimeReadersPath)
	if plan.Scope != ScopeBounded || !slices.Equal(groupPaths(plan, ClassCorvintDeclaration), []string{RuntimeReadersPath}) {
		t.Fatalf("scope=%s unknown=%v nonSource=%+v", plan.Scope, unknownDetails(plan), plan.NonSource)
	}
	plan = runtimeReadersPlan(t, func(language *fake) { language.units[3].PathTokens = []string{RuntimeReadersPath} }, RuntimeReadersPath)
	if plan.Scope != ScopeUnknown || !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH "+RuntimeReadersPath) {
		t.Fatalf("named: scope=%s unknown=%v", plan.Scope, unknownDetails(plan))
	}
}

// AFP-V0-602 (2): a make file below a declared directory stays unowned, and the
// declaration never changes which units are selected or excluded, or why; only
// the exclusions' universe, the graph digest, moves with it.
func TestDeclaredRuntimeReadersLeavesDriversAndSelectionAlone_AFPV0602(t *testing.T) {
	declaration := `{"profile":"corvint-runtime-readers/0","entries":[{"path":"build/","reason":"generated data"}]}`
	graph, err := runtimeReadersGraph(t, declaration, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"build/Makefile", "build/data.json"})
	if !slices.Contains(unknownDetails(plan), "UNOWNED_DIRTY_PATH build/Makefile") || !slices.Equal(groupPaths(plan, ClassDeclaredRuntimeReaders), []string{"build/data.json"}) {
		t.Fatalf("driver under a declared directory: unknown=%v nonSource=%+v", unknownDetails(plan), plan.NonSource)
	}
	edit := func(language *fake) {
		language.units[3].PathTokens = []string{".corvint/changes/0123.cem.json"}
		language.units[2].UnboundedReads = "root-locator"
	}
	undeclared, err := runtimeReadersGraph(t, "", edit)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := runtimeReadersGraph(t, runtimeReadersDeclaration, edit)
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{".corvint/changes/0123.cem.json", "data/catalog.json", "mid.x"}
	before, after := Select(undeclared, dirty), Select(declared, dirty)
	same := func(a, b Selection) bool {
		return a.UnitID == b.UnitID && a.Witness.Kind == b.Witness.Kind && a.Witness.DirtyPath == b.Witness.DirtyPath && slices.Equal(a.Witness.Via, b.Witness.Via)
	}
	if len(after.Selected) == 0 || !slices.EqualFunc(before.Selected, after.Selected, same) || !slices.EqualFunc(before.Excluded, after.Excluded, func(a, b Exclusion) bool { return a.UnitID == b.UnitID && a.Reason == b.Reason }) {
		t.Fatalf("selection changed:\nundeclared %+v\ndeclared   %+v", before.Selected, after.Selected)
	}
}
