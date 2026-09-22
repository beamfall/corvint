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
