// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedFlatSelection is one selectedFlat entry (AFP-V0-9987): what a lane or
// CI reads to learn what was selected and why without walking plan.selected.
// It is derived after selection and never feeds it.
type affectedFlatSelection struct {
	UnitID string `json:"unitId"`
	// Test is the Go package import path for a go: unit, the unit's only test
	// file when it lists exactly one, and the unit ID otherwise.
	Test      string `json:"test"`
	Witness   string `json:"witness"`
	DirtyPath string `json:"dirtyPath"`
	Reason    string `json:"reason"`
}

// affectedFlatSelected lists each selection in plan order. compact is the
// projection of full, index for index.
func affectedFlatSelected(full []affected.Selection, compact []affectedCompactSelection) []affectedFlatSelection {
	flat := make([]affectedFlatSelection, 0, len(full))
	for index, selection := range full {
		witness := compact[index].Witness
		flat = append(flat, affectedFlatSelection{
			UnitID:    selection.UnitID,
			Test:      affectedFlatTest(selection),
			Witness:   witness.Kind,
			DirtyPath: witness.DirtyPath,
			Reason:    affectedFlatReason(witness),
		})
	}
	return flat
}

func affectedFlatTest(selection affected.Selection) string {
	if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok && pkg != "" {
		return pkg
	}
	if len(selection.Tests) == 1 {
		return selection.Tests[0]
	}
	return selection.UnitID
}

// affectedFlatReason states the witness in one line: the dirty path, then
// the detail the witness carries.
func affectedFlatReason(witness affectedCompactWitness) string {
	reason := "dirty path " + witness.DirtyPath
	if witness.AnyDirtyPath {
		reason = "any dirty path, such as " + witness.DirtyPath
	}
	if witness.Cause != nil {
		reason += "; unbounded reads of " + witness.Cause.Unit + ": " + witness.Cause.Reads
	}
	if witness.Glob != "" {
		reason += "; glob " + witness.Glob
	}
	if witness.ReadScope != nil {
		reason += "; read scope " + witness.ReadScope.Declaration + " entry " + witness.ReadScope.Entry
	}
	if witness.ViaLength > 1 {
		reason += fmt.Sprintf("; via a %d-unit chain", witness.ViaLength)
	}
	return reason
}

// affectedUnresolvedPackages counts the selected go: units no literal or
// read-scope declaration bounds, which every change selects (AFP-V0-9987,
// AFP-V0-012 rule (d)).
func affectedUnresolvedPackages(selections []affected.Selection) int {
	count := 0
	for _, selection := range selections {
		if selection.Witness.Kind == affected.WitnessUnboundedReader && strings.HasPrefix(selection.UnitID, "go:") {
			count++
		}
	}
	return count
}
