// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clitree"
)

const machineCatalogPath = "internal/capcatalog/catalog.json"

// CheckMachineCatalog closes the route from root machine help to every tree.
// Checking the trees alone cannot detect a dropped or stale commandTrees link.
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
	findings := machineLinkFindings(trees, catalog.CommandTrees)
	inv, err := Collect(filepath.Join(root, "cmd/corvint/help.go"), filepath.Join(root, "cmd/corvint/main.go"), filepath.Join(root, "cmd"), filepath.Join(root, "internal/mcp"))
	if err != nil {
		return append(findings, Finding{"malformed", sectionTools, machineCatalogPath, err.Error()})
	}
	return append(findings, machineToolFindings(inv.Tools, catalog.MCPTools)...)
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

// CAP-V0-8800: an offline tool cannot disappear behind a server's CLI-tree link.
func machineToolFindings(names []string, entries []capcatalog.MCPTool) []Finding {
	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[name] = true
	}
	seen := make(map[string]bool, len(entries))
	var findings []Finding
	for _, entry := range entries {
		if seen[entry.Name] {
			findings = append(findings, Finding{"duplicate", sectionTools, entry.Name, "duplicate machine catalog tool"})
		}
		seen[entry.Name] = true
		if !want[entry.Name] {
			findings = append(findings, Finding{"stale", sectionTools, entry.Name, "machine catalog tool names no implementation"})
		}
		if strings.TrimSpace(entry.Availability) == "" || strings.TrimSpace(entry.Summary) == "" {
			findings = append(findings, Finding{"malformed", sectionTools, entry.Name, "machine catalog tool needs availability and summary"})
		}
	}
	for _, name := range names {
		if !seen[name] {
			findings = append(findings, Finding{"missing", sectionTools, name, "MCP tool is unreachable from corvint help --json"})
		}
	}
	return findings
}
