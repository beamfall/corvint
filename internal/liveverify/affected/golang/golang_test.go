package golang_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/testsupport"
)

func TestPackagesUnderBuildOutputDirectoryNamesAreSelected(t *testing.T) {
	root := fixtureRoot(t, "directory-names")
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"core/core.go"})
	want := map[string]bool{
		"core/core_test.go":            true,
		"nested/build/build_test.go":   true,
		"nested/dist/dist_test.go":     true,
		"nested/target/target_test.go": true,
	}
	for _, selected := range plan.SelectedTests() {
		delete(want, selected)
	}
	if len(want) != 0 {
		t.Fatalf("selected=%v missing=%v", plan.SelectedTests(), want)
	}
	if plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestIncludedDirectoryWalkBoundWidensInsteadOfRefusing_AFPV0008(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":            "module example.test/bounded\n\ngo 1.27.0\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dist := filepath.Join(root, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < affected.MaxIncludedDirectoryEntries; index++ {
		name := filepath.Join(dist, fmt.Sprintf("entry-%05d", index))
		if err := os.WriteFile(name, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatalf("included directory bound refused graph: %v", err)
	}
	plan := affected.Select(graph, []string{"core/core.go"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == "go:included-directory-walk-bounded" {
			return
		}
	}
	t.Fatalf("included-directory frontier missing: %v", plan.Unknown)
}

func TestNoGoRepositoryProducesNoUnitsOrFrontier(t *testing.T) {
	result, err := golang.New().Units(context.Background(), fixtureRoot(t, "no-go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 0 || len(result.Frontier) != 0 {
		t.Fatalf("result=%+v", result)
	}
}

func TestDeletedGoSourceSelectsItsPackageAndImporters_V1_0340(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixtureRoot(t, "directory-names"))); err != nil {
		t.Fatal(err)
	}
	deletedPath := filepath.Join(root, "core", "deleted.go")
	if err := os.WriteFile(deletedPath, []byte("package core\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, gitExecutable, root, "init", "--quiet")
	runGit(t, gitExecutable, root, "add", ".")
	runGit(t, gitExecutable, root, "commit", "--quiet", "-m", "baseline")
	if err := os.Remove(deletedPath); err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	dirty, err := affected.DirtyPaths(context.Background(), gitExecutable, root)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, dirty)
	want := "[core/core_test.go nested/build/build_test.go nested/dist/dist_test.go nested/target/target_test.go]"
	if got := fmt.Sprint(plan.SelectedTests()); got != want {
		t.Fatalf("selected tests = %s, want %s (the package that lost a file and its importers)", got, want)
	}
	reasons := make(map[string]string, len(plan.Excluded))
	for _, exclusion := range plan.Excluded {
		reasons[exclusion.UnitID] = exclusion.Reason
	}
	if got := reasons["go:example.test/directorynames/other"]; got != affected.ExcludedNoDependencyPath {
		t.Fatalf("unrelated exclusion reason=%q", got)
	}
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

var runGit = testsupport.RunGit

func fixtureRoot(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// A file below testdata is fixture data the go tool never builds: it forms no
// unit, a fixture go.mod is no nested-module frontier, and a fixture edit is an
// unowned data path, not an untested package (V1-0203).
func TestTestdataIsFixtureDataNotAPackage_AFPV0008(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                         "module example.test/m\n",
		"pkg/pkg.go":                     "package pkg\n",
		"pkg/pkg_test.go":                "package pkg\n",
		"pkg/testdata/fixture.go":        "package fixture\n",
		"pkg/testdata/module/go.mod":     "module example.test/fixture\n",
		"pkg/testdata/module/fixture.go": "package fixture\n",
		"testdata/top/top.go":            "package top\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].ID != "go:example.test/m/pkg" {
		t.Fatalf("units=%+v", result.Units)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	// The fixture is no unit's source, but it is data its hosting package's
	// tests read; that package is the only Go unit with tests and rule (b)
	// selects it, so it is attributed and raises no UNOWNED_DIRTY_PATH
	// (AFP-V0-601).
	fixture := affected.Select(graph, []string{"pkg/testdata/fixture.go"})
	if fmt.Sprint(fixture.SelectedTests()) != "[pkg/pkg_test.go]" || len(fixture.Unknown) != 0 {
		t.Fatalf("fixture edit selected=%v scope=%s unknown=%v", fixture.SelectedTests(), fixture.Scope, fixture.Unknown)
	}
	top := affected.Select(graph, []string{"testdata/top/top.go"})
	want := []affected.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "testdata/top/top.go"}}
	if fmt.Sprint(top.Unknown) != fmt.Sprint(want) || top.Scope != affected.ScopeUnknown {
		t.Fatalf("unhosted fixture edit scope=%s unknown=%v", top.Scope, top.Unknown)
	}
	source := affected.Select(graph, []string{"pkg/pkg.go"})
	if fmt.Sprint(source.SelectedTests()) != "[pkg/pkg_test.go]" || source.Scope != affected.ScopeBounded {
		t.Fatalf("source edit selected=%v scope=%s unknown=%v", source.SelectedTests(), source.Scope, source.Unknown)
	}
}

// The testdata rule is relative to the owning module, as the go tool applies
// it: a workspace module whose own directory lies below testdata is observed.
func TestWorkspaceModuleBelowTestdataIsObserved_AFPV0008(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.work":                   "go 1.22\n\nuse ./testdata/ws\n",
		"testdata/ws/go.mod":        "module example.test/ws\n",
		"testdata/ws/ws.go":         "package ws\n",
		"testdata/ws/testdata/x.go": "package x\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].ID != "go:example.test/ws" {
		t.Fatalf("units=%+v", result.Units)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	// Owns is repository-relative: an unindexed file of this module is disowned,
	// which labels it UNOWNED_DIRTY_PATH but still widens the plan to UNKNOWN.
	deleted := affected.Select(graph, []string{"testdata/ws/gone.go"})
	want := []affected.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "testdata/ws/gone.go"}}
	if fmt.Sprint(deleted.Unknown) != fmt.Sprint(want) || deleted.Scope != affected.ScopeUnknown {
		t.Fatalf("deleted workspace file scope=%s unknown=%v", deleted.Scope, deleted.Unknown)
	}
}

// AFP-V0-021: a dirty path no plugin owns selects the package whose own files
// name it by literal, witnessed as PATH_LITERAL_READER, and stays unknown. An
// import path is an edge, never a token; a dependent of the reader and a path
// no literal names select nothing.
func TestPathLiteralSelectsItsReaderPackage_AFPV0021(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":            "module example.test/m\n",
		"docs/guide.md":     "guide\n",
		"pkg/pkg.go":        "package pkg\n\nimport _ \"example.test/m/other\"\n",
		"pkg/pkg_test.go":   "package pkg\n\nconst guide, data = \"../docs/guide.md\", \"example.test/m/data/%s.json\"\n",
		"pkg/block.go":      "package pkg\n\nimport (\n\t_ \"example.test/m/other\"\n\t_ \"strings\"\n)\n\nvar asset = \"assets/logo.svg\"\n",
		"pkg/bare.go":       "package pkg\n\nvar table = \"tables/rows.txt\"\n",
		"other/other.go":    "package other\n",
		"user/user.go":      "package user\n\nimport _ \"example.test/m/pkg\"\n",
		"user/user_test.go": "package user\n",
	})
	result, err := golang.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, unit := range result.Units {
		if unit.ID != "go:example.test/m/pkg" {
			continue
		}
		found = true
		if fmt.Sprint(unit.PathTokens) != "[../docs/guide.md .json /data/ assets/logo.svg tables/rows.txt]" {
			t.Fatalf("pkg path tokens=%q", unit.PathTokens)
		}
	}
	if !found {
		t.Fatalf("no pkg unit in %v", result.Units)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	named := affected.Select(graph, []string{"docs/guide.md"})
	want := []affected.Selection{{UnitID: "go:example.test/m/pkg", Tests: []string{"pkg/pkg_test.go"}, Witness: affected.Witness{Kind: affected.WitnessPathLiteralReader, DirtyPath: "docs/guide.md", Via: []string{"go:example.test/m/pkg"}}}}
	unknown := []affected.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "docs/guide.md"}}
	if fmt.Sprint(named.Selected) != fmt.Sprint(want) || fmt.Sprint(named.Unknown) != fmt.Sprint(unknown) || named.Scope != affected.ScopeUnknown {
		t.Fatalf("named path selected=%v unknown=%v scope=%s", named.Selected, named.Unknown, named.Scope)
	}
	if reader := affected.Select(graph, []string{"data/x.json"}); len(reader.Selected) != 1 {
		t.Fatalf("a module-anchored literal must name its path: %v", reader.Selected)
	}
	unnamed := affected.Select(graph, []string{"notes/notes.md"})
	if len(unnamed.Selected) != 0 || fmt.Sprint(unnamed.Unknown) != "[{UNOWNED_DIRTY_PATH notes/notes.md}]" {
		t.Fatalf("unnamed path selected=%v unknown=%v", unnamed.Selected, unnamed.Unknown)
	}
}

