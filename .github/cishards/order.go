package cishards

import (
	"encoding/json"
	"sort"
	"strings"
)

// MaxPlanBytes bounds the advisory affected plan Order reads.
const MaxPlanBytes = 8 << 20

// Order returns packages with the Go units an affected-plan/0 document selected
// first: changed units and their dependents, then other bounded witnesses, then unbounded
// readers, then every unselected package. It only permutes its input; a plan it
// cannot read leaves the order unchanged and reports false.
func Order(packages []string, plan []byte) ([]string, bool) {
	var p struct {
		Profile string `json:"profile"`
		OK      bool   `json:"ok"`
		Plan    struct {
			Selected []struct {
				UnitID  string `json:"unitId"`
				Witness struct {
					Kind string `json:"kind"`
				} `json:"witness"`
			} `json:"selected"`
		} `json:"plan"`
	}
	if len(plan) > MaxPlanBytes || json.Unmarshal(plan, &p) != nil || p.Profile != "affected-plan/0" || !p.OK {
		return packages, false
	}
	const unselected = 3
	rank := map[string]int{}
	for _, u := range p.Plan.Selected {
		name, isGo := strings.CutPrefix(u.UnitID, "go:")
		if !isGo {
			continue
		}
		r := 1
		switch u.Witness.Kind {
		case "DIRECT_SOURCE_CHANGE", "DIRECT_TEST_CHANGE", "ENCLOSING_PACKAGE", "DEPENDENCY_PATH":
			r = 0
		case "UNBOUNDED_READER":
			r = 2
		}
		if old, ok := rank[name]; !ok || r < old {
			rank[name] = r
		}
	}
	of := func(name string) int {
		if r, ok := rank[name]; ok {
			return r
		}
		return unselected
	}
	out := append([]string(nil), packages...)
	sort.SliceStable(out, func(i, j int) bool { return of(out[i]) < of(out[j]) })
	return out, true
}

// Share is the advisory AFP-V0-025 shadow metric: the estimated milliseconds of
// the packages an affected plan selected, against the complete universe.
type Share struct {
	Profile               string `json:"profile"`
	SelectedPackages      int    `json:"selectedPackages"`
	UniversePackages      int    `json:"universePackages"`
	UnboundedPackages     int    `json:"unboundedPackages"`
	SelectedMilliseconds  int64  `json:"selectedMilliseconds"`
	UniverseMilliseconds  int64  `json:"universeMilliseconds"`
	UnboundedMilliseconds int64  `json:"unboundedMilliseconds"`
	UnpricedPackages      int    `json:"unpricedPackages"`
	SelectedPermille      int64  `json:"selectedPermille"`
}

// ShareOf prices the universe with the partition's cost estimates, an unpriced
// package at their median. It reports false for a plan or estimates it cannot
// read; it changes nothing about what runs.
func ShareOf(packages []string, plan []byte) (Share, bool) {
	return shareOf(packages, plan, defaultCosts)
}

func shareOf(packages []string, plan, costs []byte) (Share, bool) {
	universe, err := validate(packages)
	w, median, priced := weights(costs)
	ordered, ok := Order(universe, plan)
	if err != nil || !priced || !ok || len(universe) == 0 {
		return Share{}, false
	}
	// Order puts rank 0 and 1 first, then unbounded readers, then the unselected.
	selected, unbounded := selectedCounts(plan, universe)
	out := Share{Profile: "corvint-ci-selected-share/0", SelectedPackages: selected, UniversePackages: len(universe), UnboundedPackages: unbounded}
	for i, name := range ordered {
		ms, known := w[name]
		if !known {
			ms = median
			out.UnpricedPackages++
		}
		out.UniverseMilliseconds += ms
		if i < selected {
			out.SelectedMilliseconds += ms
		}
		if i >= selected-unbounded && i < selected {
			out.UnboundedMilliseconds += ms
		}
	}
	out.SelectedPermille = out.SelectedMilliseconds * 1000 / out.UniverseMilliseconds
	return out, true
}

// selectedCounts counts the universe packages a readable plan selected, and
// those selected only as unbounded readers.
func selectedCounts(plan []byte, universe []string) (selected, unbounded int) {
	var p struct {
		Plan struct {
			Selected []struct {
				UnitID  string `json:"unitId"`
				Witness struct {
					Kind string `json:"kind"`
				} `json:"witness"`
			} `json:"selected"`
		} `json:"plan"`
	}
	if json.Unmarshal(plan, &p) != nil {
		return 0, 0
	}
	in := map[string]bool{}
	for _, name := range universe {
		in[name] = true
	}
	bounded := map[string]bool{}
	seen := map[string]bool{}
	for _, u := range p.Plan.Selected {
		name, isGo := strings.CutPrefix(u.UnitID, "go:")
		if !isGo || !in[name] {
			continue
		}
		seen[name] = true
		if u.Witness.Kind != "UNBOUNDED_READER" {
			bounded[name] = true
		}
	}
	return len(seen), len(seen) - len(bounded)
}
