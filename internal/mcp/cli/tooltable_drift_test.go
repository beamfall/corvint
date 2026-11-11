// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/mcp/testvaliditybridge"
)

// coreDriftProfile selects every opt-in profile and group `corvint mcp`
// keeps, so the listing is the server's whole registered surface.
const coreDriftProfile = "flows,test-evidence,affected"

// tableDrift compares a server's listed tools with its table rows
// (MTT-V1-003): a listed tool without a row, a row without a listed tool, or
// a listed input schema whose properties differ from the row's described
// inputs. Each problem names the tool.
func tableDrift(rows []capcatalog.MCPTool, listed []map[string]any) []string {
	want := map[string]capcatalog.MCPTool{}
	for _, row := range rows {
		want[row.Name] = row
	}
	var problems []string
	seen := map[string]bool{}
	for _, tool := range listed {
		name, _ := tool["name"].(string)
		seen[name] = true
		row, ok := want[name]
		if !ok {
			problems = append(problems, name+": registered with no table row")
			continue
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		registered := slices.Sorted(maps.Keys(properties))
		var described []string
		for _, input := range row.Inputs {
			described = append(described, input.Name)
		}
		slices.Sort(described)
		if !slices.Equal(registered, described) {
			problems = append(problems, fmt.Sprintf("%s: registered input properties %q, table describes %q", name, registered, described))
		}
	}
	for _, row := range rows {
		if !seen[row.Name] {
			problems = append(problems, row.Name+": table row with no registered tool")
		}
	}
	return problems
}

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
	if problems := tableDrift(rows, listedWith(t, []string{"--root", root, "--tool-profile", coreDriftProfile})); len(problems) != 0 {
		t.Fatalf("corvint mcp drifts from the table:\n%s", strings.Join(problems, "\n"))
	}
	for _, name := range toolNames(listedWith(t, []string{"--root", root})) {
		if !slices.ContainsFunc(rows, func(row capcatalog.MCPTool) bool { return row.Name == name && row.OptIn == "" }) {
			t.Fatalf("default tool %s has no default-surface row", name)
		}
	}
}

// MTT-V1-003: the retiring corvint-test-validity-mcp registers exactly its
// table row beside the shared help tool.
func TestMTTV1003TestValidityServerRegistersExactlyTheTable(t *testing.T) {
	var listed []map[string]any
	for _, tool := range (&testvaliditybridge.Registry{}).Tools() {
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		listed = append(listed, decoded)
	}
	if problems := tableDrift(capcatalog.MCPToolsFor(capcatalog.ServerTestValidity), listed); len(problems) != 0 {
		t.Fatalf("corvint-test-validity-mcp drifts from the table:\n%s", strings.Join(problems, "\n"))
	}
}

// MTT-V1-003 scenario: a tool registered without a row, a row without a
// registered tool and a changed input schema each fail and name the tool.
func TestMTTV1003DriftNamesTheTool(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	rows := capcatalog.MCPToolsFor(capcatalog.ServerCore)
	listed := listedWith(t, []string{"--root", root, "--tool-profile", coreDriftProfile})
	added := append(slices.Clone(listed), map[string]any{"name": "corvint.unlisted_tool", "inputSchema": map[string]any{"type": "object"}})
	if problems := strings.Join(tableDrift(rows, added), "\n"); !strings.Contains(problems, "corvint.unlisted_tool: registered with no table row") {
		t.Fatalf("an unlisted tool passed: %q", problems)
	}
	dropped := slices.DeleteFunc(slices.Clone(listed), func(tool map[string]any) bool { return tool["name"] == "corvint.status" })
	if problems := strings.Join(tableDrift(rows, dropped), "\n"); !strings.Contains(problems, "corvint.status: table row with no registered tool") {
		t.Fatalf("a row without a tool passed: %q", problems)
	}
	changed := slices.Clone(listed)
	for index, tool := range changed {
		if tool["name"] == "corvint.impact" {
			changed[index] = map[string]any{"name": "corvint.impact", "inputSchema": map[string]any{"properties": map[string]any{"paths": map[string]any{}, "extra": map[string]any{}}}}
		}
	}
	if problems := strings.Join(tableDrift(rows, changed), "\n"); !strings.Contains(problems, "corvint.impact: registered input properties") {
		t.Fatalf("a changed schema passed: %q", problems)
	}
}
