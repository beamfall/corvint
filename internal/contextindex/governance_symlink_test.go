// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// symlinkGovernanceIndex commits files plus links (path -> raw target) and
// indexes the tree. untracked files are written but never added.
func symlinkGovernanceIndex(t *testing.T, files, links, untracked map[string]string) *Index {
	t.Helper()
	root := t.TempDir()
	if command := testGitOptional(t, root, "init", "-q"); command != "" {
		t.Skipf("git init unsupported: %s", command)
	}
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	testGit(t, root, "config", "core.symlinks", "true")
	for relative, content := range files {
		writeTestFile(t, root, relative, content)
	}
	for relative, target := range links {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, relative)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "governance symlink fixture")
	for relative, content := range untracked {
		writeTestFile(t, root, relative, content)
	}
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

var symlinkGovernanceFiles = map[string]string{
	"go.mod":        "module example.test/links\n\ngo 1.27.0\n",
	"CLAUDE.md":     "# Instructions\n\n## Verify\n\nRun the focused tests.\n",
	"docs/guide.md": "# Guide\n",
	"pkg/link.go":   "package link\n\nfunc Link() {}\n",
}
