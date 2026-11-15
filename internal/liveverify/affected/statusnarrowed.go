// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"strings"
)

// StatusNarrowing is one unit the plan does not select only because a dirty
// path's change status narrowed a name-listing read or a name-only walk
// (AFP-V0-11054). Gate is the requirement whose rule dropped it; Via is the
// unit on its unknown-status witness chain that holds the narrowed read, when
// that is not the unit itself; Paths are the dirty paths that would otherwise
// have selected it, at most MaxStatusNarrowedPaths of them, and PathCount
// their number.
type StatusNarrowing struct {
	UnitID    string       `json:"unitId"`
	Gate      string       `json:"gate"`
	Via       string       `json:"via,omitempty"`
	Paths     []StatusPath `json:"paths"`
	PathCount int          `json:"pathCount"`
}

// StatusPath is one dirty path with the change status the plan read for it.
type StatusPath struct {
	Path   string       `json:"path"`
	Status ChangeStatus `json:"status"`
}

// MaxStatusNarrowedPaths bounds the paths one StatusNarrowing lists.
const MaxStatusNarrowedPaths = 16

// The gates a StatusNarrowing names (AFP-V0-11054).
const (
	// StatusGateNameListing is a unit whose own name-listing read was
	// narrowed (AFP-V0-9962).
	StatusGateNameListing = "AFP-V0-9962"
	// StatusGateNameOnlyWalk is a name-only tree walker (AFP-V0-9963).
	StatusGateNameOnlyWalk = "AFP-V0-9963"
	// StatusGateChangeStatus is a unit no narrowed read on its chain
	// explains; the status input as a whole dropped it (AFP-V0-9960).
	StatusGateChangeStatus = "AFP-V0-9960"
)

// statusNarrowed lists, sorted by unit, each unit the same plan with every
// status read as unknown selects and plan does not (AFP-V0-11054). It plans
// a second time only when a dirty path is ChangeModified and some unit holds
// a name-listing read or a name-only walk, the only reads change status
// narrows; otherwise it is nil and costs one scan.
func (graph *Graph) statusNarrowed(plan Plan) []StatusNarrowing {
	if !graph.mayNarrowByStatus(plan.Dirty) {
		return nil
	}
	unknown := *graph
	unknown.status = nil
	counterfactual := selectPlan(&unknown, plan.Dirty)
	kept := make(map[string]bool, len(plan.Selected))
	for _, selection := range plan.Selected {
		kept[selection.UnitID] = true
	}
	var out []StatusNarrowing
	for _, selection := range counterfactual.Selected {
		id := selection.UnitID
		if kept[id] {
			continue
		}
		kept[id] = true
		gate, holder := graph.statusGate(id, selection.Witness.Chain())
		paths := graph.statusPaths(holder, gate, plan.Dirty)
		entry := StatusNarrowing{UnitID: id, Gate: gate, PathCount: len(paths)}
		if holder != id {
			entry.Via = holder
		}
		if len(paths) > MaxStatusNarrowedPaths {
			paths = paths[:MaxStatusNarrowedPaths]
		}
		entry.Paths = make([]StatusPath, len(paths))
		for index, dirtyPath := range paths {
			entry.Paths[index] = StatusPath{Path: dirtyPath, Status: graph.ChangeStatusOf(dirtyPath)}
		}
		out = append(out, entry)
	}
	slices.SortFunc(out, func(a, b StatusNarrowing) int { return strings.Compare(a.UnitID, b.UnitID) })
	return out
}

// mayNarrowByStatus reports whether change status can drop a unit from a
// plan over dirty: a path is ChangeModified and a unit holds a name-listing
// read or a name-only walk.
func (graph *Graph) mayNarrowByStatus(dirty []string) bool {
	modified := false
	for _, dirtyPath := range dirty {
		if !graph.listingMayChange(dirtyPath) {
			modified = true
			break
		}
	}
	if !modified {
		return false
	}
	if len(graph.namesOnly) != 0 {
		return true
	}
	for _, unit := range graph.units {
		if len(unit.NameListings) != 0 {
			return true
		}
	}
	return false
}

// statusGate names the rule that dropped id and the unit holding the
// narrowed read: id itself when it is a name-only walker or holds a
// name-listing read, else the first such unit on its witness chain, else id
// under the status input as a whole.
func (graph *Graph) statusGate(id string, chain []string) (string, string) {
	candidates := append([]string{id}, chain...)
	for _, candidate := range candidates {
		if graph.namesOnly[candidate] {
			return StatusGateNameOnlyWalk, candidate
		}
		if unit, ok := graph.units[candidate]; ok && len(unit.NameListings) != 0 {
			return StatusGateNameListing, candidate
		}
	}
	return StatusGateChangeStatus, id
}

// statusPaths is the sorted dirty paths that would otherwise have selected
// holder under gate: for a name-listing read, each path a name-listing token
// names and no other token of the unit names; for every other gate, and when
// that leaves none, every dirty path.
func (graph *Graph) statusPaths(holder, gate string, dirty []string) []string {
	paths := slices.Clone(dirty)
	if gate == StatusGateNameListing {
		unit := graph.units[holder]
		var listing, content []pathToken
		for _, value := range unit.PathTokens {
			if slices.Contains(unit.NameListings, value) {
				listing = append(listing, newPathToken(value))
			} else {
				content = append(content, newPathToken(value))
			}
		}
		goUnit := strings.HasPrefix(holder, "go:")
		var named []string
		for _, dirtyPath := range dirty {
			parts := strings.Split(dirtyPath, "/")
			anyDepth := goUnit && !resolvedEvidence(dirtyPath)
			if namesAny(listing, parts, anyDepth) && !namesAny(content, parts, anyDepth) {
				named = append(named, dirtyPath)
			}
		}
		if len(named) != 0 {
			paths = named
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}
