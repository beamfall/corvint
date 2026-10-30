package contextindex

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestImpactUntrackedPathRefusalNamesSnapshotAndNextStep: V1-2080. A path the
// snapshot does not track is refused with the stable `impact paths are not
// tracked at revision ` prefix the MCP bridge keys on, and the refusal names
// the path, labels the revision as a tree -- of the commit for a clean
// worktree, of the working-tree snapshot otherwise (a new unstaged file makes
// the worktree dirty) -- with its commit, and states the stage-or-commit next
// step.
func TestImpactUntrackedPathRefusalNamesSnapshotAndNextStep(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{"README.md": "# Fixture\n\nFixture.\n"})
	head := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	for _, test := range []struct {
		name, path, label string
	}{
		{"clean", "MISSING.md", "tree of commit " + head},
		{"new-unstaged-file", "AGENTS.md", "working-tree snapshot tree over commit " + head},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.path == "AGENTS.md" {
				writeTestFile(t, root, "AGENTS.md", "# Agents\n\nNew, unstaged.\n")
			}
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Impact(index, []string{test.path}, maxLimit)
			var refusal *Error
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a refusal", err)
			}
			message := refusal.Error()
			for _, want := range []string{
				"impact paths are not tracked at revision " + index.Revision + " ",
				"(" + test.label + ")",
				": " + test.path + ";",
				"stage the path with `git add` or commit it",
			} {
				if !strings.Contains(message, want) {
					t.Fatalf("refusal %q lacks %q", message, want)
				}
			}
		})
	}
}
