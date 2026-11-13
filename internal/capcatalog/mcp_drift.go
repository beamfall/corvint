// SPDX-License-Identifier: Apache-2.0
package capcatalog

import (
	"fmt"
	"maps"
	"slices"
)

// MCPDrift compares a server's tools/list tools, decoded from the wire, with
// its table rows (MTT-V1-003): a listed tool without a row, a row without a
// listed tool, or a listed input schema whose property names differ from the
// row's described inputs. The table records no input type or required flag,
// so neither is compared. Each problem names the tool; none means no drift.
func MCPDrift(rows []MCPTool, listed []map[string]any) []string {
	want := map[string]MCPTool{}
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
