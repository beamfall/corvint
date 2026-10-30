package affected

import (
	"slices"
	"sort"
	"strings"
)

// This file is the selection half of ADN-V0-001..009: a Go package whose
// change only adds top-level declarations keeps its own tests, but carries the
// change only to the importers that reference one of the new identifiers. The
// golang plugin decides which packages qualify and which importers reference
// them; the graph only applies a record it can still check against the dirty
// set it is asked to plan, and falls back to package-level selection for
// anything else.

// GoAdditive record states (ADN-V0-001).
const (
	GoAdditiveNarrowed = "NARROWED"
	GoAdditiveFallback = "FALLBACK"
)

// GoAdditiveInvalidation names what would select a plan.goAdditiveExcluded
// unit again (ADN-V0-005).
const GoAdditiveInvalidation = "REFERENCE_TO_NEW_DECLARATION_OR_NON_ADDITIVE_CHANGE"

// GoAdditiveExclusion is one unit with tests the plan would reach only
// through a narrowed package's importers that reference none of its new
// declarations (ADN-V0-005). Through names that package, whose
// plan.goAdditive record is the witness; Universe is the graph digest as in
// Exclusion. It is a member of its own, not a plan.excluded entry, because
// that member's reason and invalidation enumerations are closed (CCF-V1-006).
type GoAdditiveExclusion struct {
	UnitID       string `json:"unitId"`
	Through      string `json:"through"`
	Universe     string `json:"universe"`
	Invalidation string `json:"invalidation"`
}

// Fallback reasons the graph itself records when it refuses a record
// (ADN-V0-004).
const (
	GoAdditiveNotDirectChange = "NOT_A_DIRECT_SOURCE_CHANGE"
	GoAdditiveDirtyMismatch   = "DIRTY_SET_DIFFERS_FROM_RECORD"
	GoAdditiveDataChange      = "DATA_PATH_CHANGE"
	GoAdditiveReachedThrough  = "REACHED_THROUGH_ANOTHER_CHANGED_UNIT"
)

// GoAdditive is one Go package's additive-change record (ADN-V0-001). A
// NARROWED record's witness is its Declarations, the new top-level
// identifiers, and its Unreferenced importers, which reference none of the
// exported ones; Referencing are the importers the plan still follows. A
// FALLBACK record states why the package is selected the ordinary way.
type GoAdditive struct {
	Unit         string   `json:"unit"`
	State        string   `json:"state"`
	Reason       string   `json:"reason,omitempty"`
	Paths        []string `json:"paths"`
	Declarations []string `json:"declarations,omitempty"`
	Referencing  []string `json:"referencing,omitempty"`
	Unreferenced []string `json:"unreferenced,omitempty"`
}

// WithGoAdditive returns a copy of graph whose selection applies the
// additive-change records (ADN-V0-002). Like the read-scope base they are a
// selection input, not part of the observed graph or its digest.
func (graph *Graph) WithGoAdditive(records []GoAdditive) *Graph {
	copied := *graph
	copied.goAdditive = make([]GoAdditive, 0, len(records))
	for _, record := range records {
		record.Paths = slices.Clone(record.Paths)
		record.Declarations = slices.Clone(record.Declarations)
		record.Referencing = slices.Clone(record.Referencing)
		record.Unreferenced = slices.Clone(record.Unreferenced)
		copied.goAdditive = append(copied.goAdditive, record)
	}
	sort.Slice(copied.goAdditive, func(i, j int) bool { return copied.goAdditive[i].Unit < copied.goAdditive[j].Unit })
	return &copied
}

// Dependents returns, sorted and unique, the units whose source or tests
// import id: the edges a narrowed record prunes (ADN-V0-003).
func (graph *Graph) Dependents(id string) []string {
	merged := append(append([]string(nil), graph.dependents[id]...), graph.testUsers[id]...)
	sort.Strings(merged)
	return slices.Compact(merged)
}

