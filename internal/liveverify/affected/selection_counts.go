// SPDX-License-Identifier: Apache-2.0

package affected

import "strings"

// DriverSelection reports a selection of a driver unit, a make target or a
// workflow job (AZU-V0-001): its Kind is set, or its unit identity is in the
// driver namespace. A driver unit is never a test unit (SLF-V1-008).
func DriverSelection(unitID, kind string) bool {
	return kind != "" || strings.HasPrefix(unitID, DriverNamespace+":")
}

// SelectionCounts splits the plan's selections into test units and driver
// units (SLF-V1-008), so a summary never counts a driver as a test.
func (plan Plan) SelectionCounts() (tests, drivers int) {
	for _, selection := range plan.Selected {
		if DriverSelection(selection.UnitID, selection.Kind) {
			drivers++
		} else {
			tests++
		}
	}
	return tests, drivers
}
