// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// coreDriftProfile selects every opt-in profile and group `corvint mcp`
// keeps, so the listing is the server's whole registered surface.
const coreDriftProfile = "flows,test-evidence,affected"

// listedWith lists `corvint mcp` tools for arguments with the embedding
// binary's affected hook, as `corvint mcp` serves them.
func listedWith(t *testing.T, arguments []string) []map[string]any {
	t.Helper()
	exit, responses, stderr := hookedSession(t, Hooks{Affected: stubAffectedRunner}, arguments, map[string]any{"method": "tools/list"})
	if exit != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", exit, stderr)
	}
	var tools []map[string]any
	for _, tool := range resultOf(t, responses[1])["tools"].([]any) {
		tools = append(tools, tool.(map[string]any))
	}
	return tools
}

// MTT-V1-003: `corvint mcp` with every profile and group it keeps registers
// exactly the table's corvint mcp rows and their input properties, and its
// default surface is a subset of them.
func TestMTTV1003CoreServerRegistersExactlyTheTable(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	rows := capcatalog.MCPToolsFor(capcatalog.ServerCore)
	if problems := capcatalog.MCPDrift(rows, listedWith(t, []string{"--root", root, "--tool-profile", coreDriftProfile})); len(problems) != 0 {
		t.Fatalf("corvint mcp drifts from the table:\n%s", strings.Join(problems, "\n"))
	}
	for _, name := range toolNames(listedWith(t, []string{"--root", root})) {
		if !slices.ContainsFunc(rows, func(row capcatalog.MCPTool) bool { return row.Name == name && row.OptIn == "" }) {
			t.Fatalf("default tool %s has no default-surface row", name)
		}
	}
}

// MTT-V1-003 scenario: a tool registered without a row, a row without a
// registered tool and a changed input schema each fail and name the tool.
func TestMTTV1003DriftNamesTheTool(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	rows := capcatalog.MCPToolsFor(capcatalog.ServerCore)
	listed := listedWith(t, []string{"--root", root, "--tool-profile", coreDriftProfile})
	added := append(slices.Clone(listed), map[string]any{"name": "corvint.unlisted_tool", "inputSchema": map[string]any{"type": "object"}})
	if problems := strings.Join(capcatalog.MCPDrift(rows, added), "\n"); !strings.Contains(problems, "corvint.unlisted_tool: registered with no table row") {
		t.Fatalf("an unlisted tool passed: %q", problems)
	}
	dropped := slices.DeleteFunc(slices.Clone(listed), func(tool map[string]any) bool { return tool["name"] == "corvint.status" })
	if problems := strings.Join(capcatalog.MCPDrift(rows, dropped), "\n"); !strings.Contains(problems, "corvint.status: table row with no registered tool") {
		t.Fatalf("a row without a tool passed: %q", problems)
	}
	changed := slices.Clone(listed)
	for index, tool := range changed {
		if tool["name"] == "corvint.impact" {
			changed[index] = map[string]any{"name": "corvint.impact", "inputSchema": map[string]any{"properties": map[string]any{"paths": map[string]any{}, "extra": map[string]any{}}}}
		}
	}
	if problems := strings.Join(capcatalog.MCPDrift(rows, changed), "\n"); !strings.Contains(problems, "corvint.impact: registered input properties") {
		t.Fatalf("a changed schema passed: %q", problems)
	}
}