// AFP-V0-021: a package over the token bound keeps no tokens and is unknown in
// a plan that matched a dirty path, rather than widening every plan.
func TestPathTokenBoundNamesThePackage(t *testing.T) {
	golang.SetMaxPathTokens(t, 2)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":            "module example.test/m\n",
		"big/big.go":        "package big\n\nvar a, b, c = \"a.txt\", \"b.txt\", \"c.txt\"\n",
		"big/big_test.go":   "package big\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if clean := affected.Select(graph, nil); len(clean.Unknown) != 0 {
		t.Fatalf("clean plan unknown=%v", clean.Unknown)
	}
	plan := affected.Select(graph, []string{"core/core.go"})
	want := "[{LANGUAGE_FRONTIER go:path-token-bound:go:example.test/m/big}]"
	if fmt.Sprint(plan.Unknown) != want || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("unknown=%v scope=%s", plan.Unknown, plan.Scope)
	}
}

// AFP-V0-021: a Go file that does not lex raises go:unparsed-source, except
// under a directory the go tool ignores.
func TestUnlexableSourceIsAFrontierOutsideIgnoredDirectories(t *testing.T) {
	unlexable := "package p\n\nvar s = \"unterminated\n"
	cases := map[string]bool{"_scratch/p.go": false, "fixtures/testdata/p.go": false, "p/p.go": true}
	for file, raised := range cases {
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"go.mod": "module example.test/m\n", file: unlexable})
		result, err := golang.New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(fmt.Sprint(result.Frontier), golang.FrontierUnparsedSource); got != raised {
			t.Fatalf("%s frontier=%v want raised=%v", file, result.Frontier, raised)
		}
	}
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, body := range files {
		target := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// V1-0291: a test-only import selects the importing package's tests and stops
// there, as `go list -deps -test` does: an importer of that package never
// compiles its tests. A real dependency chain through the helper still reaches
// every package whose build includes it.
func TestTestOnlyImportSelectsTheTestUserButNotItsImporters(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":           "module example.test/m\n",
		"helper/h.go":      "package helper\n",
		"user/u.go":        "package user\n",
		"user/u_test.go":   "package user\n\nimport _ \"example.test/m/helper\"\n",
		"top/t.go":         "package top\n\nimport _ \"example.test/m/user\"\n",
		"top/t_test.go":    "package top\n",
		"real/r.go":        "package real\n\nimport _ \"example.test/m/helper\"\n",
		"real/r_test.go":   "package real\n\nimport _ \"example.test/m/helper\"\n",
		"deeper/d.go":      "package deeper\n\nimport _ \"example.test/m/real\"\n",
		"deeper/d_test.go": "package deeper\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	user, _ := graph.Unit("go:example.test/m/user")
	if fmt.Sprint(user.Imports, user.TestImports) != "[] [go:example.test/m/helper]" {
		t.Fatalf("user imports=%v testImports=%v", user.Imports, user.TestImports)
	}
	real, _ := graph.Unit("go:example.test/m/real")
	if fmt.Sprint(real.Imports, real.TestImports) != "[go:example.test/m/helper] []" {
		t.Fatalf("real imports=%v testImports=%v, want a shared import to stay an ordinary edge", real.Imports, real.TestImports)
	}
	plan := affected.Select(graph, []string{"helper/h.go"})
	want := "[deeper/d_test.go real/r_test.go user/u_test.go]"
	if got := fmt.Sprint(plan.SelectedTests()); got != want {
		t.Fatalf("selected tests = %s, want %s (top only imports user's non-test files)", got, want)
	}
	for _, selection := range plan.Selected {
		if selection.UnitID == "go:example.test/m/user" && fmt.Sprint(selection.Witness.Via) != "[go:example.test/m/helper go:example.test/m/user]" {
			t.Errorf("test user witness = %+v", selection.Witness)
		}
	}
	for _, exclusion := range plan.Excluded {
		if exclusion.UnitID == "go:example.test/m/top" && exclusion.Reason == affected.ExcludedNoDependencyPath {
			return
		}
	}
	t.Errorf("excluded = %+v, want top excluded with no dependency path", plan.Excluded)
}

// V1-0290: an unanchored one-component token names a file, never a directory,
// so the `internal/` of `"internal/%03d.go"` does not make its package a reader
// of every path under an `internal` directory. Two-component and file-name
// tokens still select their real readers.
func TestDirectoryShapedLiteralNamesNoPath(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                  "module example.test/m\n",
		"noise/n.go":              "package noise\n\nvar pattern = \"internal/%03d.go\"\n",
		"noise/n_test.go":         "package noise\n",
		"joined/j.go":             "package joined\n\nvar rows = \"store/rows.txt\"\n",
		"joined/j_test.go":        "package joined\n",
		"named/n.go":              "package named\n\nvar rows, build = \"rows.txt\", \"Makefile\"\n",
		"named/n_test.go":         "package named\n",
		"internal/store/s.go":     "package store\n",
		"internal/store/rows.txt": "rows\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"Makefile", "internal/store/rows.txt"})
	selected := make([]string, 0, len(plan.Selected))
	for _, selection := range plan.Selected {
		selected = append(selected, selection.UnitID+"<-"+selection.Witness.DirtyPath)
	}
	want := "[go:example.test/m/joined<-internal/store/rows.txt go:example.test/m/named<-Makefile]"
	if got := fmt.Sprint(selected); got != want {
		t.Fatalf("selected = %s, want %s", got, want)
	}
}

func TestUnownedDirtyPathSelectsItsPackageAndImporters_V1_0340(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                      "module example.test/m\n",
		"web/web.go":                  "package web\n\nimport \"embed\"\n\n//go:embed static\nvar files embed.FS\n",
		"web/web_test.go":             "package web\n",
		"web/static/css/site.css":     "body{}\n",
		"server/server.go":            "package server\n\nimport _ \"example.test/m/web\"\n",
		"server/server_test.go":       "package server\n",
		"lib/lib.go":                  "package lib\n",
		"lib/lib_test.go":             "package lib\n",
		"lib/testdata/deep/case.json": "{}\n",
		"app/app.go":                  "package app\n\nimport _ \"example.test/m/lib\"\n",
		"app/app_test.go":             "package app\n",
		"caller/caller.go":            "package caller\n\nimport _ \"example.test/m/gone\"\n",
		"caller/caller_test.go":       "package caller\n",
		"other/other.go":              "package other\n",
		"other/other_test.go":         "package other\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty, want string
	}{
		// An embedded asset reaches the embedding package and its importers.
		{"web/static/css/site.css", "[server/server_test.go web/web_test.go]"},
		// A fixture below a non-embedding package selects only that package.
		{"lib/testdata/deep/case.json", "[lib/lib_test.go]"},
		// A file directly in a package directory reaches its importers.
		{"lib/README.md", "[app/app_test.go lib/lib_test.go]"},
		// A deleted whole package reaches the importers that still name it.
		{"gone/gone.go", "[caller/caller_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: selected tests = %s, want %s", tc.dirty, got, tc.want)
		}
		for _, exclusion := range plan.Excluded {
			if exclusion.UnitID == "go:example.test/m/other" && exclusion.Reason != affected.ExcludedNoDependencyPath {
				t.Errorf("%s: other exclusion = %+v", tc.dirty, exclusion)
			}
		}
	}
}

