// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"strings"
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
	catalog := capcatalog.Catalog{Schema: capcatalog.Schema}
	// A valid fixture includes the checker binary's concrete offline descriptors.
	for _, name := range offlineToolNames() {
		catalog.MCPTools = append(catalog.MCPTools, capcatalog.MCPTool{Name: name, Availability: "fixture server", Summary: "Fixture tool"})
	}
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

// CAP-V0-8800: constant-backed names come from the offline descriptors, not a
// namespace or literal spelling. Every tasks-work tool must survive discovery.
func TestCAPV08800OfflineDescriptorOmissionFails(t *testing.T) {
	for _, name := range []string{"tasks_work_start", "tasks_work_deliver", "tasks_work_block", "tasks_work_finding", "tasks_work_close_merged", "tasks_work_release_sync"} {
		t.Run(name, func(t *testing.T) {
			entries := capcatalog.MCPTools()
			found := false
			for i, entry := range entries {
				if entry.Name == name {
					entries = append(entries[:i], entries[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("catalog lacks %s", name)
			}
			files := syntheticRepository()
			var catalog capcatalog.Catalog
			if err := json.Unmarshal([]byte(files[machineCatalogPath]), &catalog); err != nil {
				t.Fatal(err)
			}
			catalog.MCPTools = entries
			data, err := json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
			files[machineCatalogPath] = string(data)
			root := t.TempDir()
			writeFiles(t, root, files)
			findings := CheckMachineCatalog(root)
			for _, finding := range findings {
				if finding.Kind == "missing" && finding.Name == name {
					return
				}
			}
			t.Fatalf("omitted offline descriptor accepted: %s: %v", name, findings)
		})
	}
}

// CAP-V0-8800: an unfamiliar computed name needs no source-parser allowlist.
// The only admitted membership is an exact concrete descriptor name; an unknown
// name stays missing even when a familiar namespace entry is present.
func TestCAPV08800UnknownOfflineDescriptorFailsClosed(t *testing.T) {
	computedName := func(parts ...string) string { return strings.Join(parts, "_") }
	descriptor := bridge.ToolDescriptor{Name: computedName("unfamiliar", "computed", "tool")}
	findings := machineToolFindings([]string{descriptor.Name}, capcatalog.MCPTools())
	for _, finding := range findings {
		if finding.Kind == "missing" && finding.Name == descriptor.Name {
			return
		}
	}
	t.Fatalf("unknown computed descriptor accepted: %v", findings)
}
