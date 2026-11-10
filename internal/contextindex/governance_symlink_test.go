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

func governingEvidence(t *testing.T, packet map[string]any) []map[string]any {
	t.Helper()
	found := []map[string]any{}
	for _, result := range mapsFromAny(packet["results"]) {
		if result["kind"] == governingRelation {
			found = append(found, mapsFromAny(result["evidence"])[0])
		}
	}
	return found
}

func TestGoverningSymlinkTargetReachedTwiceReservesOneRow_TCPV0273(t *testing.T) {
	t.Parallel()
	index := symlinkGovernanceIndex(t, symlinkGovernanceFiles, map[string]string{"pkg/AGENTS.md": "../CLAUDE.md"}, nil)
	packet, err := TaskContext(context.Background(), index, "fix Link", "pkg/link.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	rows := governingEvidence(t, packet)
	if len(rows) != 1 || rows[0]["path"] != "CLAUDE.md" || rows[0]["link_path"] != "pkg/AGENTS.md" {
		t.Fatalf("governing rows=%v", rows)
	}
	for _, entry := range mapsFromAny(packet["coverage"].(map[string]any)["unexamined"]) {
		if entry["relation"] == governingRelation && entry["withheld"] != 0 {
			t.Fatalf("an admitted link counted as withheld: %v", entry)
		}
	}
}

// assertSelfModifiedWarning requires TCP-V0-057's warning on a downgraded
// row's evidence, not only its downgraded authority label.
func assertSelfModifiedWarning(t *testing.T, row map[string]any) {
	t.Helper()
	for _, warning := range mapsFromAny(row["warnings"]) {
		if warning["code"] == SelfModifiedAuthorityWarning && warning["scope"] == "working-tree" {
			return
		}
	}
	t.Fatalf("downgraded row lacks the self-modified-authority warning: %v", row)
}

func TestGoverningSymlinkLaterLinkModifiedStillScreens_TCPV0271(t *testing.T) {
	t.Parallel()
	links := map[string]string{"pkg/AGENTS.md": "../CLAUDE.md", "AGENTS.md": "CLAUDE.md"}
	index := symlinkGovernanceIndex(t, symlinkGovernanceFiles, links, nil)
	index.DirtyPaths = append(index.DirtyPaths, "AGENTS.md")
	packet, err := TaskContext(context.Background(), index, "fix Link", "pkg/link.go", 10)
	if err != nil {
		t.Fatal(err)
	}
	rows := governingEvidence(t, packet)
	if len(rows) != 1 || rows[0]["path"] != "CLAUDE.md" || rows[0]["link_path"] != "pkg/AGENTS.md" ||
		rows[0]["authority"] != DowngradedAuthority {
		t.Fatalf("a folded link's modification left the row authoritative: %v", rows)
	}
	assertSelfModifiedWarning(t, rows[0])
}
