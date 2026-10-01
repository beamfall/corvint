package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// NGI-V0-001 NGI-V0-005 NGI-V0-006: schema and runtime retain a closed
// language set and expose the capability limits to callers.
func TestNonGoImpactMCPAdmission(t *testing.T) {
	root := makeRepository(t)
	for _, suffix := range []string{"go", "rb", "js", "jsx", "mjs", "cjs", "ts", "tsx"} {
		body := "export function value() {}\n"
		if suffix == "go" {
			body = "package value\nfunc Value() {}\n"
		}
		if suffix == "rb" {
			body = "def value; end\n"
		}
		writeFile(t, filepath.Join(root, "src/value."+suffix), body)
	}
	gitOutput(t, root, "add", ".")
	gitOutput(t, root, "commit", "-qm", "non-Go MCP fixture")
	registry, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	var pattern, description string
	for _, tool := range registry.Tools() {
		if tool.Name == ToolImpact {
			description = tool.Description
			p := tool.InputSchema["properties"].(map[string]any)["paths"].(map[string]any)
			pattern = p["items"].(map[string]any)["pattern"].(string)
		}
	}
	if !strings.Contains(pattern, "(?:go|rb|js|jsx|mjs|cjs|ts|tsx)$") {
		t.Fatalf("schema=%s", pattern)
	}
	for _, suffix := range []string{"go", "rb", "js", "jsx", "mjs", "cjs", "ts", "tsx"} {
		if !validImpact(impactInput{Paths: []string{"src/value." + suffix}, Limit: 10}) {
			t.Fatalf("runtime refuses %s", suffix)
		}
		_, callErr := registry.Call(context.Background(), ToolImpact, []byte(`{"paths":["src/value.`+suffix+`"]}`))
		if callErr != nil {
			t.Fatalf("dispatch refuses %s", suffix)
		}
	}
	for _, value := range []string{"src/a.py", "src/a.sql", "/a.rb", "src/../a.rb", "src/a.RB", "src//a.ts"} {
		if validImpact(impactInput{Paths: []string{value}, Limit: 10}) {
			t.Fatalf("admitted %s", value)
		}
	}
	for _, language := range []string{"Go", "Ruby", "JavaScript", "TypeScript", "Dynamic dispatch"} {
		if !strings.Contains(description, language) {
			t.Fatalf("capability absent: %s", language)
		}
	}
}
