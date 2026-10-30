package golang_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// nestedFixture is a root module with one tested package and an unlisted
// module at nested/ whose go.mod is the case under test.
func nestedFixture(t *testing.T, nestedManifest string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.test/root\n\ngo 1.26\n",
		"a/a.go":                 "package a\n",
		"a/a_test.go":            "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"nested/go.mod":          nestedManifest,
		"nested/n.go":            "package nested\n",
		"nested/n_test.go":       "package nested\n\nimport \"testing\"\n\nfunc TestN(t *testing.T) {}\n",
		"sibling/go.mod":         "module example.test/sibling\n\ngo 1.26\n",
		"sibling/s.go":           "package sibling\n",
		"a/testdata/fix/go.mod":  "module example.test/fixture\n\nrequire example.test/root v0.0.0\n",
		"a/testdata/fix/main.go": "package main\n",
	}
	for name, body := range extra {
		files[name] = body
	}
	writeFiles(t, root, files)
	return root
}

func nestedEvidence(t *testing.T, root string) map[string]string {
	t.Helper()
	evidence := map[string]string{}
	for _, judged := range golang.NestedModules(t, affected.DiskSource(root)) {
		evidence[judged.Manifest] = judged.Open
	}
	return evidence
}

func hasNestedFrontier(t *testing.T, root string) bool {
	t.Helper()
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	for _, reason := range result.Frontier {
		if reason == golang.FrontierNestedModule {
			return true
		}
	}
	return false
}

// nestedModuleUnitFrontier reports whether the plugin models nested/ as the
// dependent module unit go:example.test/nested carrying the nested frontier.
func nestedModuleUnitFrontier(t *testing.T, root string) bool {
	t.Helper()
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	for _, unit := range result.Units {
		if unit.ID == "go:example.test/nested" {
			return slices.Contains(unit.Frontier, golang.FrontierNestedModule) && len(unit.Sources) == 0 && len(unit.Tests) == 0
		}
	}
	return false
}

// TestIndependentNestedModulesCloseTheFrontier_V1_0867: unlisted modules whose
// manifests neither require nor replace the root module path cannot import a
// root package, so a root-only change is a bounded plan; the evidence names
// every nested go.mod read, and a testdata fixture module is not one of them.
// A change inside a nested module still has no unit and stays UNKNOWN.
func TestIndependentNestedModulesCloseTheFrontier_V1_0867(t *testing.T) {
	root := nestedFixture(t, strings.Join([]string{
		"// an independent module",
		"module example.test/nested",
		"",
		"go 1.26",
		"toolchain go1.27.1",
		"godebug default=go1.21",
		"",
		"require example.test/other v1.0.0",
		"require (",
		"\t\"example.test/quoted\" v1.0.0 // indirect",
		"\texample.test/pseudo v0.0.0-20240101000000-abcdef123456",
		"\texample.test/legacy v3.0.0+incompatible",
		"\texample.test/pre v1.2.3-rc.1",
		"\texample.test/major/v2 v2.0.0",
		"\tgopkg.in/yaml.v3 v3.0.1",
		"\tgopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405",
		"\texample.test/root/v2 v2.0.0",
		")",
		"replace example.test/other v1.0.0 => example.test/fork v1.0.1",
		"replace example.test/sibling => ../sibling",
		"exclude example.test/root v0.1.0",
		"retract [v0.1.0, v0.2.0] // a range",
		"tool example.test/nested/cmd/gen",
		"",
	}, "\n"), map[string]string{
		// The root draws in other modules, never the nested ones.
		"go.mod": "module example.test/root\n\ngo 1.26\n\nrequire example.test/other v1.0.0\n\nreplace example.test/other => example.test/fork v1.0.1\n\ntool example.test/other/cmd/gen\n",
	})
	want := map[string]string{"nested/go.mod": "", "sibling/go.mod": ""}
	if got := nestedEvidence(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("evidence = %q, want both nested manifests read and closed", got)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if frontier := graph.Frontier(); len(frontier) != 0 {
		t.Fatalf("frontier = %v, want none", frontier)
	}
	plan := affected.Select(graph, []string{"a/a.go"})
	if plan.Scope != affected.ScopeBounded || len(plan.Unknown) != 0 {
		t.Errorf("root-only plan scope = %s unknown = %+v, want BOUNDED with none", plan.Scope, plan.Unknown)
	}
	inside := affected.Select(graph, []string{"nested/n.go"})
	// A change inside the nested module stays UNKNOWN, named as that module's
	// own runnable test root (AFP-V0-191, AFP-V0-650) rather than as an
	// unindexed source path.
	wantInside := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: "go:unobserved-module:nested:nested/n.go"}}
	if inside.Scope != affected.ScopeUnknown || !reflect.DeepEqual(inside.Unknown, wantInside) {
		t.Errorf("nested-change plan scope = %s unknown = %+v, want UNKNOWN with %+v", inside.Scope, inside.Unknown, wantInside)
	}
}

