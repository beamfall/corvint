// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/clitree"
)

// CheckNodeCatalog gates every node at every depth, including Tasks verbs.
func CheckNodeCatalog(root string) []Finding {
	trees, err := readTrees(filepath.Join(root, treesDir))
	if err != nil {
		return []Finding{{"malformed", "cli-nodes", treesDir, err.Error()}}
	}
	data, err := os.ReadFile(filepath.Join(root, "docs/CAPABILITIES.md"))
	if err != nil {
		return []Finding{{"missing", "cli-nodes", "catalog", err.Error()}}
	}
	_, tail, ok := strings.Cut(string(data), "<!-- capability-gate:cli-nodes -->\n")
	content, _, closed := strings.Cut(tail, "<!-- /capability-gate:cli-nodes -->")
	if !ok || !closed || content != clitree.CatalogMarkdown(trees) {
		return []Finding{{"node-catalog-drift", "cli-nodes", "catalog", "full-depth command table differs from the command trees; run make capability-catalog"}}
	}
	return nil
}
