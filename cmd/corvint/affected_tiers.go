package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedTiers ranks every selected unit by how directly the dirty set
// reaches it (AFP-V0-162). It never adds or removes a selection: tier 0 and 1
// are a bounded local subset of the full sound selection, and tier 2 is the
// remainder a lane leaves to CI with the gap recorded.
type affectedTiers struct {
	Counts  [3]int         `json:"counts"`
	Summary string         `json:"summary"`
	Units   map[string]int `json:"units"`
}

// affectedTierSummary is the one human-readable per-tier count line.
const affectedTierSummary = "tier0=%d tier1=%d tier2=%d"

// tierAffectedSelections assigns each selection its tier: 0 when the unit
// owns or encloses a dirty path; 1 when it is a direct dependent of a changed
// unit (a one-edge witness chain) or shares the parent directory of a tier-0
// unit whose dirty path lies at least three directories deep (so the parent
// is not a top-level directory); 2 otherwise, path readers included, since most changes touch a spec or
// build-log path that many packages name, and so is a ROUTE_DISPATCH_BOOT
// unit, which every controller change reaches (RUBY-AFFECTED-V0-049). It
// returns nil for an empty selection, so the member is then omitted.
func tierAffectedSelections(selected []affected.Selection) *affectedTiers {
	if len(selected) == 0 {
		return nil
	}
	tiers := &affectedTiers{Units: make(map[string]int, len(selected))}
	parents := map[string]bool{}
	for _, selection := range selected {
		switch selection.Witness.Kind {
		case affected.WitnessDirectSource, affected.WitnessDirectTest, affected.WitnessEnclosingPackage:
			tiers.Units[selection.UnitID] = 0
			// A top-level directory such as internal/ or cmd/ is no
			// neighbourhood: its children are siblings only by layout.
			if strings.Contains(path.Dir(path.Dir(selection.Witness.DirtyPath)), "/") {
				parents[affectedUnitParent(selection.UnitID)] = true
			}
		}
	}
	for _, selection := range selected {
		if _, changed := tiers.Units[selection.UnitID]; changed {
			continue
		}
		tier := 2
		switch selection.Witness.Kind {
		case affected.WitnessDependency, affected.WitnessAdvisoryConstant:
			if selection.Witness.ViaLength() <= 2 {
				tier = 1
			}
		}
		if parents[affectedUnitParent(selection.UnitID)] {
			tier = 1
		}
		tiers.Units[selection.UnitID] = tier
	}
	for _, tier := range tiers.Units {
		tiers.Counts[tier]++
	}
	tiers.Summary = fmt.Sprintf(affectedTierSummary, tiers.Counts[0], tiers.Counts[1], tiers.Counts[2])
	return tiers
}

// affectedUnitParent is the parent directory of a unit id's path part, the
// text after its language namespace ("go:a/b/c" has parent "a/b").
func affectedUnitParent(unitID string) string {
	_, rest, found := strings.Cut(unitID, ":")
	if !found {
		rest = unitID
	}
	return path.Dir(rest)
}
