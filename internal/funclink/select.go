package funclink

import "sort"

// ReasonFunctionalityLink is the plan reason of a test MCJ-V0-021 adds.
const ReasonFunctionalityLink = "functionality-link"

// ReasonTestSurface is the plan reason of a test MCJ-V0-023 adds.
const ReasonTestSurface = "test-surface"

// Reason says why one test was added: the rule, the unit and authority that link the test, and
// the changed block that reached the unit.
type Reason struct {
	Rule      string    `json:"rule"`
	Unit      Ref       `json:"unit"`
	Authority Authority `json:"authority"`
	Block     Span      `json:"block"`
	Surface   string    `json:"surface,omitempty"`
}

// Added is one test a rule adds, with every reason that adds it.
type Added struct {
	Test    string   `json:"test"`
	Reasons []Reason `json:"reasons"`
}

// Selection is the result of a rule that only adds: Tests is the kept set plus every added test.
type Selection struct {
	Tests []string `json:"tests"`
	Added []Added  `json:"added"`
	// Uncovered lists the changed blocks no fresh record covers; Unlinked those of them that no
	// unit's code set holds, which keep the static plan (MCJ-V0-024).
	Uncovered []Span `json:"uncovered,omitempty"`
	Unlinked  []Span `json:"unlinked,omitempty"`
	// Unresolved lists the changed test-side surfaces whose users SKH-V0 extracted none of: the
	// caller keeps its static plan for them (MCJ-V0-023).
	Unresolved []Surface `json:"unresolved,omitempty"`
}

type adder struct {
	keep    map[string]bool
	reasons map[string][]Reason
}

func newAdder(keep []string) *adder {
	a := &adder{keep: map[string]bool{}, reasons: map[string][]Reason{}}
	for _, t := range keep {
		a.keep[t] = true
	}
	return a
}

func (a *adder) add(test string, r Reason) { a.reasons[test] = append(a.reasons[test], r) }

func (a *adder) selection() Selection {
	all := map[string]bool{}
	for t := range a.keep {
		all[t] = true
	}
	sel := Selection{Added: []Added{}}
	for t := range a.reasons {
		all[t] = true
	}
	sel.Tests = sortedSet(all)
	for _, t := range sortedSet(boolKeys(a.reasons)) {
		rs := a.reasons[t]
		sort.Slice(rs, func(i, j int) bool { return lessReason(rs[i], rs[j]) })
		sel.Added = append(sel.Added, Added{Test: t, Reasons: dedupeReasons(rs)})
	}
	return sel
}

func boolKeys(m map[string][]Reason) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func lessReason(a, b Reason) bool {
	if a.Unit != b.Unit {
		return lessRef(a.Unit, b.Unit)
	}
	if a.Block != b.Block {
		return lessSpan(a.Block, b.Block)
	}
	if a.Surface != b.Surface {
		return a.Surface < b.Surface
	}
	return a.Authority > b.Authority
}

func dedupeReasons(rs []Reason) []Reason {
	out := []Reason{}
	for i, r := range rs {
		if i > 0 && r == rs[i-1] {
			continue
		}
		out = append(out, r)
	}
	return out
}

func covered(b Span, fresh []Span) bool {
	for _, f := range fresh {
		if f.contains(b) {
			return true
		}
	}
	return false
}