// TestNestedModuleThatCanReachTheRootKeepsTheFrontier_V1_0867: a nested module
// that requires, replaces, or otherwise can build against the root module
// keeps the frontier open, and the evidence says why.
func TestNestedModuleThatCanReachTheRootKeepsTheFrontier_V1_0867(t *testing.T) {
	head := "module example.test/nested\n\ngo 1.26\n"
	rootHead := "module example.test/root\n\ngo 1.26\n\n"
	for _, testCase := range []struct {
		name     string
		manifest string
		extra    map[string]string
		want     string
		// dependent: open only by its own manifest (AFP-V0-582), so the
		// frontier is the module unit's, not the plugin's.
		dependent bool
	}{
		{"require", head + "require example.test/root v0.0.0\n", nil, "requires example.test/root", true},
		{"require block", head + "require (\n\texample.test/other v1.0.0\n\t\"example.test/root\" v0.0.0 // indirect\n)\n", nil, "requires example.test/root", true},
		{"replace root", head + "replace example.test/root v0.0.0 => example.test/fork v0.0.1\n", nil, "replaces example.test/root", true},
		{"replace root block", head + "replace (\n\texample.test/root => ../\n)\n", nil, "replaces example.test/root", true},
		{"relative replace to root", head + "replace example.test/alias => ../\n", nil, "replace target ../ resolves to ., which is not an unlisted module", false},
		{"relative replace into root module", head + "replace example.test/alias => ../a\n", nil, "replace target ../a resolves to a, which is not an unlisted module", false},
		{"replace outside repository", head + "replace example.test/alias => ../../elsewhere\n", nil, "replace target ../../elsewhere lies outside the repository", false},
		{"absolute replace", head + "replace example.test/alias => /srv/alias\n", nil, "replace target /srv/alias is not a repository-relative directory", false},
		{"replace with root", head + "replace example.test/alias v1.0.0 => example.test/root v0.0.0\n", nil, "replaces example.test/alias with example.test/root", false},
		{"root tool", head + "tool example.test/root/cmd/gen\n", nil, "names the tool example.test/root/cmd/gen", true},
		{"nested workspace", head, map[string]string{"nested/go.work": "go 1.26\n\nuse (\n\t.\n\t..\n)\n"}, "workspace nested/go.work may use an observed module", false},
		// The observed build itself can compile a nested module's packages,
		// which then resolve their imports of the root against it.
		{"root requires nested", head, map[string]string{"go.mod": rootHead + "require example.test/nested v0.0.0\n\nreplace example.test/nested => ./nested\n"}, "go.mod requires example.test/nested", false},
		{"root replaces nested", head, map[string]string{"go.mod": rootHead + "replace example.test/nested v0.0.0 => example.test/fork v0.0.1\n"}, "go.mod replaces example.test/nested", false},
		{"root replaces with nested directory", head, map[string]string{"go.mod": rootHead + "replace example.test/alias => ./nested\n"}, "go.mod replaces example.test/alias with ./nested", false},
		{"root tool in nested", head, map[string]string{"go.mod": rootHead + "tool example.test/nested/cmd/gen\n"}, "go.mod names the tool example.test/nested/cmd/gen", false},
		{"root workspace replaces nested", head, map[string]string{"go.work": "go 1.26\n\nuse .\n\nreplace example.test/nested => ./nested\n"}, "go.work replaces example.test/nested", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := nestedFixture(t, testCase.manifest, testCase.extra)
			if got := nestedEvidence(t, root)["nested/go.mod"]; got != testCase.want {
				t.Errorf("nested/go.mod open = %q, want %q", got, testCase.want)
			}
			if got := hasNestedFrontier(t, root); got == testCase.dependent {
				t.Errorf("plugin-wide frontier %v, want %v", got, !testCase.dependent)
			}
			if got := nestedModuleUnitFrontier(t, root); !reflect.DeepEqual(got, testCase.dependent) {
				t.Errorf("module unit with the nested frontier %v, want %v", got, testCase.dependent)
			}
		})
	}
}

