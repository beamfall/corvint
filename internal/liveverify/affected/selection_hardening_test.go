// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// Finding 11 settle experiment (selection-hardening task 6a, R42 (panel pick)
// A8): a Go-only edit must not admit the TypeScript plugin, whose open
// DISCOVERY and OUTBOUND frontiers would otherwise bear and pull in its whole
// suite or turn the plan UNKNOWN. The TypeScript project here holds an
// executable Vitest config, a dynamic import and two tests; the Go edit
// reaches none of them.
func TestFinding11GoOnlyEditDoesNotAdmitTypeScript(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":               "module example.test/mixed\n\ngo 1.22\n",
		"pkg/value.go":         "package pkg\n\nfunc Value() int { return 1 }\n",
		"pkg/value_test.go":    "package pkg\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"web/package.json":     `{"devDependencies":{"vitest":"1"}}`,
		"web/vitest.config.js": "export default { test: { include: [process.env.SUITE] } }\n",
		"web/src/a.ts":         "export const a = () => import(process.env.MODULE)\n",
		"web/src/a.test.ts":    "import { test } from \"vitest\"\nimport { a } from \"./a\"\ntest(\"a\", () => a)\n",
		"web/src/b.test.ts":    "import { test } from \"vitest\"\ntest(\"b\", () => 1)\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), typescript.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"pkg/value.go"})
	if plan.Scope != affected.ScopeBounded || slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool {
		return strings.HasPrefix(unknown.Detail, "typescript:")
	}) {
		t.Fatalf("Go-only edit admitted TypeScript unknowns: scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	for _, test := range plan.SelectedTests() {
		if strings.HasPrefix(test, "web/") {
			t.Fatalf("Go-only edit selected TypeScript test %s: %v", test, plan.SelectedTests())
		}
	}
	if !slices.Contains(plan.SelectedTests(), "pkg/value_test.go") {
		t.Fatalf("selected = %v", plan.SelectedTests())
	}
	// The control: a TypeScript edit below the config does admit it.
	if control := affected.Select(graph, []string{"web/src/a.ts"}); control.Scope != affected.ScopeUnknown {
		t.Fatalf("control: scope=%s unknown=%+v", control.Scope, control.Unknown)
	}
}