// V1-0230: gate rule (d). A package that locates the repository root or
// reads a path its literals do not bound is selected on any non-empty dirty
// set; the dependents of a non-test locator are selected with it, while a
// locator only in a test file reaches no importer.
func TestUnboundedReaderIsSelectedOnAnyChange_V1_0230(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n",
		"caller/caller.go":          "package caller\n\nimport \"os\"\n\nfunc F() string { dir, _ := os.Getwd(); return dir }\n",
		"caller/caller_test.go":     "package caller\n",
		"user/user.go":              "package user\n\nimport _ \"example.test/m/caller\"\n",
		"user/user_test.go":         "package user\n",
		"aliased/aliased.go":        "package aliased\n\nimport system \"os\"\n\nfunc F() { system.Getwd() }\n",
		"aliased/aliased_test.go":   "package aliased\n\nimport \"os\"\n\nvar read = os.ReadFile\n",
		"dotted/dotted.go":          "package dotted\n\nimport . \"os\"\n\nfunc F() { Getwd() }\n",
		"dotted/dotted_test.go":     "package dotted\n",
		"climb/climb.go":            "package climb\n\nvar data = \"../shared/data.json\"\n",
		"climb/climb_test.go":       "package climb\n",
		"toplevel/toplevel.go":      "package toplevel\n",
		"toplevel/toplevel_test.go": "package toplevel\n\nvar arguments = []string{\"rev-parse\", \"--show-" + "toplevel\"}\n",
		"testuser/testuser.go":      "package testuser\n\nimport _ \"example.test/m/toplevel\"\n",
		"testuser/testuser_test.go": "package testuser\n",
		"plain/plain.go":            "package plain\n",
		"plain/plain_test.go":       "package plain\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/notes.md"})
	const want = "[aliased/aliased_test.go caller/caller_test.go climb/climb_test.go dotted/dotted_test.go toplevel/toplevel_test.go user/user_test.go]"
	if got := fmt.Sprint(plan.SelectedTests()); got != want {
		t.Fatalf("selected tests = %s, want %s", got, want)
	}
	for _, selection := range plan.Selected {
		if selection.Witness.Kind != affected.WitnessUnboundedReader {
			t.Errorf("%s witness = %+v, want %s", selection.UnitID, selection.Witness, affected.WitnessUnboundedReader)
		}
	}
	if empty := affected.Select(graph, nil); len(empty.Selected) != 0 {
		t.Errorf("empty dirty set selected %+v", empty.Selected)
	}
}

