// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clitree"
)

const machineCatalogPath = "internal/capcatalog/catalog.json"

// CheckMachineCatalog closes the route from root machine help to every tree.
// Checking the trees alone cannot detect a dropped or stale commandTrees link.
// The MCP tool half of this check left with the MCP servers (decision 0594).
func CheckMachineCatalog(root string) []Finding {
	trees, err := readTrees(filepath.Join(root, treesDir))
	if err != nil {
		return []Finding{{"malformed", sectionTrees, treesDir, err.Error()}}
	}
	data, err := os.ReadFile(filepath.Join(root, machineCatalogPath))
	if err != nil {
		return []Finding{{"missing", sectionTrees, machineCatalogPath, err.Error()}}
	}
	var catalog capcatalog.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return []Finding{{"malformed", sectionTrees, machineCatalogPath, err.Error()}}
	}
	if catalog.Schema != capcatalog.Schema {
		return []Finding{{"malformed", sectionTrees, machineCatalogPath, "unsupported machine catalog schema"}}
	}
	return machineLinkFindings(trees, catalog.CommandTrees)
}

func machineLinkFindings(trees []clitree.Tree, refs []capcatalog.CommandTreeRef) []Finding {
	want := make(map[string]capcatalog.CommandTreeRef, len(trees))
	for _, tree := range trees {
		want[tree.Binary] = capcatalog.CommandTreeRef{
			Binary: tree.Binary, Command: tree.HelpJSON, Snapshot: treesDir + "/" + tree.Binary + ".json",
		}
	}
	seen := make(map[string]bool, len(refs))
	var findings []Finding
	for _, ref := range refs {
		if seen[ref.Binary] {
			findings = append(findings, Finding{"duplicate", sectionTrees, ref.Binary, "duplicate root machine catalog link"})
		}
		seen[ref.Binary] = true
		expected, ok := want[ref.Binary]
		if !ok {
			findings = append(findings, Finding{"stale", sectionTrees, ref.Binary, "root machine catalog link names no tree"})
		} else if ref != expected {
			findings = append(findings, Finding{"tree-link-drift", sectionTrees, ref.Binary, "root machine catalog command or snapshot differs from its tree; run make capability-catalog"})
		}
	}
	for _, tree := range trees {
		if !seen[tree.Binary] {
			findings = append(findings, Finding{"tree-unlinked", sectionTrees, tree.Binary, "tree is unreachable from corvint help --json; run make capability-catalog"})
		}
	}
	return findings
}