// TestUnreadableOrUnparsableNestedManifestKeepsTheFrontier_V1_0867: a nested
// go.mod that cannot be read whole within the bound, or parsed, is uncertainty,
// never a closed frontier.
func TestUnreadableOrUnparsableNestedManifestKeepsTheFrontier_V1_0867(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		manifest string
		want     string
	}{
		{"unknown directive", "module example.test/nested\n\nrequires example.test/root v0.0.0\n", "unparsable: line 3: unknown directive \"requires\""},
		{"unterminated block", "module example.test/nested\n\nrequire (\n\texample.test/other v1.0.0\n", "unparsable: unterminated require block"},
		{"unterminated string", "module example.test/nested\n\nrequire \"example.test/root v0.0.0\n", "unparsable: line 3: unterminated string"},
		{"malformed require", "module example.test/nested\n\nrequire example.test/root\n", "unparsable: line 3: malformed require directive"},
		{"glued arrow", "module example.test/nested\n\nreplace example.test/alias=>../\n", "unparsable: line 3: malformed replace directive"},
		{"no module", "go 1.26\n", "unparsable: no module directive"},
		{"byte order mark", "\ufeffmodule example.test/nested\n", "unparsable: line 1: unexpected character '\\ufeff'"},
		{"invalid go version", "module example.test/nested\n\ngo nonsense\n", "unparsable: line 3: malformed go directive"},
		{"invalid toolchain", "module example.test/nested\n\ntoolchain 1.27\n", "unparsable: line 3: malformed toolchain directive"},
		{"godebug without value", "module example.test/nested\n\ngodebug panicnil\n", "unparsable: line 3: malformed godebug directive"},
		{"stray token in exclude block", "module example.test/nested\n\nexclude (\n\texample.test/root v0.1.0 extra\n)\n", "unparsable: line 4: malformed exclude directive"},
		{"retract without comma", "module example.test/nested\n\nretract [v0.1.0 v0.2.0]\n", "unparsable: line 3: malformed retract directive"},
		{"bracket in require", "module example.test/nested\n\nrequire example.test/other [v1.0.0]\n", "unparsable: line 3: misplaced \"[\" in require directive"},
		{"replace without version", "module example.test/nested\n\nreplace example.test/alias nonsense => ../sibling\n", "unparsable: line 3: malformed replace directive"},
		{"block comment", "module example.test/nested\n/* note */\n", "unparsable: line 2: block comment"},
		{"repeated go", "module example.test/nested\n\ngo 1.26\ngo 1.27\n", "unparsable: line 4: malformed go directive"},
		// The go tool matches these against the raw token, so quoting fails.
		{"quoted go version", "module example.test/nested\n\ngo \"1.26\"\n", "unparsable: line 3: malformed go directive"},
		{"quoted toolchain", "module example.test/nested\n\ntoolchain \"go1.27.1\"\n", "unparsable: line 3: malformed toolchain directive"},
		// Codex round 3: forms the go tool's parser refuses.
		{"raw string", "module example.test/nested\n\nrequire `example.test/other` v1.0.0\n", "unparsable: line 3: raw string"},
		{"quote inside an unquoted argument", "module example.test/nested\n\nrequire example.test/o'ther v1.0.0\n", "unparsable: line 3: quote in require directive"},
		{"go block", "module example.test/nested\n\ngo (\n\t1.26\n)\n", "unparsable: line 3: go block"},
		{"toolchain block", "module example.test/nested\n\ntoolchain (\n\tgo1.27.1\n)\n", "unparsable: line 3: toolchain block"},
		{"non-canonical version", "module example.test/nested\n\nrequire example.test/other vgarbage\n", "unparsable: line 3: malformed require directive"},
		{"version with a leading zero", "module example.test/nested\n\nrequire example.test/other v1.02.0\n", "unparsable: line 3: malformed require directive"},
		// Codex round 4: x/mod/module.CheckPathMajor and parseReplace forms.
		{"major without a path suffix", "module example.test/nested\n\nrequire example.test/other v2.0.0\n", "unparsable: line 3: malformed require directive"},
		{"major unlike the path suffix", "module example.test/nested\n\nrequire example.test/other/v2 v3.0.0\n", "unparsable: line 3: malformed require directive"},
		{"invalid v1 path suffix", "module example.test/nested\n\nrequire example.test/other/v1 v1.0.0\n", "unparsable: line 3: malformed require directive"},
		{"exclude major mismatch", "module example.test/nested\n\nexclude example.test/other v2.0.0\n", "unparsable: line 3: malformed exclude directive"},
		{"gopkg.in v0 unstable", "module example.test/nested\n\nrequire gopkg.in/yaml.v0-unstable v0.1.0\n", "unparsable: line 3: malformed require directive"},
		{"replace source major mismatch", "module example.test/nested\n\nreplace example.test/other v2.0.0 => example.test/fork v2.0.0\n", "unparsable: line 3: malformed replace directive"},
		{"directory replacement with a version", "module example.test/nested\n\nreplace example.test/other => ../sibling v1.0.0\n", "unparsable: line 3: malformed replace directive"},
		{"module replacement without a version", "module example.test/nested\n\nreplace example.test/other => example.test/fork\n", "unparsable: line 3: malformed replace directive"},
		{"quoted godebug", "module example.test/nested\n\ngodebug \"panicnil=1\"\n", "unparsable: line 3: malformed godebug directive"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := nestedFixture(t, testCase.manifest, nil)
			if got := nestedEvidence(t, root)["nested/go.mod"]; got != testCase.want {
				t.Errorf("nested/go.mod open = %q, want %q", got, testCase.want)
			}
			if !hasNestedFrontier(t, root) {
				t.Error("frontier closed, want go:nested-module-frontier")
			}
		})
	}
	t.Run("over-size", func(t *testing.T) {
		manifest := "module example.test/nested\n\ngo 1.26\n"
		root := nestedFixture(t, manifest, nil)
		golang.SetMaxNestedManifestBytes(t, int64(len(manifest)-1))
		want := fmt.Sprintf("over-size: %d bytes above the %d byte bound", len(manifest), len(manifest)-1)
		if got := nestedEvidence(t, root)["nested/go.mod"]; got != want {
			t.Errorf("nested/go.mod open = %q, want %q", got, want)
		}
		if !hasNestedFrontier(t, root) {
			t.Error("frontier closed, want go:nested-module-frontier")
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		root := nestedFixture(t, "module example.test/nested\n", nil)
		manifest := filepath.Join(root, "nested", "go.mod")
		if err := os.Chmod(manifest, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(manifest, 0o644) })
		if body, err := os.ReadFile(manifest); err == nil {
			t.Skipf("a mode 000 file is readable here (%d bytes); privileged runner", len(body))
		}
		if got := nestedEvidence(t, root)["nested/go.mod"]; !strings.HasPrefix(got, "unreadable: ") {
			t.Errorf("nested/go.mod open = %q, want unreadable", got)
		}
		if !hasNestedFrontier(t, root) {
			t.Error("frontier closed, want go:nested-module-frontier")
		}
	})
	// What the observed side says must be read the same way, or nothing about
	// the nested modules can be ruled out.
	for _, testCase := range []struct {
		name  string
		extra map[string]string
		want  string
	}{
		{"unparsable root manifest", map[string]string{"go.mod": "module example.test/root\n\ngo nonsense\n"}, "observed manifest go.mod is unparsable: line 3: malformed go directive"},
		{"root replacement outside the repository", map[string]string{"go.mod": "module example.test/root\n\nreplace example.test/alias => ../elsewhere\n"}, "go.mod: replace target ../elsewhere lies outside the repository"},
		// Only an exact module directory establishes identity: a differently
		// cased or linked path can name the nested module on some filesystems.
		{"root replacement with a differently cased directory", map[string]string{"go.mod": "module example.test/root\n\nrequire example.test/alias v0.0.0\n\nreplace example.test/alias => ./NESTED\n"}, "go.mod replaces example.test/alias with ./NESTED, which names no known module directory"},
		{"workspace replacement with an unknown directory", map[string]string{"go.work": "go 1.26\n\nuse .\n\nreplace example.test/alias => ./link\n"}, "go.work replaces example.test/alias with ./link, which names no known module directory"},
		{"root workspace without uses", map[string]string{"go.work": "go 1.26\n"}, `root go.work uses [], observed as ["."]`},
		{"unparsable root workspace", map[string]string{"go.work": "go 1.26\n\nuse .\nmodule example.test/root\n"}, "root go.work is unparsable: line 4: unknown directive \"module\""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := nestedFixture(t, "module example.test/nested\n", testCase.extra)
			evidence := nestedEvidence(t, root)
			if len(evidence) == 0 {
				t.Fatal("no nested evidence")
			}
			for manifest, open := range evidence {
				if open != testCase.want {
					t.Errorf("%s open = %q, want %q", manifest, open, testCase.want)
				}
			}
			if !hasNestedFrontier(t, root) {
				t.Error("frontier closed, want go:nested-module-frontier")
			}
		})
	}
	t.Run("root module path unresolved", func(t *testing.T) {
		root := nestedFixture(t, "module example.test/nested\n", map[string]string{"go.mod": "go 1.26\n"})
		if got := nestedEvidence(t, root)["nested/go.mod"]; got != "an observed module path is unresolved" {
			t.Errorf("nested/go.mod open = %q, want the unresolved observed path", got)
		}
		if !hasNestedFrontier(t, root) {
			t.Error("frontier closed, want go:nested-module-frontier")
		}
	})
}

