package golang_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// nativeFrontier is a plugin that owns only C sources and always reports a frontier, as the
// cfamily, dotnet, ruby and swift plugins do: it owns no unowned data path and reaches no Go unit.
type nativeFrontier struct{}

func (nativeFrontier) Name() string { return "native" }

func (nativeFrontier) Owns(path string) bool { return strings.HasSuffix(path, ".c") }

func (nativeFrontier) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{
		Units:    []affected.Unit{{ID: "native:lib", Sources: []string{"native/lib.c"}, Tests: []string{"native/lib_test.c"}}},
		Frontier: []string{"native:link-graph-unmodelled"},
	}, nil
}

// V1-3773: a LANGUAGE_FRONTIER that bears only because no plugin owns the dirty path, and so
// does not force FULL, cannot hide a Go test that reads that path. Each way a Go test reads an
// unowned repository file selects the test's package by a rule of its own: a path literal (rule
// (c)), a file below its own directory (rule (b)), a root it locates at run time joined to a name
// from data (rule (d), selected on every dirty path), and a declared read scope (AFP-V0-023),
// outside which the confined CI shard denies the read so it fails rather than passes stale. The
// one read these rules cannot see is a stat of a data-derived path, which Landlock does not
// confine; AFP-V0-980/981 (V1-3772) add the declared existence readers and the lint for it.
func TestUnownedFrontierCannotHideAGoReader_V13773(t *testing.T) {
	root := t.TempDir()
	parent := "." + "."
	writeFiles(t, root, map[string]string{
		"go.mod":                         "module example.test/m\n",
		"native/lib.c":                   "int lib(void) { return 1; }\n",
		"native/lib_test.c":              "int main(void) { return 0; }\n",
		"data/catalog.txt":               "catalog\n",
		"docs/scope.txt":                 "scope\n",
		"notes/any.txt":                  "any\n",
		"lit/lit.go":                     "package lit\n",
		"lit/lit_test.go":                "package lit\n\nimport \"testing\"\n\nvar catalog = \"data/catalog.txt\"\n\nfunc TestCatalog(t *testing.T) { _ = catalog }\n",
		"encl/encl.go":                   "package encl\n",
		"encl/encl_test.go":              "package encl\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestTable(t *testing.T) { _, _ = os.ReadFile(\"testdata/table.txt\") }\n",
		"encl/testdata/table.txt":        "table\n",
		"comp/comp.go":                   "package comp\n",
		"comp/comp_test.go":              "package comp\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestNamed(t *testing.T) {\n\troot, _ := os.Getwd()\n\tfor _, name := range []string{os.Getenv(\"NAME\")} {\n\t\t_, _ = os.ReadFile(filepath.Join(root, name))\n\t}\n}\n",
		"scoped/scoped.go":               "package scoped\n",
		"scoped/scoped_test.go":          "package scoped\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestScoped(t *testing.T) {\n\tfor _, name := range []string{os.Getenv(\"NAME\")} {\n\t\t_, _ = os.ReadFile(filepath.Join(\"" + parent + "\", \"docs\", name))\n\t}\n}\n",
		"plain/plain.go":                 "package plain\n",
		"plain/plain_test.go":            "package plain\n\nimport \"testing\"\n\nfunc TestPlain(t *testing.T) {}\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":["docs/"]}}`,
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), nativeFrontier{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty, unit, witness string
	}{
		{"data/catalog.txt", "go:example.test/m/lit", affected.WitnessPathLiteralReader},
		{"encl/testdata/table.txt", "go:example.test/m/encl", affected.WitnessEnclosingPackage},
		{"notes/any.txt", "go:example.test/m/comp", affected.WitnessUnboundedReader},
		{"docs/scope.txt", "go:example.test/m/scoped", affected.WitnessDeclaredReadScope},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		bears := false
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == "native:link-graph-unmodelled" && strings.Contains(unknown.Hint, "bears only because no plugin owns the dirty path") {
				bears = true
			}
		}
		if !bears {
			t.Fatalf("%s: the native frontier does not bear as unowned-only: %+v", tc.dirty, plan.Unknown)
		}
		witness := ""
		for _, selection := range plan.Selected {
			if selection.UnitID == tc.unit {
				witness = selection.Witness.Kind
			}
			if selection.UnitID == "go:example.test/m/plain" || strings.HasPrefix(selection.UnitID, "native:") {
				t.Errorf("%s: selected %s, which reads nothing", tc.dirty, selection.UnitID)
			}
		}
		if witness != tc.witness {
			t.Errorf("%s: %s witness = %q, want %s; selected %v", tc.dirty, tc.unit, witness, tc.witness, plan.Selected)
		}
	}
}