// AFP-V0-505, AFP-V0-506 (V1-2553): a `/` directly after `<` closes a markup
// tag and anchors nothing, so `</script>` is the lone name `script` and never
// reaches `script/` from the root; a backslash after `..` separates a climbing
// path, so `..\x` is `../x` and not the bare `..` that reaches the root. A
// `..` inside a comment is no literal. The dropped units cannot observe the
// change: a tag name is no file name, and `../fixtures` resolves inside the
// repository, below the package's parent.
func TestLexiconClosingTagAndParentBackslash_AFPV0505(t *testing.T) {
	t.Parallel()
	body := "package p\n\n// a \"..\" in a comment is not a literal\nvar _ = []string{\"<script>x</script>\", \"a</b/c>\", \"..\\\\x\", \"x\\\\..\\\\y\"}\n"
	tokens, err := golang.FilePathTokens("p.go", []byte(body), "example.test/m")
	if want := "[script x script a ./b/c/ ../x x ../y]"; err != nil || fmt.Sprint(tokens) != want {
		t.Errorf("tokens = %v (%v), want %s", tokens, err, want)
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"tag/tag.go":            "package tag\n",
		"tag/tag_test.go":       "package tag\n\nvar page = \"<script>x</script>\"\n",
		"slash/slash.go":        "package slash\n",
		"slash/slash_test.go":   "package slash\n\nvar windows = \"..\\\\fixtures\\\\x\"\n",
		"parent/parent.go":      "package parent\n",
		"parent/parent_test.go": "package parent\n\nvar up = \"..\"\n",
		"script/script.go":      "package script\n",
		"script/script_test.go": "package script\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprint(graph.UnboundedReaders()), "[go:example.test/m/parent]"; got != want {
		t.Errorf("unbounded readers = %s, want %s", got, want)
	}
	plan := affected.Select(graph, []string{"script/heavy.sh"})
	if got, want := fmt.Sprint(plan.SelectedTests()), "[parent/parent_test.go script/script_test.go]"; got != want {
		t.Errorf("script/heavy.sh selected %s, want %s", got, want)
	}
	plan = affected.Select(graph, []string{"fixtures/x"})
	if got, want := fmt.Sprint(plan.SelectedTests()), "[parent/parent_test.go slash/slash_test.go]"; got != want {
		t.Errorf("fixtures/x selected %s, want %s", got, want)
	}
}