// TestNestedFrontierReadsTheSuppliedSource_V1_0867: the decision reads nested
// manifests through the plugin's source, so an immutable tree decides it, not
// whatever the worktree holds. The tree admits bounded opens, as a Git
// revision does: a plain fs.FS cannot, and UnitsSource now surfaces that
// refusal instead of reading past it (V1-1416).
func TestNestedFrontierReadsTheSuppliedSource_V1_0867(t *testing.T) {
	tree := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.test/root\n")},
		"a/a.go":        {Data: []byte("package a\n")},
		"nested/go.mod": {Data: []byte("module example.test/nested\n")},
		"nested/n.go":   {Data: []byte("package nested\n")},
	}
	closed, err := golang.New().UnitsSource(affected.FSSource(&budgetFS{files: tree, limit: 1 << 30}))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	if slices.Contains(closed.Frontier, golang.FrontierNestedModule) {
		t.Errorf("independent tree frontier = %v, want no %s", closed.Frontier, golang.FrontierNestedModule)
	}
	tree["nested/go.mod"] = &fstest.MapFile{Data: []byte("module example.test/nested\n\nrequire example.test/root v0.0.0\n")}
	open, err := golang.New().UnitsSource(affected.FSSource(&budgetFS{files: tree, limit: 1 << 30}))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	// It is open only by its own manifest, so the frontier is its module
	// unit's (AFP-V0-582).
	unitFrontier := false
	for _, unit := range open.Units {
		unitFrontier = unitFrontier || (unit.ID == "go:example.test/nested" && slices.Contains(unit.Frontier, golang.FrontierNestedModule))
	}
	if slices.Contains(open.Frontier, golang.FrontierNestedModule) || !unitFrontier {
		t.Errorf("requiring tree frontier = %v, module unit frontier %v, want only the unit's %s", open.Frontier, unitFrontier, golang.FrontierNestedModule)
	}
}

