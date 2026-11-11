// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"strings"
	"testing"
)

// attributionLines is selectPackages without the NCM-V1 decision: the rule
// (c) attribution of a path the decision now runs everything for, which
// packageBounds (tools/gate-ledger) still applies (V1-5424).
func attributionLines(plan Receipt, root string, withUnresolved bool) []string {
	return attributeFrom(plan, fixtureModule, root, withUnresolved, func() (*repositoryIndex, error) {
		return indexRepository(root, fixtureModule)
	}, false)
}

// attributionVerdict is verdict without the NCM-V1 decision.
func attributionVerdict(plan Receipt, root string) string {
	lines := attributionLines(plan, root, true)
	return lines[len(lines)-1]
}

// attributionStructural is structuralVerdict without the NCM-V1 decision.
func attributionStructural(plan Receipt, root string) (string, []string) {
	lines := attributionLines(plan, root, false)
	return lines[len(lines)-1], lines
}

// requireCorvintConfig asserts that both selectors run everything for a
// change under .corvint/, naming it (NCM-V1-006).
func requireCorvintConfig(t *testing.T, plan Receipt, root, dirty string) {
	t.Helper()
	want := "FALLBACK corvint-config " + dirty
	for name, lines := range map[string][]string{
		"Select":           Select(plan, fixtureModule, root),
		"SelectStructural": SelectStructural(plan, fixtureModule, root),
	} {
		if got := lines[len(lines)-1]; got != want {
			t.Fatalf("%s verdict = %q, want %q (NCM-V1-006):\n%s", name, got, want, strings.Join(lines, "\n"))
		}
	}
}