// AFP-V0-840 (V1-3207): a path literal selects a reader only when it names
// the changed path or a directory holding it. A bare or prefixed file name
// (`source.sh`, `foo-source.sh`) is not `core-n1-source.sh`, and the
// HTML-escaped closing tag `&lt;/script&gt;` is the lone name `script`, not
// the directory `script/`; the full path and a `../script/` directory prefix
// still select.
func TestPathLiteralBoundary_AFPV0840(t *testing.T) {
	t.Parallel()
	tokens, err := golang.FilePathTokens("p.go", []byte("package p\n\nvar _ = []string{\"&lt;/script&gt;\", \"&LT;/b&gt;\"}\n"), "example.test/m")
	if want := "[lt script gt LT b gt]"; err != nil || fmt.Sprint(tokens) != want {
		t.Errorf("tokens = %v (%v), want %s", tokens, err, want)
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                   "module example.test/m\n",
		"script/core-n1-source.sh": "#!/bin/sh\n",
		"bare/bare.go":             "package bare\n",
		"bare/bare_test.go":        "package bare\n\nvar name = \"source.sh\"\n",
		"prefix/prefix.go":         "package prefix\n",
		"prefix/prefix_test.go":    "package prefix\n\nvar name = \"foo-source.sh\"\n",
		"escaped/escaped.go":       "package escaped\n",
		"escaped/escaped_test.go":  "package escaped\n\nvar page = \"&lt;script&gt;x&lt;/script&gt;\"\n",
		"full/full.go":             "package full\n",
		"full/full_test.go":        "package full\n\nvar path = \"../script/core-n1-source.sh\"\n",
		"dir/dir.go":               "package dir\n",
		"dir/dir_test.go":          "package dir\n\nvar scripts = \"../script/\"\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"script/core-n1-source.sh"})
	if got, want := fmt.Sprint(plan.SelectedTests()), "[dir/dir_test.go full/full_test.go]"; got != want {
		t.Errorf("script/core-n1-source.sh selected %s, want %s", got, want)
	}
	for _, selection := range plan.Selected {
		if selection.Witness.Kind != affected.WitnessPathLiteralReader {
			t.Errorf("%s witness = %+v, want %s", selection.UnitID, selection.Witness, affected.WitnessPathLiteralReader)
		}
	}
}

// AFP-V0-841 (V1-3201): a change to a build-constrained test file, here only
// its `//go:build` line, selects the owning package's tests as any test-file
// change does: no package imports a test file, and the constraint changes
// only which platforms compile it. The platforms the plan did not build stay
// the retained go:build-constraint-variants unknown, never an unbounded
// fan-out to other packages.
func TestBuildConstrainedTestFileSelectsOwner_AFPV0841(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                       "module example.test/m\n",
		"fifo/fifo.go":                 "package fifo\n\nfunc Make() {}\n",
		"fifo/fifo_test.go":            "package fifo\n",
		"fifo/draft_fifo_unix_test.go": "//go:build darwin || freebsd || linux || netbsd || openbsd\n\npackage fifo\n",
		"user/user.go":                 "package user\n\nimport \"example.test/m/fifo\"\n\nfunc Use() { fifo.Make() }\n",
		"user/user_test.go":            "package user\n",
		"other/other.go":               "package other\n",
		"other/other_test.go":          "package other\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"fifo/draft_fifo_unix_test.go"})
	if got, want := fmt.Sprint(plan.SelectedTests()), "[fifo/draft_fifo_unix_test.go fifo/fifo_test.go]"; got != want {
		t.Errorf("build-constrained test change selected %s, want %s", got, want)
	}
	if len(plan.Selected) != 1 || plan.Selected[0].Witness.Kind != affected.WitnessDirectTest {
		t.Errorf("selected %+v, want only the owning package by DIRECT_TEST_CHANGE", plan.Selected)
	}
	retained := false
	for _, unknown := range plan.Unknown {
		retained = retained || unknown.Detail == golang.FrontierBuildConstraint
	}
	if !retained || plan.Scope != affected.ScopeUnknown {
		t.Errorf("unknown = %+v, scope %s; want the %s platform set retained", plan.Unknown, plan.Scope, golang.FrontierBuildConstraint)
	}
}

// AFP-V0-810 (V1-2758): frame metadata is not a file read, even in a
// logging package importing net/http and logrus as navidrome does.
func TestFrameMetadataDoesNotReadFiles_AFPV0810(t *testing.T) {
	for _, call := range []struct{ imports, body string }{
		{`"runtime"`, `_, file, _, _ := runtime.Caller(0); _ = file`},
		{`rt "runtime"`, `_, file, _, _ := rt.Caller(0); _ = file`},
		{`. "runtime"`, `_, file, _, _ := Caller(0); _ = file`},
		{`"runtime"`, `pcs := make([]uintptr, 4); runtime.Callers(0, pcs); _ = runtime.CallersFrames(pcs); _ = runtime.FuncForPC(pcs[0])`},
	} {
		t.Run(call.body, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":                               "module example.test/m\n",
				"log/log.go":                           "package log\nimport (" + call.imports + "; _ \"net/http\"; _ \"github.com/sirupsen/logrus\")\nfunc Source() { " + call.body + " }\n",
				"log/log_test.go":                      "package log\n",
				"persistence/album_repository.go":      "package persistence\nimport _ \"example.test/m/log\"\n",
				"persistence/album_repository_test.go": "package persistence\n",
				"server/server.go":                     "package server\nimport _ \"example.test/m/persistence\"\n",
				"server/server_test.go":                "package server\n",
				"scanner/scanner.go":                   "package scanner\nimport _ \"example.test/m/log\"\n",
				"scanner/scanner_test.go":              "package scanner\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			if got := graph.UnboundedReaders(); len(got) != 0 {
				t.Fatalf("metadata readers: %v", got)
			}
			plan := affected.Select(graph, []string{"persistence/album_repository.go"})
			if got, want := fmt.Sprint(plan.SelectedTests()), "[persistence/album_repository_test.go server/server_test.go]"; got != want {
				t.Fatalf("selected %s, want %s", got, want)
			}
		})
	}
}

