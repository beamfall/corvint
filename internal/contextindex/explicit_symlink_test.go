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