// AddUncovered is MCJ-V0-021. keep is the plan's selected tests, changed the changed blocks and
// fresh the blocks a fresh coverage record covers. A changed block that no fresh block holds
// whole is uncovered; the test set of every unit whose code set holds an uncovered block is added
// with reason functionality-link, naming the unit and the authority of the code entry that links
// the block. The result always holds every test of keep: the rule only adds.
func (l *Links) AddUncovered(keep []string, changed, fresh []Span) Selection {
	a := newAdder(keep)
	var uncovered, unlinked []Span
	for _, b := range changed {
		if covered(b, fresh) {
			continue
		}
		uncovered = append(uncovered, b)
		linked := false
		for _, u := range l.Units {
			auth := codeAuthority(u, b)
			if auth == 0 {
				continue
			}
			linked = true
			for _, t := range u.Tests {
				a.add(t.Test, Reason{Rule: ReasonFunctionalityLink, Unit: Ref{u.Kind, u.ID}, Authority: auth, Block: b})
			}
		}
		if !linked {
			unlinked = append(unlinked, b)
		}
	}
	sel := a.selection()
	sel.Uncovered, sel.Unlinked = uncovered, unlinked
	return sel
}

// codeAuthority is the highest authority among the unit's code entries overlapping b, or 0.
func codeAuthority(u Unit, b Span) Authority {
	var best Authority
	for _, c := range u.Code {
		if c.Span.Overlaps(b) && c.Authority > best {
			best = c.Authority
		}
	}
	return best
}

// SelectSurfaces is MCJ-V0-023. Each changed block on a test-side surface adds exactly the tests
// that use that surface. A page object's own span counts only for a block that none of its method
// or binding spans holds whole, so a block reaching past them selects every test of the page
// object. A surface with no extracted users adds nothing and is listed as
// unresolved. The result always holds every test of keep.
func (l *Links) SelectSurfaces(keep []string, changed []Span) Selection {
	a := newAdder(keep)
	unresolved := map[string]Surface{}
	for _, b := range changed {
		var narrow []Span
		for _, s := range l.Surfaces {
			if (s.Kind == SurfaceBinding || s.Kind == SurfaceMethod || s.Kind == SurfaceStepHelper) && s.Span.contains(b) {
				narrow = append(narrow, s.Span)
			}
		}
		for _, s := range l.Surfaces {
			if !s.Span.Overlaps(b) || (s.Kind == SurfacePageObject && holdsAny(s.Span, narrow)) {
				continue
			}
			if len(s.Tests) == 0 {
				unresolved[string(s.Kind)+" "+s.ID+" "+s.Span.String()] = s
				continue
			}
			for _, t := range s.Tests {
				a.add(t, Reason{Rule: ReasonTestSurface, Unit: s.Unit, Authority: s.Authority, Block: b, Surface: string(s.Kind) + " " + s.ID})
			}
		}
	}
	sel := a.selection()
	for _, k := range sortedKeys(unresolved) {
		sel.Unresolved = append(sel.Unresolved, unresolved[k])
	}
	return sel
}

