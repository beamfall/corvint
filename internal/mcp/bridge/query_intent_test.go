package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestQueryDescriptionNamesAcceptedIntentsAndTheAlternative pins MCPV0-650 (V1-3705): the two
// audit tasks abstain UNSUPPORTED_QUERY_INTENT without an index build, and the corvint.query
// description gives an accepted example and names corvint.context for every other task.
func TestQueryDescriptionNamesAcceptedIntentsAndTheAlternative(t *testing.T) {
	registry, err := New(makeRepository(t))
	if err != nil {
		t.Fatal(err)
	}
	var description string
	for _, tool := range registry.Tools() {
		if tool.Name == ToolQuery {
			description = tool.Description
		}
	}
	for _, want := range []string{"UNSUPPORTED_QUERY_INTENT", "corvint.context", "for example: "} {
		if !strings.Contains(description, want) {
			t.Fatalf("corvint.query description lacks %q: %s", want, description)
		}
	}
	_, example, _ := strings.Cut(description, "for example: ")
	example, _, _ = strings.Cut(example, ";")
	if err := contextindex.ValidateQueryAuthorityStart(example, 1); err != nil {
		t.Fatalf("description example %q is refused: %v", example, err)
	}
	operations := registry.operations
	operations.buildQuery = func(context.Context, string, string) (*contextindex.Index, error) {
		t.Fatal("an unsupported intent started an index build")
		return nil, nil
	}
	isolated := registryWithOperations(registry, operations)
	for _, task := range []string{"how do I run the tests", "fix MCP server root binding"} {
		result, callErr := isolated.Call(context.Background(), ToolQuery, []byte(`{"task":"`+task+`"}`))
		if callErr != nil {
			t.Fatal(callErr)
		}
		if result.State != "ABSTAINED" || result.Abstention.Reason != "UNSUPPORTED_QUERY_INTENT" {
			t.Fatalf("%q = %#v, want the UNSUPPORTED_QUERY_INTENT abstention", task, result)
		}
	}
}
