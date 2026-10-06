package golang_test

import (
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
	result, err := golang.New().Units(root)
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
		"\t`example.test/raw` v1.0.0",
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
	graph, err := affected.Build(root, golang.New())
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
	wantInside := []affected.Unknown{{Reason: affected.UnknownUnindexedSourcePath, Detail: "nested/n.go"}}
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
	}{
		{"require", head + "require example.test/root v0.0.0\n", nil, "requires example.test/root"},
		{"require block", head + "require (\n\texample.test/other v1.0.0\n\t\"example.test/root\" v0.0.0 // indirect\n)\n", nil, "requires example.test/root"},
		{"replace root", head + "replace example.test/root v0.0.0 => example.test/fork v0.0.1\n", nil, "replaces example.test/root"},
		{"replace root block", head + "replace (\n\texample.test/root => ../\n)\n", nil, "replaces example.test/root"},
		{"relative replace to root", head + "replace example.test/alias => ../\n", nil, "replace target ../ resolves to ., which is not an unlisted module"},
		{"relative replace into root module", head + "replace example.test/alias => ../a\n", nil, "replace target ../a resolves to a, which is not an unlisted module"},
		{"replace outside repository", head + "replace example.test/alias => ../../elsewhere\n", nil, "replace target ../../elsewhere lies outside the repository"},
		{"absolute replace", head + "replace example.test/alias => /srv/alias\n", nil, "replace target /srv/alias is not a repository-relative directory"},
		{"replace with root", head + "replace example.test/alias v1.0.0 => example.test/root v0.0.0\n", nil, "replaces example.test/alias with example.test/root"},
		{"root tool", head + "tool example.test/root/cmd/gen\n", nil, "names the tool example.test/root/cmd/gen"},
		{"nested workspace", head, map[string]string{"nested/go.work": "go 1.26\n\nuse (\n\t.\n\t..\n)\n"}, "workspace nested/go.work may use an observed module"},
		// The observed build itself can compile a nested module's packages,
		// which then resolve their imports of the root against it.
		{"root requires nested", head, map[string]string{"go.mod": rootHead + "require example.test/nested v0.0.0\n\nreplace example.test/nested => ./nested\n"}, "go.mod requires example.test/nested"},
		{"root replaces nested", head, map[string]string{"go.mod": rootHead + "replace example.test/nested v0.0.0 => example.test/fork v0.0.1\n"}, "go.mod replaces example.test/nested"},
		{"root replaces with nested directory", head, map[string]string{"go.mod": rootHead + "replace example.test/alias => ./nested\n"}, "go.mod replaces example.test/alias with ./nested"},
		{"root tool in nested", head, map[string]string{"go.mod": rootHead + "tool example.test/nested/cmd/gen\n"}, "go.mod names the tool example.test/nested/cmd/gen"},
		{"root workspace replaces nested", head, map[string]string{"go.work": "go 1.26\n\nuse .\n\nreplace example.test/nested => ./nested\n"}, "go.work replaces example.test/nested"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := nestedFixture(t, testCase.manifest, testCase.extra)
			if got := nestedEvidence(t, root)["nested/go.mod"]; got != testCase.want {
				t.Errorf("nested/go.mod open = %q, want %q", got, testCase.want)
			}
			if !hasNestedFrontier(t, root) {
				t.Error("frontier closed, want go:nested-module-frontier")
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
		{"escaped root module path", map[string]string{"go.mod": "module \"example.test/ro\\x6ft\"\n"}, `observed manifest go.mod declares "example.test/root", read as "example.test/ro\\x6ft"`},
		{"unparsable root manifest", map[string]string{"go.mod": "module example.test/root\n\ngo nonsense\n"}, "observed manifest go.mod is unparsable: line 3: malformed go directive"},
		{"root replacement outside the repository", map[string]string{"go.mod": "module example.test/root\n\nreplace example.test/alias => ../elsewhere\n"}, "go.mod: replace target ../elsewhere lies outside the repository"},
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
// whatever the worktree holds.
func TestNestedFrontierReadsTheSuppliedSource_V1_0867(t *testing.T) {
	tree := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.test/root\n")},
		"a/a.go":        {Data: []byte("package a\n")},
		"nested/go.mod": {Data: []byte("module example.test/nested\n")},
		"nested/n.go":   {Data: []byte("package nested\n")},
	}
	closed, err := golang.New().UnitsSource(affected.FSSource(tree))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	if slices.Contains(closed.Frontier, golang.FrontierNestedModule) {
		t.Errorf("independent tree frontier = %v, want no %s", closed.Frontier, golang.FrontierNestedModule)
	}
	tree["nested/go.mod"] = &fstest.MapFile{Data: []byte("module example.test/nested\n\nrequire example.test/root v0.0.0\n")}
	open, err := golang.New().UnitsSource(affected.FSSource(tree))
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	if !slices.Contains(open.Frontier, golang.FrontierNestedModule) {
		t.Errorf("requiring tree frontier = %v, want %s", open.Frontier, golang.FrontierNestedModule)
	}
}
