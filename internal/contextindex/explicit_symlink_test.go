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

// TCP-V0-7406: tracked symlinks sharing one target stay separate candidates,
// so a symbol or line selector reaching two aliases is ambiguous and every
// row keeps its own link identity, including beside the target named directly.
func TestTCPV07406SymlinkAliasesKeepEveryIdentity(t *testing.T) {
	source := "package src\n\nfunc Helper() {}\n\nfunc Target() {}\n"
	index := symlinkGovernanceIndex(t, map[string]string{"AGENTS.md": "# Rules\n", "go.mod": "module example.test/links\n\ngo 1.27.0\n", "src/impl.go": source, "z/target.go": source},
		map[string]string{"a/link.go": "../src/impl.go", "b/link.go": "../src/impl.go", "a/target.go": "../z/target.go"}, nil)
	check := func(task, target string, links ...string) {
		t.Helper()
		packet := localPrompt(t, index, task, nil, 10, 8000)
		if reason := packet["resolution"].(map[string]any)["reason"]; reason != "ambiguous-anchor" {
			t.Errorf("%s: reason=%v, want ambiguous-anchor: %v", task, reason, packet)
			return
		}
		rows := mapsFromAny(packet["task_evidence"])
		if len(rows) != len(links) {
			t.Errorf("%s: rows=%v, want one per alias %v", task, rows, links)
			return
		}
		for i, link := range links {
			row := rows[i]
			if row["path"] != target || row["line"] != 5 || row["blob_hash"] != index.Sources[target].BlobHash {
				t.Errorf("%s: row %d=%v", task, i, row)
			}
			if link == "" {
				if _, ok := row["link_path"]; ok {
					t.Errorf("%s: direct row carries a link: %v", task, row)
				}
				continue
			}
			if row["link_path"] != link || row["link_blob_hash"] != index.Sources[link].BlobHash {
				t.Errorf("%s: row %d=%v, want link %s", task, i, row, link)
			}
		}
	}
	check("Fix link.go#Target", "src/impl.go", "a/link.go", "b/link.go")
	check("Read link.go:5", "src/impl.go", "a/link.go", "b/link.go")
	// A link sorted before the target it names keeps both identities.
	check("Fix target.go#Target", "z/target.go", "a/target.go", "")
	check("Read target.go:5", "z/target.go", "a/target.go", "")
}

// TCP-V0-7406: a line or symbol selector on a refused tracked symlink
// (dangling, chained, out-of-tree or cyclic) records the link's typed refusal in
// unavailable_selectors before any selector matching, so the anchor is
// unavailable evidence, never not-found, and is never checked against the link
// blob.
func TestTCPV07406RefusedSymlinkSelectorsKeepTheirRefusal(t *testing.T) {
	target := "# Target\n\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	for _, tc := range []struct{ name, target, reason string }{
		{"dangling", "missing", linkTargetNotInTree}, {"chain", "second", linkChain},
		{"outside", "../../escape", linkTargetOutside}, {"cycle", "link", linkCycle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := symlinkGovernanceIndex(t,
				map[string]string{"AGENTS.md": "# Rules\n", "go.mod": "module example.test/links\n\ngo 1.27.0\n", "docs/target.md": target, "pkg/target.go": "package pkg\n\nfunc Helper() {}\n\nfunc Target() {}\n"},
				map[string]string{"docs/link.md": tc.target + ".md", "docs/second.md": "target.md", "pkg/link.go": tc.target + ".go", "pkg/second.go": "target.go"}, nil)
			for _, selector := range []struct{ task, link string }{
				{"Read link.md:10", "docs/link.md"}, {"Fix link.go#Target", "pkg/link.go"}, {"Read docs/link.md:1", "docs/link.md"},
			} {
				packet := localPrompt(t, index, selector.task, nil, 10, 8000)
				if rows := mapsFromAny(packet["task_evidence"]); len(rows) != 0 {
					t.Errorf("%s: refused link admitted rows: %v", selector.task, packet)
					continue
				}
				if reason := packet["resolution"].(map[string]any)["reason"]; reason != "anchor-evidence-unavailable" {
					t.Errorf("%s: reason=%v, want anchor-evidence-unavailable: %v", selector.task, reason, packet)
					continue
				}
				found := false
				for _, row := range mapsFromAny(packet["coverage"].(map[string]any)["unavailable_selectors"]) {
					found = found || row["section"] == "task_evidence" && row["path"] == selector.link && row["reason"] == tc.reason
				}
				if !found {
					t.Errorf("%s: wanted %s on %s: %v", selector.task, tc.reason, selector.link, packet)
				}
			}
		})
	}
}