// V1-0230: the CEM sidecar narrowing of gate rule (c). Only a reader whose
// token resolves to the sidecar, from its own directory or from the root when
// an anchored or parent-only token can put it there, is selected; any other
// path keeps the component-run readers, including the one the sidecar drops.
func TestChangeEvidenceReadersAreNarrowed_V1_0230(t *testing.T) {
	if affected.ChangeEvidencePath != frontier.ExcludedPath {
		t.Fatalf("ChangeEvidencePath = %q, want %q", affected.ChangeEvidencePath, frontier.ExcludedPath)
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n",
		"fixture/fixture_test.go":   "package fixture\n\nvar a, b, c = \".corvint\", \"change.cem.json\", \".corvint/change.cem.json\"\n",
		"exact/exact_test.go":       "package exact\n\nvar sidecar = \"../.corvint/change.cem.json\"\n",
		"anchored/anchored.go":      "package anchored\n\nfunc Sidecar(root string) string { return root + \"/.corvint/change.cem.json\" }\n",
		"anchored/anchored_test.go": "package anchored\n",
		"deep/dir/dir_test.go":      "package dir\n\nvar evidence = \"../../.corvint\"\n",
		"partial/partial.go":        "package partial\n\nimport \"path/filepath\"\n\nvar evidence = filepath.Join(\"..\", \".corvint/change.cem\") + \".json\"\n",
		"partial/partial_test.go":   "package partial\n",
		"split/split.go":            "package split\n\nfunc Sidecar(root string) string { return root + \"/.cor\" + \"vint/change.cem.json\" }\n",
		"split/split_test.go":       "package split\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		{".corvint/change.cem.json", "[anchored/anchored_test.go deep/dir/dir_test.go exact/exact_test.go partial/partial_test.go split/split_test.go]"},
		// V1-0290 keeps the lone `.corvint` of fixture from naming a directory.
		{".corvint/other.json", "[deep/dir/dir_test.go]"},
		// A path of the same shape that is not the sidecar is not narrowed.
		{"docs/.corvint/change.cem.json", "[anchored/anchored_test.go deep/dir/dir_test.go exact/exact_test.go fixture/fixture_test.go partial/partial_test.go split/split_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: selected tests = %s, want %s", tc.dirty, got, tc.want)
		}
	}
}

// AFP-V0-152 (V1-1820): a sealed CEM directly under .corvint/changes/ narrows
// its Go readers as the sidecar does. A fixture-root token naming the same
// components is dropped; a reader whose token resolves to the sealed CEM or an
// ancestor, from its own directory or from the root, is still selected; any
// other path under .corvint/changes/ keeps the component-run readers.
func TestSealedChangeEvidenceReadersAreNarrowed_AFPV0152(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                    "module example.test/m\n",
		"fixture/fixture_test.go":   "package fixture\n\nvar a, b = \".corvint/changes\", \".corvint/changes/\"\n",
		"exact/exact_test.go":       "package exact\n\nvar sealed = \"../.corvint/changes\"\n",
		"anchored/anchored.go":      "package anchored\n\nfunc Sealed(root string) string { return root + \"/.corvint/changes/\" }\n",
		"anchored/anchored_test.go": "package anchored\n",
		"deep/dir/dir_test.go":      "package dir\n\nvar evidence = \"../../.corvint\"\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		{".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json", "[anchored/anchored_test.go deep/dir/dir_test.go exact/exact_test.go]"},
		// Not a sealed CEM: a nested path and another file name keep rule (c).
		{".corvint/changes/x/y.cem.json", "[anchored/anchored_test.go deep/dir/dir_test.go exact/exact_test.go fixture/fixture_test.go]"},
		{".corvint/changes/README.md", "[anchored/anchored_test.go deep/dir/dir_test.go exact/exact_test.go fixture/fixture_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: selected tests = %s, want %s", tc.dirty, got, tc.want)
		}
	}
}

