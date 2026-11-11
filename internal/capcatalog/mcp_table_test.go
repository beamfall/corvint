// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"slices"
	"strings"
	"testing"
)

// MTT-V1-001, MTT-V1-006: the authored table passes every table rule: unique
// names across the servers, every column set, distinct descriptions, a
// description on every input property, the shared error codes on every
// tool, and the Tasks "no MCP tool" row with server none.
func TestMTTV1001TableRules(t *testing.T) {
	rows := MCPTools()
	if problems := CheckMCPTools(rows); len(problems) != 0 {
		t.Fatalf("table problems:\n%s", strings.Join(problems, "\n"))
	}
	if !slices.IsSortedFunc(rows, func(left, right MCPTool) int { return strings.Compare(left.Name, right.Name) }) {
		t.Fatal("table is not sorted by name")
	}
	var tasks []MCPTool
	for _, row := range rows {
		if strings.Contains(row.Name, "tasks") {
			tasks = append(tasks, row)
		}
	}
	if len(tasks) != 1 || tasks[0].Server != ServerNone {
		t.Fatalf("tasks rows %+v, want one with server none", tasks)
	}
	docs := 0
	for _, row := range MCPToolsFor(ServerDocs) {
		if strings.HasPrefix(row.Name, "corvint.docs_") {
			docs++
		}
	}
	// 20 corpus tools until docs-mcp-hardening removes docs_recommend_tests
	// (DMF-V1-009), plus docs_draft and docs_consume; docs_how is planned.
	if docs != 22 {
		t.Fatalf("%d registered docs rows on %s, want 22", docs, ServerDocs)
	}
	if how := rowNamed(rows, "corvint.docs_how"); how.Server != ServerDocs || how.Availability != AvailabilityPlanned || how.Registered() {
		t.Fatalf("docs_how row %+v", how)
	}
}

// MTT-V1-001 scenarios: a duplicate name, a shared description, an input
// property without a description and a row without the shared error codes
// each fail and name the row.
func TestMTTV1001TableRulesNameTheBrokenRow(t *testing.T) {
	base := MCPTools()
	cases := map[string]struct {
		edit func([]MCPTool) []MCPTool
		want string
	}{
		"duplicate name across servers": {func(rows []MCPTool) []MCPTool {
			status := rowNamed(rows, "corvint.status")
			status.Server = ServerDocs
			status.Summary = "a second status"
			return append(rows, status)
		}, "corvint.status: duplicate tool name"},
		"shared description": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.docs_get" })
			rows[index].Summary = rowNamed(rows, "corvint.docs_info").Summary
			return rows
		}, "shares its description with"},
		"input without description": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.impact" })
			rows[index].Inputs[0].Description = ""
			return rows
		}, "corvint.impact: input property budgetBytes has no description"},
		"missing server-busy": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.help" })
			rows[index].Errors = []string{ErrorCancelled}
			return rows
		}, "corvint.help: typed errors lack server-busy"},
		"missing cancelled": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.docs_search" })
			rows[index].Errors = []string{ErrorServerBusy}
			return rows
		}, "corvint.docs_search: typed errors lack cancelled"},
		"no tasks row": {func(rows []MCPTool) []MCPTool {
			return slices.DeleteFunc(rows, func(row MCPTool) bool { return row.Server == ServerNone })
		}, "no no-MCP-tool row for Tasks"},
		"two dots": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.status" })
			rows[index].Name = "corvint.repo.status"
			return rows
		}, "corvint.repo.status: name is not corvint.<area>_<verb> with one dot"},
		"cli equivalent without parity test": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.impact" })
			rows[index].ParityTest = ""
			return rows
		}, "corvint.impact: has a CLI equivalent but no parity test (MTT-V1-009)"},
		"no output profile": {func(rows []MCPTool) []MCPTool {
			index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == "corvint.status" })
			rows[index].OutputProfile = ""
			return rows
		}, "corvint.status: missing output profile"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rows := tc.edit(cloneRows(base))
			problems := strings.Join(CheckMCPTools(rows), "\n")
			if !strings.Contains(problems, tc.want) {
				t.Fatalf("problems %q lack %q", problems, tc.want)
			}
		})
	}
}

func rowNamed(rows []MCPTool, name string) MCPTool {
	index := slices.IndexFunc(rows, func(row MCPTool) bool { return row.Name == name })
	if index < 0 {
		return MCPTool{}
	}
	return rows[index]
}

func cloneRows(rows []MCPTool) []MCPTool {
	out := slices.Clone(rows)
	for index := range out {
		out[index].Inputs = slices.Clone(out[index].Inputs)
		out[index].Errors = slices.Clone(out[index].Errors)
	}
	return out
}

// MTT-V1-007 scenario: every served name has one dot and the flow gaps tool
// is corvint.flows_gaps, replacing corvint.flows.gaps.
func TestMTTV1007OneNamingConvention(t *testing.T) {
	rows := MCPTools()
	gaps := rowNamed(rows, "corvint.flows_gaps")
	if gaps.Server != ServerCore || !slices.Equal(gaps.Replaces, []string{"corvint.flows.gaps"}) {
		t.Fatalf("flows gaps row %+v", gaps)
	}
	for _, row := range rows {
		if row.Server != ServerNone && strings.Count(row.Name, ".") != 1 {
			t.Fatalf("%s has more than one dot", row.Name)
		}
		if strings.HasPrefix(row.Name, "corvint.flows_") && len(row.Replaces) != 1 {
			t.Fatalf("renamed flow tool %s does not list its old name", row.Name)
		}
	}
}
