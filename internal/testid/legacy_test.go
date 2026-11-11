// SPDX-License-Identifier: Apache-2.0
package testid

import "testing"

// CTI-V1-012: legacy unit keys read at their own granularity: a Go unit as
// its package-level ID, a TypeScript runner unit and a Ruby test unit as
// their file-level IDs; a key that names no file (a Python module, a nested
// Go module, a runner outside the seven) has none.
func TestCTIV1012LegacyUnitKeys(t *testing.T) {
	t.Parallel()
	goDir := GoModuleDir("example.test/m", ".")
	for unit, want := range map[string]string{
		"go:example.test/m/internal/calc":        "go:internal/calc",
		"go:example.test/m":                      "go:.",
		"typescript:vitest:src/a.test.ts":        "vitest:src/a.test.ts",
		"typescript:jest:web/sum.test.js":        "jest:web/sum.test.js",
		"typescript:playwright:e2e/a.spec.ts":    "playwright:e2e/a.spec.ts",
		"ruby:rspec:spec/models/user_spec":       "rspec:spec/models/user_spec.rb",
		"ruby:minitest:test/calc_test":           "minitest:test/calc_test.rb",
		"ruby:rails-test:test/models/user_test":  "minitest:test/models/user_test.rb",
		"go:other.test/x":                        "",
		"go-module:tools/x":                      "",
		"python:pkg.mod":                         "",
		"typescript:mocha:test/a.js":             "",
		"typescript:cypress:cypress/e2e/a.cy.ts": "",
		"typescript:src/a.ts":                    "",
		"ruby:source:lib/calc":                   "",
		"ruby:rspec:":                            "",
		"typescript:vitest:../a.test.ts":         "",
		"no-colon":                               "",
	} {
		id, ok := FromUnit(unit, goDir)
		if got := map[bool]string{true: id.String(), false: ""}[ok]; got != want {
			t.Errorf("FromUnit(%q) = %q, %t; want %q", unit, got, ok, want)
		}
	}
	id, ok := FromUnit("typescript:vitest:src/a.test.ts", nil)
	if !ok || id.Granularity() != GranularityFile {
		t.Errorf("vitest unit granularity %q", id.Granularity())
	}
	if id, ok := FromUnit("go:example.test/m/a", goDir); !ok || id.Granularity() != GranularityPackage {
		t.Errorf("go unit granularity %q", id.Granularity())
	}
	if _, ok := FromUnit("go:example.test/m/a", nil); ok {
		t.Error("a Go unit was resolved without a module")
	}
}