// TestEscapedRootModulePathNamesUnitsByItsValue_V1_0873: a root go.mod whose
// quoted module path carries an escape names units by the unquoted path, as
// the Go toolchain does, not by the literal escaped text. It replaces the
// V1-0867 escaped-root row of the case above: both sides now read the same
// path, so that mismatch no longer holds the nested frontier open.
func TestEscapedRootModulePathNamesUnitsByItsValue_V1_0873(t *testing.T) {
	tree := fstest.MapFS{
		"go.mod": {Data: []byte("module \"example.test/ro\\x6ft\"\n")},
		"a/a.go": {Data: []byte("package a\n")},
	}
	result, err := golang.New().UnitsSource(affected.FSSource(&budgetFS{files: tree, limit: 1 << 30}))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	ids := []string{}
	for _, unit := range result.Units {
		ids = append(ids, unit.ID)
	}
	if !slices.Contains(ids, "go:example.test/root/a") {
		t.Fatalf("unit ids = %v, want go:example.test/root/a", ids)
	}
	if slices.Contains(result.Frontier, golang.FrontierModulePath) {
		t.Fatalf("frontier = %v, an escaped path that unquotes is not unresolved", result.Frontier)
	}
}

// TestDependentNestedModuleFrontierBearsOnlyOnWhatItCanSee_AFP_V0_582: a
// nested module open only because its own manifest requires and replaces the
// root module names its frontier on a plan that changes a root package it
// imports, a root manifest, or the module itself, and no longer on one that
// changes a root package it cannot see. A module file that does not parse, or
// one that locates the root or carries a literal escaping its package, keeps
// the frontier on every such plan; a manifest tool or a literal naming a root
// package is an edge to it.
func TestDependentNestedModuleFrontierBearsOnlyOnWhatItCanSee_AFP_V0_582(t *testing.T) {
	manifest := "module example.test/nested\n\ngo 1.26\n\nrequire example.test/root v0.0.0\n\nreplace example.test/root => ../\n"
	plan := func(t *testing.T, extra map[string]string, dirty string) (string, bool) {
		t.Helper()
		files := map[string]string{
			"b/b.go":      "package b\n",
			"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
			"nested/n.go": "package nested\n\nimport _ \"example.test/root/a\"\n",
		}
		for name, body := range extra {
			files[name] = body
		}
		root := nestedFixture(t, manifest, files)
		graph, err := affected.Build(context.Background(), root, golang.New())
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		selected := affected.Select(graph, []string{dirty})
		for _, unknown := range selected.Unknown {
			if unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == golang.FrontierNestedModule {
				return selected.Scope, true
			}
		}
		return selected.Scope, false
	}
	for _, testCase := range []struct {
		name  string
		extra map[string]string
		dirty string
		scope string
		bears bool
	}{
		{"imported root package", nil, "a/a.go", affected.ScopeUnknown, true},
		{"unrelated root package", nil, "b/b.go", affected.ScopeBounded, false},
		{"module itself", nil, "nested/n.go", affected.ScopeUnknown, false},
		{"unparsable module file", map[string]string{"nested/bad.go": "package nested\n\nimport \"unterminated\n"}, "b/b.go", affected.ScopeUnknown, true},
		{"root locator", map[string]string{"nested/loc.go": "package nested\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n"}, "b/b.go", affected.ScopeUnknown, true},
		{"escaping literal", map[string]string{"nested/walk_test.go": "package nested\n\nimport (\n\t\"io/fs\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestWalk(t *testing.T) {\n\t_ = filepath.WalkDir(\"..\", func(string, fs.DirEntry, error) error { return nil })\n}\n"}, "b/b.go", affected.ScopeUnknown, true},
		{"manifest tool", map[string]string{"nested/go.mod": manifest + "\ntool example.test/root/b\n"}, "b/b.go", affected.ScopeUnknown, true},
		{"literal naming a root package", map[string]string{"nested/run_test.go": "package nested\n\nimport \"os/exec\"\n\nvar _ = exec.Command(\"go\", \"run\", \"example.test/root/b\")\n"}, "b/b.go", affected.ScopeUnknown, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scope, bears := plan(t, testCase.extra, testCase.dirty)
			if scope != testCase.scope || bears != testCase.bears {
				t.Errorf("dirty %s: scope %s nested frontier %v, want %s %v", testCase.dirty, scope, bears, testCase.scope, testCase.bears)
			}
		})
	}
}

// STR-V0-069: only the synthetic AFP-V0-582 unit of a dependent nested module
// is a DependentModuleUnit; a root package is not.
func TestDependentModuleUnitNamesOnlyTheSyntheticUnit_STR_V0_069(t *testing.T) {
	root := nestedFixture(t, "module example.test/nested\n\ngo 1.26\n\nrequire example.test/root v0.0.0\n\nreplace example.test/root => ../\n", map[string]string{
		"nested/n.go": "package nested\n\nimport _ \"example.test/root/a\"\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	dependent := []string{}
	for _, id := range graph.UnitIDs() {
		if unit, _ := graph.Unit(id); golang.DependentModuleUnit(unit) {
			dependent = append(dependent, id)
		}
	}
	if len(dependent) != 1 || dependent[0] != "go:example.test/nested" {
		t.Fatalf("dependent module units %q of %q", dependent, graph.UnitIDs())
	}
}
