package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// AFP-V0-581 (2) soundness: a shell script whose only frontier is
// shell:foreign-invocation-unmodelled reads a Go source file; a Go test runs the
// script, an edge the shell plugin cannot model. Editing the Go file changes
// what the script does, so the plan must either select the Go test or keep the
// script's frontier: a BOUNDED plan that excludes the test under-selects.
func TestOwnedPathReaderWithUnmodelledInvokersKeepsItsFrontier_AFP_V0_581(t *testing.T) {
	root := t.TempDir()
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/m\n\ngo 1.27.0\n")
	write("core/core.go", "package core\n\nfunc Value() int { return 1 }\n")
	write("core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n")
	write("script/read.sh", "#!/bin/sh\ncat core/core.go\n")
	write("runner/runner_test.go", "package runner\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestRun(t *testing.T) {\n\tif err := exec.Command(\"sh\", \"../script/read.sh\").Run(); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n")
	graph, err := affected.Build(context.Background(), root, golang.New(), shell.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"core/core.go"})
	named := slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool {
		return unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == shell.FrontierForeignInvocation
	})
	selected := slices.Contains(plan.SelectedTests(), "runner/runner_test.go")
	if plan.Scope == affected.ScopeBounded && !selected {
		t.Fatalf("BOUNDED plan excludes the script's invoker: selected %v unknown %v excluded %v", plan.SelectedTests(), plan.Unknown, plan.FrontierExcluded)
	}
	if !named && !selected {
		t.Fatalf("neither the frontier nor the invoker: scope %s selected %v unknown %v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

// V1-4392: an unreferenced leaf and documentation do not acquire a shell frontier.
func TestNewLeafPackageWithoutShellCallerIsBounded(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                            "module example.test/m\n\ngo 1.27.0\n",
		"internal/leaf/leaf.go":             "package leaf\nfunc Value() int {return 1}\n",
		"internal/leaf/leaf_test.go":        "package leaf\nimport \"testing\"\nfunc TestValue(t *testing.T) {if Value()!=1 {t.Fatal(Value())}}\n",
		"script/unrelated.sh":               "#!/bin/sh\necho hello\n",
		".corvint/documentation-paths.json": `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`,
		"docs/specs/leaf-v0.md":             "# Leaf\n\n## Requirements\n- leaf\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), shell.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"internal/leaf/leaf.go", "internal/leaf/leaf_test.go", "docs/specs/leaf-v0.md"})
	if plan.Scope != affected.ScopeBounded || !slices.Equal(plan.SelectedTests(), []string{"internal/leaf/leaf_test.go"}) {
		t.Fatalf("unrelated shell widens leaf: scope=%s tests=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}
