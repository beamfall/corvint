// SPDX-License-Identifier: Apache-2.0
package contextindex

import "testing"

// TCP-V0-7406: explicit paths use the governance one-hop admission and pins.
func TestTCPV07406ExplicitSymlink(t *testing.T) {
	for _, tc := range []struct{ name, target, reason string }{
		{"valid", "docs/target.md", ""}, {"outside", "../escape", linkTargetOutside},
		{"dangling", "missing.md", linkTargetNotInTree}, {"chain", "second.md", linkChain}, {"cycle", "link.md", linkCycle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := symlinkGovernanceIndex(t, map[string]string{"AGENTS.md": "# Rules\n", "docs/target.md": "# Target\n"}, map[string]string{"link.md": tc.target, "second.md": "docs/target.md"}, nil)
			packet := localPrompt(t, index, "Read link.md", nil, 10, 8000)
			if tc.reason == "" {
				rows := mapsFromAny(packet["task_evidence"])
				if len(rows) != 1 || rows[0]["path"] != "docs/target.md" || rows[0]["blob_hash"] != index.Sources["docs/target.md"].BlobHash || rows[0]["link_path"] != "link.md" || rows[0]["link_blob_hash"] != index.Sources["link.md"].BlobHash {
					t.Fatalf("%v", packet)
				}
				index.DirtyPaths = append(index.DirtyPaths, "link.md")
				dirty := localPrompt(t, index, "Read link.md", nil, 10, 8000)
				if dirty["resolution"].(map[string]any)["reason"] != "anchor-worktree-changed" {
					t.Fatalf("dirty link was resolved: %v", dirty)
				}
			} else {
				found := false
				// The coverage retains unavailable selectors including their typed reason.
				for _, row := range mapsFromAny(packet["coverage"].(map[string]any)["unavailable_selectors"]) {
					if row["reason"] == tc.reason {
						found = true
					}
				}
				if !found {
					t.Fatalf("wanted %s: %v", tc.reason, packet)
				}
			}
		})
	}
}

// TCP-V0-7406: line and symbol selectors on a tracked symlink are validated
// against its pinned one-hop target, and the row keeps both identities.
func TestTCPV07406SymlinkSelectorsUseTheTarget(t *testing.T) {
	target := "# Target\n\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	index := symlinkGovernanceIndex(t, map[string]string{"AGENTS.md": "# Rules\n", "docs/target.md": target, "go.mod": "module example.test/links\n\ngo 1.27.0\n", "pkg/target.go": "package pkg\n\nfunc Helper() {}\n\nfunc Target() {}\n"}, map[string]string{"link.md": "docs/target.md", "pkg/link.go": "target.go"}, nil)
	check := func(task, path string, line int) {
		t.Helper()
		packet := localPrompt(t, index, task, nil, 10, 8000)
		rows := mapsFromAny(packet["task_evidence"])
		link := map[string]string{"docs/target.md": "link.md", "pkg/target.go": "pkg/link.go"}[path]
		if len(rows) != 1 || rows[0]["path"] != path || rows[0]["line"] != line || rows[0]["link_path"] != link || rows[0]["link_blob_hash"] != index.Sources[link].BlobHash || rows[0]["blob_hash"] != index.Sources[path].BlobHash {
			t.Fatalf("%s: %v", task, packet)
		}
	}
	check("Read link.md:10", "docs/target.md", 10)
	check("Fix pkg/link.go#Target", "pkg/target.go", 5)
	// Control: a line past the target's end is still not found.
	packet := localPrompt(t, index, "Read link.md:12", nil, 10, 8000)
	if rows := mapsFromAny(packet["task_evidence"]); len(rows) != 0 {
		t.Fatalf("out-of-range target line admitted: %v", packet)
	}
}
