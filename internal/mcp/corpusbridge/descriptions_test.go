package corpusbridge

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/docsbridge"
)

// docsToolTable is the 19 corpus docs tools kept after DMF-V1-009 (R42 B9): they are re-described,
// not pruned. A tool leaves this table only with a fixture test proving its answer a strict subset
// of another tool's for the same input (DMF-V1-008).
var docsToolTable = []string{
	"corvint.docs_claims", "corvint.docs_concept", "corvint.docs_coverage", "corvint.docs_dependencies",
	"corvint.docs_find_related", "corvint.docs_flow", "corvint.docs_gaps", "corvint.docs_get",
	"corvint.docs_get_journey", "corvint.docs_get_stability", "corvint.docs_info", "corvint.docs_intent",
	"corvint.docs_inventory", "corvint.docs_locate", "corvint.docs_navigation", "corvint.docs_search",
	"corvint.docs_trace", "corvint.docs_validate", "corvint.docs_vocabulary",
}

var alternativeTool = regexp.MustCompile("use corvint\\.docs_[a-z_]+")

// docsDescriptors is every corvint.docs_* tool a client can list: the corpus tools and the draft
// and consume tools.
func docsDescriptors() []bridge.ToolDescriptor {
	return append(AllTools(), docsbridge.DescribedTools()...)
}

// DMF-V1-008: every docs tool has a description no other docs tool shares, naming one alternative
// tool and the testing server, and every input property has a description; the 19-tool table is
// complete, so a removal without a strict-subset fixture names the missing tool.
func TestDMFV1008DistinctToolDescriptions(t *testing.T) {
	tools := docsDescriptors()
	seen := map[string]string{}
	names := []string{}
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, "corvint.docs_") {
			continue
		}
		names = append(names, tool.Name)
		if other, dup := seen[tool.Description]; dup {
			t.Errorf("%s and %s share a description", tool.Name, other)
		}
		seen[tool.Description] = tool.Name
		if !alternativeTool.MatchString(tool.Description) {
			t.Errorf("%s names no alternative tool: %q", tool.Name, tool.Description)
		}
		if !strings.Contains(tool.Description, "`corvint mcp`") {
			t.Errorf("%s does not point test questions to corvint mcp", tool.Name)
		}
		properties, _ := tool.InputSchema["properties"].(map[string]any)
		if len(properties) == 0 {
			t.Errorf("%s has no input properties", tool.Name)
		}
		for property, raw := range properties {
			schema, _ := raw.(map[string]any)
			if text, _ := schema["description"].(string); strings.TrimSpace(text) == "" {
				t.Errorf("%s property %s has no description", tool.Name, property)
			}
			if nested, ok := schema["properties"].(map[string]any); ok {
				for member, value := range nested {
					if m, _ := value.(map[string]any); m["description"] == nil || m["description"] == "" {
						t.Errorf("%s property %s member %s has no description", tool.Name, property, member)
					}
				}
			}
		}
	}
	for _, want := range append(slices.Clone(docsToolTable), "corvint.docs_draft", "corvint.docs_consume") {
		if !slices.Contains(names, want) {
			t.Errorf("docs tool %s is missing: a docs tool is removed only with a strict-subset fixture test (DMF-V1-008, R42 B9)", want)
		}
	}
	for _, name := range names {
		if !slices.Contains(docsToolTable, name) && name != "corvint.docs_draft" && name != "corvint.docs_consume" {
			t.Errorf("docs tool %s is not in the tool table", name)
		}
	}
}

// DMF-V1-009: no docs tool answers which tests to run: corvint.docs_recommend_tests is not listed and
// is refused as an unsupported tool, and every docs tool description sends test questions to
// corvint mcp.
func TestDMFV1009NoDocsTestSelectionTool(t *testing.T) {
	for _, tool := range docsDescriptors() {
		if tool.Name == "corvint.docs_recommend_tests" || strings.Contains(tool.Name, "test") {
			t.Fatalf("docs tool %s offers a test question", tool.Name)
		}
		if strings.HasPrefix(tool.Name, "corvint.docs_") && !strings.Contains(tool.Description, "corvint.affected on the testing server `corvint mcp`") {
			t.Fatalf("%s does not name corvint mcp for test questions", tool.Name)
		}
	}
	if _, ok := tools["corvint.docs_recommend_tests"]; ok {
		t.Fatal("corvint.docs_recommend_tests is still dispatched")
	}
	if len(AllTools()) != len(docsToolTable) {
		t.Fatalf("corpus tools %d, want the %d of the table", len(AllTools()), len(docsToolTable))
	}
}
