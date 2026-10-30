package dispatch

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RoleShadow reports that an enabled ticket role is shadowed (CAL-V0-290):
// another enabled ticket role with a lower priority value, which the roster
// prefers, matches every ticket Role matches. Role then launches a ticket
// only while ShadowedBy cannot take it (its cap, a tier cap, a budget, or a
// CAL-V0-149 relaunch kept on Role).
type RoleShadow struct {
	Role, ShadowedBy             string
	Priority, ShadowedByPriority int
}

// Message is the operator warning for the shadow.
func (s RoleShadow) Message() string {
	return fmt.Sprintf("role %s (priority %d) is shadowed by role %s (priority %d): lower priority values win and %s matches every ticket %s matches, so %s launches one only while %s cannot take it",
		s.Role, s.Priority, s.ShadowedBy, s.ShadowedByPriority, s.ShadowedBy, s.Role, s.Role, s.ShadowedBy)
}

// RoleShadows lists the shadowed roles of a valid configuration in role
// order, each with the covering role the roster prefers most (lowest
// priority value, then the earlier role). The coverage check is
// conservative: a role is reported only when every ticket its match admits
// is provably admitted by the other role's match, so a reported shadow is
// never a false positive, while an overlap the check cannot prove (several
// roles covering one together, glob containment beyond equality or a
// literal ID) is not reported. Lane roles and disabled (cap 0) roles are
// never reported and never shadow. The roster order is unchanged.
func (c *Config) RoleShadows() []RoleShadow {
	var out []RoleShadow
	for _, a := range c.Roles {
		if a.Cap == 0 || a.Match == nil {
			continue
		}
		var best *Role
		for i := range c.Roles {
			b := &c.Roles[i]
			if b.Name == a.Name || b.Cap == 0 || b.Match == nil || b.Priority >= a.Priority || !matchCovers(b.Match, a.Match) {
				continue
			}
			if best == nil || b.Priority < best.Priority {
				best = b
			}
		}
		if best != nil {
			out = append(out, RoleShadow{Role: a.Name, ShadowedBy: best.Name, Priority: a.Priority, ShadowedByPriority: best.Priority})
		}
	}
	return out
}

// matchCovers reports whether every ticket a admits is provably admitted by
// b under matches. It answers false whenever containment is not certain.
func matchCovers(b, a *Match) bool {
	if a.Pool != b.Pool || b.PlanSelected && !a.PlanSelected {
		return false
	}
	if !subset(defaultStatuses(a.Statuses), defaultStatuses(b.Statuses)) || !subset(b.Labels, a.Labels) {
		return false
	}
	if len(b.Kinds) > 0 && (len(a.Kinds) == 0 || !subset(a.Kinds, b.Kinds)) {
		return false
	}
	return globCovers(b.IDGlob, a.IDGlob) && statesCover(b, a) && gatesCover(b.Gates, a.Gates)
}

func defaultStatuses(s []string) []string {
	if len(s) == 0 {
		return []string{"OPEN"}
	}
	return s
}

// subset reports whether every member of xs is in ys.
func subset(xs, ys []string) bool {
	for _, x := range xs {
		if !contains(ys, x) {
			return false
		}
	}
	return true
}

// globCovers is certain only for an absent b glob, equal globs, or a literal
// a glob (one local ID) that b matches.
func globCovers(b, a string) bool {
	if b == "" || b == a {
		return true
	}
	if a == "" || strings.ContainsAny(a, `*?[\`) {
		return false
	}
	ok, _ := filepath.Match(b, a)
	return ok
}

// statesCover reports whether b's work-state predicate admits every state
// a's admits (stateMatches).
func statesCover(b, a *Match) bool {
	if len(b.States)+len(b.ExcludeStates) == 0 {
		return true // b admits every state, UNKNOWN included
	}
	if len(a.States)+len(a.ExcludeStates) == 0 {
		return false // a admits UNKNOWN, which b refuses
	}
	if len(a.States) > 0 {
		for _, s := range a.States {
			if s != StateUnknown && !contains(a.ExcludeStates, s) && !stateMatches(b, s) {
				return false
			}
		}
		return true
	}
	// a admits every state outside its exclusions and UNKNOWN.
	if len(b.States) > 0 {
		return false
	}
	for _, e := range b.ExcludeStates {
		if e != StateUnknown && !contains(a.ExcludeStates, e) {
			return false
		}
	}
	return true
}

// gatesCover reports whether every gate predicate of b is implied by a's
// predicate on the same gate.
func gatesCover(b, a []GateMatch) bool {
	for _, bg := range b {
		found := false
		for _, ag := range a {
			if ag.Gate == bg.Gate {
				found = subset(ag.States, bg.States)
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
