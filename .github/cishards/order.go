package cishards

import (
	"encoding/json"
	"sort"
	"strings"
)

// MaxPlanBytes bounds the advisory affected plan Order reads.
const MaxPlanBytes = 8 << 20

// Order returns packages with the Go units an affected-plan/0 document selected
// first: changed units and their dependents, then bounded readers, then unbounded
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
		case "DIRECT_SOURCE_CHANGE", "DIRECT_TEST_CHANGE", "DEPENDENCY_PATH":
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
