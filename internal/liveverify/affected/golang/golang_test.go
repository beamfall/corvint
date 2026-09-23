package golang_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

func TestDeletedGoSourceNamesDeletionInOwnUnitExclusion(t *testing.T) {
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
	reasons := make(map[string]string, len(plan.Excluded))
	for _, exclusion := range plan.Excluded {
		reasons[exclusion.UnitID] = exclusion.Reason
	}
	if got := reasons["go:example.test/directorynames/core"]; got != affected.ExcludedDirtyGoPathMayBeDeletedOrRenamed {
		t.Fatalf("core exclusion reason=%q", got)
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
		"other/other.go":    "package other\n",
		"user/user.go":      "package user\n\nimport _ \"example.test/m/pkg\"\n",
		"user/user_test.go": "package user\n",
	})
	result, err := golang.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		if unit.ID == "go:example.test/m/pkg" && fmt.Sprint(unit.PathTokens) != "[../docs/guide.md .json /data/]" {
			t.Fatalf("pkg path tokens=%q", unit.PathTokens)
		}
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
		t.Fatalf("a root-anchored module literal must name its path: %v", reader.Selected)
	}
	unnamed := affected.Select(graph, []string{"other/notes.md"})
	if len(unnamed.Selected) != 0 || fmt.Sprint(unnamed.Unknown) != "[{UNOWNED_DIRTY_PATH other/notes.md}]" {
		t.Fatalf("unnamed path selected=%v unknown=%v", unnamed.Selected, unnamed.Unknown)
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
