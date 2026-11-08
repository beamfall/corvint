// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// CAP-V0-8800: standalone discovery must reach the public completeness gate,
// including constant names outside corvint.* and expressions it cannot prove.
func TestCAPV08800StandaloneDescriptorDiscovery(t *testing.T) {
	for _, row := range []struct {
		name, declaration, expression, kind string
	}{
		{"literal", "", `"other.new_tool"`, "missing"},
		{"constant", `const extraTool = "other.new_tool"`, "extraTool", "missing"},
		{"unknown call", `func extraTool() string { return "other.new_tool" }`, "extraTool()", "malformed"},
		{"variable", `var extraTool = "other.new_tool"`, "extraTool", "malformed"},
		{"constant expression", `const extraTool = "other." + "new_tool"`, "extraTool", "malformed"},
		{"unresolved", "", "extraTool", "malformed"},
	} {
		for _, shape := range []string{"struct", "map"} {
			t.Run(row.name+"/"+shape, func(t *testing.T) {
				files := syntheticRepository()
				body := `bridge.ToolDescriptor{Name: ` + row.expression + `, InputSchema: map[string]any{}}`
				if shape == "map" {
					body = `map[string]any{"name": ` + row.expression + `, "inputSchema": map[string]any{}}`
				}
				files["cmd/corvint-corpus-mcp/extra.go"] = "package main\n" + row.declaration + "\nvar extra = " + body
				root := t.TempDir()
				writeFiles(t, root, files)
				findings := CheckMachineCatalog(root)
				if len(findings) != 1 || findings[0].Kind != row.kind || findings[0].Section != sectionTools {
					t.Fatalf("want one %s tool finding, got %v", row.kind, findings)
				}
				if row.kind == "missing" && findings[0].Name != "other.new_tool" {
					t.Fatalf("constant/literal name lost: %v", findings)
				}
				if row.kind == "malformed" && !strings.Contains(findings[0].Detail, "standalone MCP descriptor name is unproven") {
					t.Fatalf("unknown expression did not fail closed: %v", findings)
				}
				// Supplying a row repairs proven names, but must never certify an
				// unsupported expression merely because its literal also appears.
				var catalog capcatalog.Catalog
				if err := json.Unmarshal([]byte(files[machineCatalogPath]), &catalog); err != nil {
					t.Fatal(err)
				}
				catalog.MCPTools = append(catalog.MCPTools, capcatalog.MCPTool{Name: "other.new_tool", Availability: "fixture", Summary: "fixture"})
				encoded, err := json.Marshal(catalog)
				if err != nil {
					t.Fatal(err)
				}
				files[machineCatalogPath] = string(encoded)
				writeFiles(t, root, files)
				findings = CheckMachineCatalog(root)
				if row.kind == "missing" && len(findings) != 0 || row.kind == "malformed" && (len(findings) != 1 || findings[0].Kind != "malformed") {
					t.Fatalf("catalog entry changed discovery proof incorrectly: %v", findings)
				}
			})
		}
	}
}
