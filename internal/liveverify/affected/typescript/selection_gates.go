// SPDX-License-Identifier: Apache-2.0
package typescript

// The SLF-V1-006 enable switches of the TypeScript narrowing rules. Each stays
// false until the rule's FIH-V1 report passes: while it is false the plugin
// keeps the frontier the rule would discharge, so the rule ships tested but
// inert. There is no flag or environment variable, so no run can turn an
// unproven rule on; the commit that retains the passing report flips it.
var (
	// globExpansionNarrowing enables SLF-V1-003: a literal import.meta.glob or
	// require.context call expands to one edge per matching file and a
	// membership reader instead of raising typescript:dynamic-import.
	globExpansionNarrowing = false
	// literalConfigNarrowing enables SLF-V1-005: a Vitest or Jest config whose
	// every recognised marker holds a literal value is evaluated instead of
	// raising typescript:executable-config-unresolved.
	literalConfigNarrowing = false
)
