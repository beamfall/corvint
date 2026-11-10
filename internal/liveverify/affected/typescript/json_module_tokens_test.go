package typescript

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// jsonModuleRepository is a vitest project whose test imports a JSON config
// module naming, in its artifact member, a fixture file the test then reads
// at run time (the beamfall-web-data edit-oracle case, V1-5183).
func jsonModuleRepository(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"web","private":true,"devDependencies":{"vitest":"2.1.0"}}`+"\n")
	write(t, root, "vitest.config.ts", "import { defineConfig } from 'vitest/config';\nexport default defineConfig({});\n")
	write(t, root, "test/token-contract.config.json", config)
	write(t, root, "test/fixtures/beamfall-tokens.json", "{\"tokens\":{\"color\":\"#fff\"}}\n")
	write(t, root, "test/fixtures/other.json", "{}\n")
	write(t, root, "test/tokenContract.test.ts", "import { readFileSync } from 'node:fs';\nimport { test, expect } from 'vitest';\nimport contract from './token-contract.config.json';\n\n"+
		"test('tokens', () => {\n  const body = readFileSync(contract.artifact, 'utf8');\n  expect(JSON.parse(body)).toBeTruthy();\n});\n")
	write(t, root, "test/other.test.ts", "import { test, expect } from 'vitest';\ntest('other', () => { expect(1).toBe(1); });\n")
	return root
}

// TestJSONModulePathsAreCandidateReads_TJAA_V0_622 selects a TypeScript test
// for an edit to a fixture that only an imported JSON module names: the
// module's path-shaped strings are candidate reads of every unit that imports
// it, so a byte change to the fixture reaches the test (PATH_LITERAL_READER),
// while a sibling fixture the module does not name selects nothing. A module
// that is not valid JSON keeps every path-shaped token of its text.
func TestJSONModulePathsAreCandidateReads_TJAA_V0_622(t *testing.T) {
	for _, tc := range []struct{ name, config string }{
		{"valid", "{\n  \"artifact\": \"test/fixtures/beamfall-tokens.json\",\n  \"schema\": 1\n}\n"},
		{"invalid-json", "{\n  \"artifact\": \"test/fixtures/beamfall-tokens.json\",\n  trailing\n"},
		{"escaped-slashes", "{\"artifact\":\"test\\/fixtures\\/beamfall-tokens.json\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := jsonModuleRepository(t, tc.config)
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"test/fixtures/beamfall-tokens.json"})
			var selected *affected.Selection
			for index, selection := range plan.Selected {
				if slices.Contains(selection.Tests, "test/tokenContract.test.ts") {
					selected = &plan.Selected[index]
				}
			}
			if selected == nil {
				t.Fatalf("tokenContract.test.ts not selected for the fixture its JSON config names: %+v unknown %+v", plan.Selected, plan.Unknown)
			}
			if selected.Witness.Kind != affected.WitnessPathLiteralReader {
				t.Errorf("witness %+v, want PATH_LITERAL_READER", selected.Witness)
			}
			for _, selection := range plan.Selected {
				if slices.Contains(selection.Tests, "test/other.test.ts") {
					t.Errorf("other.test.ts selected by %+v; it imports no module naming the fixture", selection.Witness)
				}
			}
			plan = affected.Select(graph, []string{"test/fixtures/other.json"})
			for _, selection := range plan.Selected {
				if slices.Contains(selection.Tests, "test/tokenContract.test.ts") {
					t.Errorf("tokenContract.test.ts selected for a fixture no module names: %+v", selection.Witness)
				}
			}
		})
	}
}