func holdsAny(outer Span, inner []Span) bool {
	for _, s := range inner {
		if outer.contains(s) {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ShadowReplacement is the MCJ-V0-022 shadow record for one change that took a broader static
// fallback. It never narrows a plan: CI still runs Fallback, and a fallback miss is a test of
// Omitted that failed in that run (FallbackMisses).
type ShadowReplacement struct {
	Mode string `json:"mode"` // always "shadow"
	// Eligible is true when every changed block lies in the code set of a unit with a declared or
	// static code entry; Ineligible then names each block that kept it false and why.
	Eligible   bool         `json:"eligible"`
	Ineligible []Ineligible `json:"ineligible,omitempty"`
	Units      []LinkedUnit `json:"units"`
	Fallback   []string     `json:"fallback"`
	Proposed   []string     `json:"proposed"`
	Omitted    []string     `json:"omitted"`
}

// Ineligible is one changed block that keeps the fallback: unlinked, or linked only through units
// whose code set is only observed.
type Ineligible struct {
	Block  Span   `json:"block"`
	Reason string `json:"reason"` // unlinked or observed-only
}

// LinkedUnit is one unit whose test set the proposed linked set holds, and its code authority.
type LinkedUnit struct {
	Unit      Ref       `json:"unit"`
	Authority Authority `json:"authority"`
}

// ShadowReplace computes the linked set that could replace fallback for changed. A unit whose code
// set is only observed never contributes; a changed block with no other link makes the change
// ineligible. The proposed set and the omitted fallback tests are recorded either way.
func (l *Links) ShadowReplace(changed []Span, fallback []string) ShadowReplacement {
	r := ShadowReplacement{Mode: "shadow", Eligible: len(changed) > 0, Units: []LinkedUnit{}, Fallback: sortedUnique(fallback)}
	proposed := map[string]bool{}
	units := map[Ref]Authority{}
	for _, b := range changed {
		linked, observedOnly := false, false
		for _, u := range l.Units {
			auth := codeAuthority(u, b)
			if auth == 0 {
				continue
			}
			if onlyObserved(u) {
				observedOnly = true
				continue
			}
			linked = true
			ref := Ref{u.Kind, u.ID}
			if auth > units[ref] {
				units[ref] = auth
			}
			for _, t := range u.Tests {
				proposed[t.Test] = true
			}
		}
		if !linked {
			r.Eligible = false
			reason := "unlinked"
			if observedOnly {
				reason = "observed-only"
			}
			r.Ineligible = append(r.Ineligible, Ineligible{Block: b, Reason: reason})
		}
	}
	for ref, a := range units {
		r.Units = append(r.Units, LinkedUnit{Unit: ref, Authority: a})
	}
	sort.Slice(r.Units, func(i, j int) bool { return lessRef(r.Units[i].Unit, r.Units[j].Unit) })
	r.Proposed = sortedSet(proposed)
	r.Omitted = []string{}
	for _, t := range r.Fallback {
		if !proposed[t] {
			r.Omitted = append(r.Omitted, t)
		}
	}
	return r
}

func onlyObserved(u Unit) bool {
	if len(u.Code) == 0 {
		return false
	}
	for _, c := range u.Code {
		if c.Authority != Observed {
			return false
		}
	}
	return true
}

func sortedUnique(in []string) []string {
	m := map[string]bool{}
	for _, t := range in {
		if t != "" {
			m[t] = true
		}
	}
	return sortedSet(m)
}

// FallbackMisses returns the tests the fallback ran that failed while the proposed linked set
// omitted them (MCJ-V0-022). Any miss keeps replacement in shadow.
func FallbackMisses(r ShadowReplacement, failed []string) []string {
	f := map[string]bool{}
	for _, t := range failed {
		f[t] = true
	}
	out := []string{}
	for _, t := range r.Omitted {
		if f[t] {
			out = append(out, t)
		}
	}
	return out
}

// Gaps is the MCJ-V0-024 report.
type Gaps struct {
	// Untested lists the units with a code set and an empty test set.
	Untested []Ref `json:"untested"`
	// Unmapped lists the tests of the given universe that no unit's test set holds.
	Unmapped []string `json:"unmapped"`
	// Unlinked lists the changed blocks that no unit's code set holds; they keep the static plan.
	Unlinked []Span `json:"unlinked"`
}

// Gaps reports untested functionality, unmapped tests among tests, and unlinked changed code.
func (l *Links) Gaps(tests []string, changed []Span) Gaps {
	g := Gaps{Untested: []Ref{}, Unmapped: []string{}, Unlinked: []Span{}}
	linked := map[string]bool{}
	for _, u := range l.Units {
		if len(u.Code) > 0 && len(u.Tests) == 0 {
			g.Untested = append(g.Untested, Ref{u.Kind, u.ID})
		}
		for _, t := range u.Tests {
			linked[t.Test] = true
		}
	}
	for _, t := range sortedUnique(tests) {
		if !linked[t] {
			g.Unmapped = append(g.Unmapped, t)
		}
	}
	for _, b := range changed {
		in := false
		for _, u := range l.Units {
			if codeAuthority(u, b) != 0 {
				in = true
				break
			}
		}
		if !in {
			g.Unlinked = append(g.Unlinked, b)
		}
	}
	return g
}