// AFP-V0-072 (V1-1806): a Go unit that names a dirty path only by an
// unanchored lone file name, such as a fixture's README.md under t.TempDir(),
// is a reader only when the path is in its own subtree or every directory
// component of the path is a whole component of one of its tokens. A token
// that places a directory (a multi-component run, an anchored token) keeps the
// component-run relation.
func TestFileNameOnlyReadersMustPlaceThePath_AFPV0072(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                      "module example.test/m\n",
		"tempfixture/fixture_test.go": "package tempfixture\n\nimport (\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestFixture(t *testing.T) { _ = filepath.Join(t.TempDir(), \"README.md\") }\n",
		"joined/joined.go":            "package joined\n\nimport \"path/filepath\"\n\nfunc Index(root string) string { return filepath.Join(root, \"docs\", \"specs\", \"README.md\") }\n",
		"joined/joined_test.go":       "package joined\n",
		"multi/multi_test.go":         "package multi\n\nvar index = \"specs/README.md\"\n",
		"anchored/anchored.go":        "package anchored\n\nfunc Readme(root string) string { return root + \"/README.md\" }\n",
		"anchored/anchored_test.go":   "package anchored\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		// The PR #1134 shape: a spec index edit no longer selects the fixture.
		{"docs/specs/README.md", "[anchored/anchored_test.go joined/joined_test.go multi/multi_test.go]"},
		// The unit's own subtree, where its tests run, keeps the fixture.
		{"tempfixture/testdata/README.md", "[anchored/anchored_test.go tempfixture/fixture_test.go]"},
		// A root-level file has no directory to place.
		{"README.md", "[anchored/anchored_test.go joined/joined_test.go tempfixture/fixture_test.go]"},
		// Every directory component is in the vocabulary of joined only.
		{"docs/README.md", "[anchored/anchored_test.go joined/joined_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: selected tests = %s, want %s", tc.dirty, got, tc.want)
		}
	}
}

// AFP-V0-072: a climbing token reaches the unit's ancestors, a directory
// component the lexicon splits cannot be witnessed, and a narrowed unit's
// frontier still bears on the plan (Codex round 1 on #1138).
func TestFileNameOnlyReadersKeepWhatTheyCanReach_AFPV0072(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                  "module example.test/m\n",
		"pkg/model.go":            "package pkg\n",
		"pkg/model_test.go":       "package pkg\n",
		"pkg/check/read.go":       "package check\n\nimport \"path/filepath\"\n\nfunc Model() string { return filepath.Join(\"..\", \"model.go\") }\n",
		"pkg/check/read_test.go":  "package check\n",
		"guides/guides.go":        "package guides\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n)\n\nfunc Index() string { return filepath.Join(os.Getenv(\"REPO_ROOT\"), \"docs\", \"user guides\", \"README.md\") }\n",
		"guides/guides_test.go":   "package guides\n",
		"variant/linux.go":        "//go:build linux\n\npackage variant\n\nvar fixture = \"core.go\"\n",
		"variant/variant_test.go": "package variant\n",
		"core/core.go":            "package core\n",
		"core/core_test.go":       "package core\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		// ".." puts the parent directory's model.go in reach.
		{"pkg/model.go", "BOUNDED [pkg/check/read_test.go pkg/model_test.go]"},
		// No ancestor of pkg/check holds other/, and no token names it.
		{"other/model.go", "UNKNOWN []"},
		// "user guides" lexes as two tokens; the space is never witnessed.
		{"docs/user guides/README.md", "UNKNOWN [guides/guides_test.go]"},
		// A lexable component outside the vocabulary still narrows.
		{"docs/user-guides/README.md", "UNKNOWN []"},
		// variant is narrowed away, but its build-constraint frontier stays.
		{"core/core.go", "UNKNOWN [core/core_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.Scope, " ", plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: %s, want %s (unknown %v)", tc.dirty, got, tc.want, plan.Unknown)
		}
	}
	plan := affected.Select(graph, []string{"core/core.go"})
	if got := fmt.Sprint(plan.Unknown); got != "[{LANGUAGE_FRONTIER go:build-constraint-variants}]" {
		t.Errorf("core/core.go unknown = %s, want the narrowed variant's frontier", got)
	}
}

// V1-0289: every build variant's imports are edges, so a constrained file
// raises a frontier on its own package only, named by a plan that reaches it.
func TestBuildConstraintIsTheConstrainedPackagesFrontier_V1_0289(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                  "module example.test/m\n",
		"variant/linux.go":        "//go:build linux\n\npackage variant\n\nimport _ \"example.test/m/core\"\n",
		"variant/variant_test.go": "package variant\n",
		"core/core.go":            "package core\n",
		"core/core_test.go":       "package core\n",
		"plain/plain.go":          "package plain\n",
		"plain/plain_test.go":     "package plain\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	if frontier := graph.Frontier(); len(frontier) != 0 {
		t.Fatalf("graph frontier = %v, want none", frontier)
	}
	cases := []struct{ dirty, want string }{
		{"plain/plain.go", "BOUNDED [] [plain/plain_test.go]"},
		{"core/core.go", "UNKNOWN [{LANGUAGE_FRONTIER go:build-constraint-variants}] [core/core_test.go variant/variant_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.Scope, " ", plan.Unknown, " ", plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.dirty, got, tc.want)
		}
	}
}