// additiveTraverse is traverse with each narrowed record's import and test
// import edges limited to its referencing importers (ADN-V0-003). A record is
// applied only when the package is a seed by its own source change, no data
// path reaches it, and the dirty paths in its directory are exactly the ones
// it was computed for (ADN-V0-004). A record whose package the traversal
// reaches from another unit is demoted and the walk rerun, so a package that
// changes because something it imports changed carries that change to every
// importer. It returns the reached set, the allowed importers of each applied
// record, and every record with its final state.
func (graph *Graph) additiveTraverse(start, traversed, enclosing map[string]Witness, normalized []string) (map[string]Witness, map[string]map[string]bool, []GoAdditive) {
	if len(graph.goAdditive) == 0 {
		return graph.traverse(start, nil), nil, nil
	}
	records := make([]GoAdditive, len(graph.goAdditive))
	copy(records, graph.goAdditive)
	allowed := make(map[string]map[string]bool)
	for index := range records {
		record := &records[index]
		if record.State != GoAdditiveNarrowed {
			continue
		}
		if reason := graph.additiveRefusal(*record, start, traversed, enclosing, normalized); reason != "" {
			record.State, record.Reason = GoAdditiveFallback, reason
			continue
		}
		set := make(map[string]bool, len(record.Referencing))
		for _, id := range record.Referencing {
			set[id] = true
		}
		allowed[record.Unit] = set
	}
	for {
		reached := graph.traverse(start, allowed)
		through := graph.reachedThrough(reached, allowed)
		if len(through) == 0 {
			return reached, allowed, records
		}
		for index := range records {
			if from, demoted := through[records[index].Unit]; demoted {
				records[index].State, records[index].Reason = GoAdditiveFallback, GoAdditiveReachedThrough+": "+from
				delete(allowed, records[index].Unit)
			}
		}
	}
}

// additiveRefusal is the graph's own check that a NARROWED record still
// describes this dirty set; empty means it applies.
func (graph *Graph) additiveRefusal(record GoAdditive, start, traversed, enclosing map[string]Witness, normalized []string) string {
	seed, ok := start[record.Unit]
	unit, known := graph.units[record.Unit]
	if !ok || !known || seed.Kind != WitnessDirectSource || len(unit.Sources) == 0 {
		return GoAdditiveNotDirectChange
	}
	if _, data := traversed[record.Unit]; data {
		return GoAdditiveDataChange
	}
	if _, data := enclosing[record.Unit]; data {
		return GoAdditiveDataChange
	}
	directory := relativeDirectory(unit.Sources[0])
	var inUnit []string
	for _, path := range normalized {
		if owner := graph.owner[path]; owner == record.Unit || relativeDirectory(path) == directory {
			inUnit = append(inUnit, path)
		}
	}
	paths := append([]string(nil), record.Paths...)
	sort.Strings(paths)
	if !slices.Equal(inUnit, slices.Compact(paths)) {
		return GoAdditiveDirtyMismatch
	}
	return ""
}

// reachedThrough names each applied record's package that the walk reaches by
// an edge it follows from another reached unit, with the smallest such unit.
func (graph *Graph) reachedThrough(reached map[string]Witness, allowed map[string]map[string]bool) map[string]string {
	through := make(map[string]string)
	for from := range reached {
		for class, edges := range []map[string][]string{graph.dependents, graph.advisory, graph.routed} {
			for _, to := range edges[from] {
				if _, narrowed := allowed[to]; !narrowed || to == from || !graph.follows(allowed, class, from, to) {
					continue
				}
				if prior, seen := through[to]; !seen || from < prior {
					through[to] = from
				}
			}
		}
	}
	return through
}

// follows reports whether the walk takes the edge from one unit to another:
// every edge except a narrowed package's import edge to an importer that
// references none of its new declarations.
func (graph *Graph) follows(allowed map[string]map[string]bool, class int, from, to string) bool {
	if class != 0 {
		return true
	}
	set, narrowed := allowed[from]
	return !narrowed || set[to]
}

// additiveExclusions names, for each unit the narrowed plan does not reach
// but the plan without the records does, the narrowed package its unnarrowed
// witness chain first passes through (ADN-V0-005).
func (graph *Graph) additiveExclusions(normalized []string, records []GoAdditive) map[string]string {
	narrowed := make(map[string]bool)
	for _, record := range records {
		if record.State == GoAdditiveNarrowed {
			narrowed[record.Unit] = true
		}
	}
	if len(narrowed) == 0 {
		return nil
	}
	plain := *graph
	plain.goAdditive = nil
	full, _, _, _, _, _, _, _ := plain.reach(normalized, true)
	through := make(map[string]string, len(full))
	for id, witness := range full {
		chain := witness.Chain()
		for _, step := range chain {
			if narrowed[step] {
				through[id] = step
				break
			}
		}
		if _, found := through[id]; !found && len(chain) != 0 {
			through[id] = chain[0]
		}
	}
	return through
}

// additiveSummary is the record list a plan reports: nil without records.
func additiveSummary(records []GoAdditive) []GoAdditive {
	if len(records) == 0 {
		return nil
	}
	out := make([]GoAdditive, len(records))
	copy(out, records)
	for index := range out {
		if out[index].State != GoAdditiveNarrowed {
			out[index].Referencing, out[index].Unreferenced = nil, nil
		}
		out[index].Reason = strings.TrimSpace(out[index].Reason)
	}
	return out
}
