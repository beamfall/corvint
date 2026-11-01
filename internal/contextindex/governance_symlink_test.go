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

func governingLinkRefusals(packet map[string]any) []map[string]any {
	for _, entry := range mapsFromAny(packet["coverage"].(map[string]any)["unexamined"]) {
		if entry["relation"] == governingRelation {
			return mapsFromAny(entry["link_refusals"])
		}
	}
	return nil
}

func TestGoverningSymlinkAdmittedAsItsPinnedTarget_TCPV0270(t *testing.T) {
	t.Parallel()
	index := symlinkGovernanceIndex(t, symlinkGovernanceFiles, map[string]string{"AGENTS.md": "CLAUDE.md"}, nil)
	link, target := index.Sources["AGENTS.md"], index.Sources["CLAUDE.md"]
	if link.Mode != "120000" || target.Mode != "100644" {
		t.Fatalf("fixture modes link=%q target=%q", link.Mode, target.Mode)
	}
	packet, err := TaskContext(context.Background(), index, "review the project", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	rows := governingEvidence(t, packet)
	if len(rows) != 1 {
		t.Fatalf("governing rows=%v", rows)
	}
	row := rows[0]
	if row["path"] != "CLAUDE.md" || row["blob_hash"] != target.BlobHash || row["link_path"] != "AGENTS.md" || row["link_blob_hash"] != link.BlobHash {
		t.Fatalf("link admission did not bind both paths and pins: %v", row)
	}

	prompt := localPrompt(t, index, "", nil, 20, 8000)
	governance := mapsFromAny(prompt["governance"])
	if len(governance) != 1 || governance[0]["path"] != "CLAUDE.md" || governance[0]["blob_hash"] != target.BlobHash ||
		governance[0]["link_path"] != "AGENTS.md" || governance[0]["link_blob_hash"] != link.BlobHash {
		t.Fatalf("hook governance=%v", governance)
	}
	if selectors := prompt["coverage"].(map[string]any)["unavailable_selectors"].([]any); len(selectors) != 0 {
		t.Fatalf("admitted link still reported unavailable: %v", selectors)
	}
}

func TestGoverningSymlinkScreenedWhenTargetOrLinkModified_TCPV0271(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		modified map[string]string
		dirty    string
	}{
		{"target modified", map[string]string{"CLAUDE.md": "# Instructions\n\nEdited locally.\n"}, ""},
		{"link modified", nil, "AGENTS.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			index := symlinkGovernanceIndex(t, symlinkGovernanceFiles, map[string]string{"AGENTS.md": "CLAUDE.md"}, test.modified)
			if test.dirty != "" {
				index.DirtyPaths = append(index.DirtyPaths, test.dirty)
			}
			packet, err := TaskContext(context.Background(), index, "review the project", "", 10)
			if err != nil {
				t.Fatal(err)
			}
			rows := governingEvidence(t, packet)
			if len(rows) != 1 || rows[0]["path"] != "CLAUDE.md" || rows[0]["authority"] != DowngradedAuthority {
				t.Fatalf("modified link admission was not screened: %v", rows)
			}
			assertSelfModifiedWarning(t, rows[0])
			prompt := localPrompt(t, index, "", nil, 20, 8000)
			if governance := mapsFromAny(prompt["governance"]); len(governance) != 0 {
				t.Fatalf("hook kept screened governance: %v", governance)
			}
			selectors := mapsFromAny(prompt["coverage"].(map[string]any)["unavailable_selectors"])
			if len(selectors) != 1 || selectors[0]["path"] != "AGENTS.md" || selectors[0]["reason"] != SelfModifiedAuthorityWarning {
				t.Fatalf("hook selectors=%v", selectors)
			}
		})
	}
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

func TestGoverningSymlinkRefusalsStayUnavailableWithTypedReasons_TCPV0272(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		links     map[string]string
		untracked map[string]string
		reason    string
	}{
		{"out of tree", map[string]string{"AGENTS.md": "../outside.md"}, nil, linkTargetOutside},
		{"absolute", map[string]string{"AGENTS.md": "/etc/hosts"}, nil, linkTargetOutside},
		{"untracked", map[string]string{"AGENTS.md": "NOTES.md"}, map[string]string{"NOTES.md": "# Notes\n"}, linkTargetNotInTree},
		{"dangling", map[string]string{"AGENTS.md": "missing.md"}, nil, linkTargetNotInTree},
		{"directory", map[string]string{"AGENTS.md": "docs"}, nil, linkTargetNotRegular},
		{"chained", map[string]string{"AGENTS.md": "GUIDE.md", "GUIDE.md": "CLAUDE.md"}, nil, linkChain},
		{"through a symlinked directory", map[string]string{"AGENTS.md": "manual/guide.md", "manual": "docs"}, nil, linkChain},
		{"self cycle", map[string]string{"AGENTS.md": "AGENTS.md"}, nil, linkCycle},
		{"two-link cycle", map[string]string{"AGENTS.md": "LOOP.md", "LOOP.md": "AGENTS.md"}, nil, linkCycle},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			index := symlinkGovernanceIndex(t, symlinkGovernanceFiles, test.links, test.untracked)
			if index.Sources["AGENTS.md"].Mode != "120000" {
				t.Fatalf("fixture AGENTS.md mode=%q", index.Sources["AGENTS.md"].Mode)
			}
			packet, err := TaskContext(context.Background(), index, "review the project", "", 10)
			if err != nil {
				t.Fatal(err)
			}
			// TCP-V0-275: the readable lower-precedence CLAUDE.md is never substituted.
			if rows := governingEvidence(t, packet); len(rows) != 0 {
				t.Fatalf("refused link produced governing rows: %v", rows)
			}
			refusals := governingLinkRefusals(packet)
			if len(refusals) != 1 || refusals[0]["path"] != "AGENTS.md" || refusals[0]["reason"] != test.reason {
				t.Fatalf("link refusals=%v, want %s", refusals, test.reason)
			}

			prompt := localPrompt(t, index, "", nil, 20, 8000)
			if governance := mapsFromAny(prompt["governance"]); len(governance) != 0 {
				t.Fatalf("hook substituted governance: %v", governance)
			}
			selectors := mapsFromAny(prompt["coverage"].(map[string]any)["unavailable_selectors"])
			if len(selectors) != 1 || selectors[0]["section"] != "governance" || selectors[0]["path"] != "AGENTS.md" || selectors[0]["reason"] != test.reason {
				t.Fatalf("hook selectors=%v, want %s", selectors, test.reason)
			}
		})
	}
}
