package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

func writeOwnershipFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const nonSourceEvidence = `{"profile":"corvint-non-source-paths/0","entries":[{"path":"evidence/","reason":"retained run output"}]}`

// TestOwnershipAuditListsUnclaimedPaths_AZU_V0_002 checks that the audit lists
// exactly the paths a plan would report as UNOWNED_DIRTY_PATH: a claimed
// source file, a documentation path, a non-source path and a make file are
// left out; an undeclared data file is listed.
func TestOwnershipAuditListsUnclaimedPaths_AZU_V0_002(t *testing.T) {
	root := t.TempDir()
	writeOwnershipFixture(t, root, map[string]string{
		"go.mod":                        "module example.test/m\n\ngo 1.27.0\n",
		"core/core.go":                  "package core\n",
		"Makefile":                      "test:\n\tgo test ./...\n",
		"README.md":                     "readme\n",
		"evidence/run.json":             "{}\n",
		"data/table.csv":                "1,2\n",
		affected.DocumentationPathsPath: `{"profile":"corvint-documentation-paths/0","paths":["README.md"]}`,
		affected.NonSourcePathsPath:     nonSourceEvidence,
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	tracked := []string{"Makefile", "README.md", "core/core.go", "data/table.csv", "evidence/run.json", "go.mod"}
	// go.mod has no claimant: the Go structure rule selects its importers but
	// a plan still reports it as UNOWNED_DIRTY_PATH, so the audit lists it.
	if got := graph.OwnershipAudit(tracked); !slices.Equal(got, []string{"data/table.csv", "go.mod"}) {
		t.Fatalf("OwnershipAudit = %q, want data/table.csv and go.mod", got)
	}
	plan := affected.Select(graph, []string{"evidence/run.json"})
	for _, entry := range plan.Unknown {
		if entry.Reason == affected.UnknownUnownedDirtyPath {
			t.Fatalf("declared non-source path stayed unowned: %+v", plan.Unknown)
		}
	}
	if !slices.ContainsFunc(plan.NonSource, func(group affected.NonSource) bool {
		return group.Class == affected.ClassDeclaredNonSource && slices.Equal(group.Paths, []string{"evidence/run.json"}) && strings.HasSuffix(group.Reason, ": retained run output")
	}) {
		t.Fatalf("NonSource = %+v, want one %s group for evidence/run.json", plan.NonSource, affected.ClassDeclaredNonSource)
	}
}

// TestNonSourcePathsRefusesInvalidDeclarations_AZU_V0_002 checks that the
// declaration takes the closed runtime-readers grammar: a driver input, a
// Corvint declaration, `.github/`, an unordered list or an empty reason
// refuses the graph build.
func TestNonSourcePathsRefusesInvalidDeclarations_AZU_V0_002(t *testing.T) {
	for name, declaration := range map[string]string{
		"make file":      `{"profile":"corvint-non-source-paths/0","entries":[{"path":"Makefile","reason":"r"}]}`,
		"declaration":    `{"profile":"corvint-non-source-paths/0","entries":[{"path":".corvint/non-source-paths.json","reason":"r"}]}`,
		"corvint dir":    `{"profile":"corvint-non-source-paths/0","entries":[{"path":".corvint/","reason":"r"}]}`,
		"github":         `{"profile":"corvint-non-source-paths/0","entries":[{"path":".github/forward-rows/","reason":"r"}]}`,
		"unordered":      `{"profile":"corvint-non-source-paths/0","entries":[{"path":"z/","reason":"r"},{"path":"a/","reason":"r"}]}`,
		"empty reason":   `{"profile":"corvint-non-source-paths/0","entries":[{"path":"a/","reason":""}]}`,
		"no entries":     `{"profile":"corvint-non-source-paths/0","entries":[]}`,
		"unknown member": `{"profile":"corvint-non-source-paths/0","entries":[{"path":"a/","reason":"r","x":1}]}`,
		"wrong profile":  `{"profile":"corvint-runtime-readers/0","entries":[{"path":"a/","reason":"r"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeOwnershipFixture(t, root, map[string]string{"go.mod": "module example.test/m\n", affected.NonSourcePathsPath: declaration})
			if _, err := affected.Build(context.Background(), root, golang.New()); err == nil || !strings.Contains(err.Error(), affected.NonSourcePathsPath) {
				t.Fatalf("Build error = %v, want a %s refusal", err, affected.NonSourcePathsPath)
			}
		})
	}
}
