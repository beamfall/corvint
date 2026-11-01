// SPDX-License-Identifier: Apache-2.0
package gateselect

import "strings"

// AllowedUnknown is the one allowlist of plan.unknown entries a narrowed run
// may carry (AFP-V0-982): the deciding driver's unknownNarrowing (CIN-V0-860)
// and the shadow `corvint test` (sessiontest, AFP-V0-028) both decide through
// it. A LANGUAGE_FRONTIER names a category, not a unit, except a Go frontier
// other than build-constraint variants and the nested-module frontier, which
// may hide a Go reader; an UNOWNED_DIRTY_PATH is read only by the Go packages
// rules (b) to (d) select; a NO_SELECTABLE_TEST unit gets no test from a FULL
// run either. Every other entry forces FULL.
func AllowedUnknown(reason, detail string) bool {
	switch reason {
	case "LANGUAGE_FRONTIER":
		return !strings.HasPrefix(detail, "go:") ||
			detail == "go:build-constraint-variants" || detail == "go:nested-module-frontier"
	case "UNOWNED_DIRTY_PATH", "NO_SELECTABLE_TEST":
		return true
	}
	return false
}
