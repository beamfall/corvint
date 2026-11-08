// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/clitree"
)

// CAP-V0-8800: tree coverage alone is insufficient; every tree must be reachable
// through the exact printing command and snapshot in root machine help.
func TestCAPV08800MachineCatalogLinks(t *testing.T) {
	for _, finding := range CheckMachineCatalog("../..") {
		t.Error(finding)
	}
	for _, row := range []struct {
		name string
		edit func(*capcatalog.Catalog)
		kind string
	}{
		{"complete", func(*capcatalog.Catalog) {}, ""},
		{"missing tool", func(c *capcatalog.Catalog) { c.MCPTools = nil }, "missing"},
		{"duplicate tool", func(c *capcatalog.Catalog) { c.MCPTools = append(c.MCPTools, c.MCPTools[0]) }, "duplicate"},
		{"stale tool", func(c *capcatalog.Catalog) { c.MCPTools[0].Name = "corvint.absent" }, "stale"},
		{"tool without availability", func(c *capcatalog.Catalog) { c.MCPTools[0].Availability = " " }, "malformed"},
		{"tool without summary", func(c *capcatalog.Catalog) { c.MCPTools[0].Summary = "" }, "malformed"},
		{"missing node tree", func(c *capcatalog.Catalog) { c.CommandTrees = c.CommandTrees[1:] }, "tree-unlinked"},
		{"no links", func(c *capcatalog.Catalog) { c.CommandTrees = nil }, "tree-unlinked"},
		{"duplicate", func(c *capcatalog.Catalog) { c.CommandTrees = append(c.CommandTrees, c.CommandTrees[0]) }, "duplicate"},
		{"stale", func(c *capcatalog.Catalog) { c.CommandTrees[0].Binary = "corvint-absent" }, "stale"},
		{"wrong command", func(c *capcatalog.Catalog) { c.CommandTrees[0].Command = "corvint help" }, "tree-link-drift"},
		{"wrong snapshot", func(c *capcatalog.Catalog) { c.CommandTrees[0].Snapshot = "missing.json" }, "tree-link-drift"},
		{"unsupported schema", func(c *capcatalog.Catalog) { c.Schema = "unknown/1" }, "malformed"},
	} {
		t.Run(row.name, func(t *testing.T) {
			files := syntheticRepository()
			var catalog capcatalog.Catalog
			if err := json.Unmarshal([]byte(files[machineCatalogPath]), &catalog); err != nil {
				t.Fatal(err)
			}
			row.edit(&catalog)
			encoded, err := json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
			files[machineCatalogPath] = string(encoded)
			root := t.TempDir()
			writeFiles(t, root, files)
			found := CheckMachineCatalog(root)
			if row.kind == "" && len(found) != 0 || row.kind != "" && len(kinds(found)[row.kind]) == 0 {
				t.Fatalf("want %q, got %v", row.kind, found)
			}
		})
	}
	for _, body := range []string{"{", "null", `{}`, `{"schema":"corvint-capability-catalog/0","commandTrees":"bad"}`} {
		root := t.TempDir()
		files := syntheticRepository()
		files[machineCatalogPath] = body
		writeFiles(t, root, files)
		if len(kinds(CheckMachineCatalog(root))["malformed"]) == 0 {
			t.Errorf("malformed catalog accepted: %s", body)
		}
	}
	root := t.TempDir()
	files := syntheticRepository()
	delete(files, machineCatalogPath)
	writeFiles(t, root, files)
	if len(kinds(CheckMachineCatalog(root))["missing"]) == 0 {
		t.Fatal("missing catalog accepted")
	}
}

func syntheticMachineCatalog(trees ...clitree.Tree) string {
	catalog := capcatalog.Catalog{Schema: capcatalog.Schema, MCPTools: []capcatalog.MCPTool{{Name: "corvint.query", Availability: "corvint-mcp", Summary: "Query context"}}}
	for _, tree := range trees {
		catalog.CommandTrees = append(catalog.CommandTrees, capcatalog.CommandTreeRef{
			Binary: tree.Binary, Command: tree.HelpJSON, Snapshot: treesDir + "/" + tree.Binary + ".json",
		})
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// CAP-V0-8800: a newly declared tool must fail even if no existing catalog row
// changed, including a name outside the customary corvint namespace.
func TestCAPV08800NewMCPToolNeedsCatalogEntry(t *testing.T) {
	files := syntheticRepository()
	files["cmd/corvint-mcp/new.go"] = `package main
 var added = map[string]any{"name": "other.new_tool", "inputSchema": map[string]any{}}
 `
	root := t.TempDir()
	writeFiles(t, root, files)
	for _, finding := range CheckMachineCatalog(root) {
		if finding.Kind == "missing" && finding.Section == sectionTools && finding.Name == "other.new_tool" {
			return
		}
	}
	t.Fatal("new tool without a machine entry was accepted")
}
