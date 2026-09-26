package golang_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

func TestPackagesUnderBuildOutputDirectoryNamesAreSelected(t *testing.T) {
	root := fixtureRoot(t, "directory-names")
	graph, err := affected.Build(root, golang.New())
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

	graph, err := affected.Build(root, golang.New())
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
	result, err := golang.New().Units(fixtureRoot(t, "no-go"))
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
	graph, err := affected.Build(root, golang.New())
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

func runGit(t *testing.T, gitExecutable, root string, arguments ...string) {
	t.Helper()
	command := exec.Command(gitExecutable, append([]string{"-C", root}, arguments...)...)
	command.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

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
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].ID != "go:example.test/m/pkg" {
		t.Fatalf("units=%+v", result.Units)
	}
	if len(result.Frontier) != 0 {
		t.Fatalf("frontier=%v", result.Frontier)
	}
	graph, err := affected.Build(root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	fixture := affected.Select(graph, []string{"pkg/testdata/fixture.go"})
	want := []affected.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "pkg/testdata/fixture.go"}}
	if fmt.Sprint(fixture.Unknown) != fmt.Sprint(want) || fixture.Scope != affected.ScopeUnknown {
		t.Fatalf("fixture edit scope=%s unknown=%v", fixture.Scope, fixture.Unknown)
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
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Units) != 1 || result.Units[0].ID != "go:example.test/ws" {
		t.Fatalf("units=%+v", result.Units)
	}
	graph, err := affected.Build(root, golang.New())
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
	result, err := golang.New().Units(root)
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
	graph, err := affected.Build(root, golang.New())
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
	graph, err := affected.Build(root, golang.New())
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
		result, err := golang.New().Units(root)
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
	graph, err := affected.Build(root, golang.New())
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
	graph, err := affected.Build(root, golang.New())
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
	graph, err := affected.Build(root, golang.New())
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
		"caller/caller.go":          "package caller\n\nimport \"runtime\"\n\nfunc F() { runtime.Caller(0) }\n",
		"caller/caller_test.go":     "package caller\n",
		"user/user.go":              "package user\n\nimport _ \"example.test/m/caller\"\n",
		"user/user_test.go":         "package user\n",
		"aliased/aliased.go":        "package aliased\n\nimport rt \"runtime\"\n\nfunc F() { rt.Caller(0) }\n",
		"aliased/aliased_test.go":   "package aliased\n",
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
	graph, err := affected.Build(root, golang.New())
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
	graph, err := affected.Build(root, golang.New())
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
	graph, err := affected.Build(root, golang.New())
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