// V1-1818: a literal that may be concatenated keeps its outer fragments of
// any length, so `"docs/a" + "lpha.go"` still names docs/alpha.go and its
// root package keeps its build-constraint frontier, while a short outer
// component the literal itself delimits (the `s/a` of a sed expression) is a
// whole component and names only a path that has it whole.
func TestShortFragmentsKeepConcatenatedReaders_V1_1818(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":        "module example.test/m\n",
		"linux.go":      "//go:build linux\n\npackage m\n",
		"root_test.go":  "package m\n\nimport \"os\"\n\nvar _, _ = os.ReadFile(\"docs/a\" + \"lpha.go\")\n",
		"docs/alpha.go": "package docs\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/alpha.go"})
	want := "UNKNOWN [{LANGUAGE_FRONTIER go:build-constraint-variants} {NO_SELECTABLE_TEST go:example.test/m/docs}] [root_test.go]"
	if got := fmt.Sprint(plan.Scope, " ", plan.Unknown, " ", plan.SelectedTests()); got != want {
		t.Errorf("docs/alpha.go: %s, want %s", got, want)
	}

	body := "package p\n\nvar _ = []string{\"docs/a\", \"sed 's/a b' x/yy;\", \"%sab/cd e\", \"ab/cd%s\", \"éab/cdé\", \"../a\"}\n"
	tokens, err := golang.FilePathTokens("p.go", []byte(body), "example.test/m")
	if want := "[docs/a sed ./s/a/ b ./x/yy/ ab/cd/ e ab/cd ab/cd ../a]"; err != nil || fmt.Sprint(tokens) != want {
		t.Errorf("tokens = %v (%v), want %s", tokens, err, want)
	}

	sed := t.TempDir()
	writeFiles(t, sed, map[string]string{
		"go.mod":              "module example.test/m\n",
		"tools/sed.go":        "package tools\n",
		"tools/sed_test.go":   "package tools\n\nconst script = \"sed 's/a b' file\"\n",
		"tools/tools_test.go": "package tools\n",
	})
	graph, err = affected.Build(context.Background(), sed, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		{"docs/specs/affected-plan-v0.md", "UNKNOWN []"},
		{"x/s/a/run.txt", "UNKNOWN [tools/sed_test.go tools/tools_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		if got := fmt.Sprint(plan.Scope, " ", plan.SelectedTests()); got != tc.want {
			t.Errorf("%s: %s, want %s (selected %v)", tc.dirty, got, tc.want, plan.Selected)
		}
	}
}

// AFP-V0-580: a GOOS or GOARCH file-name suffix makes a variant as a
// //go:build line does, and the build-constraint frontier bears on every plan
// that reaches the package. A variant file can call the package's other code,
// so a change reaching the package through a plain file's import or literal
// still changes what the variant runs (the file-local scoping was withdrawn).
func TestBuildConstraintFrontierBearsOnEveryReach_AFP_V0_580(t *testing.T) {
	files := map[string]string{
		"go.mod":                  "module example.test/m\n",
		"variant/variant.go":      "package variant\n\nimport _ \"example.test/m/mid\"\n\nvar plain = \"data/plain.txt\"\n\nfunc helper() string { return plain }\n",
		"variant/os_windows.go":   "package variant\n\nvar win = helper()\n",
		"variant/variant_test.go": "package variant\n",
		"mid/mid.go":              "package mid\n",
		"mid/mid_test.go":         "package mid\n",
		"other/other.go":          "package other\n",
		"other/other_test.go":     "package other\n",
	}
	root := t.TempDir()
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		want  bool
	}{
		{"variant/variant.go", true},
		{"mid/mid.go", true},
		{"data/plain.txt", true},
		{"other/other.go", false},
	} {
		plan := affected.Select(graph, []string{tc.dirty})
		got := slices.Contains(plan.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: golang.FrontierBuildConstraint})
		if got != tc.want {
			t.Errorf("%s: build-constraint frontier %v, want %v (selected %v)", tc.dirty, got, tc.want, plan.Selected)
		}
	}
}

// AFP-V0-580: a file name is a variant exactly when go/build's goodOSArchFile
// would constrain it.
func TestPlatformSuffixMatchesGoBuild_AFP_V0_580(t *testing.T) {
	for name, want := range map[string]bool{
		"a/os_windows.go": true, "a/os_linux_amd64.go": true, "a/x_arm64.go": true,
		"a/os_windows_test.go": true, "a/linux.go": false, "a/windows_test.go": false,
		"a/os_test.go": false, "a/plain.go": false, "a/os_unix.go": false,
	} {
		if got := golang.HasPlatformSuffix(name); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
}
