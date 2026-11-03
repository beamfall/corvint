// Package funclink links functional units to their code and their tests (Map Coverage Join V0,
// docs/specs/map-coverage-join-v0.md, MCJ-V0-020..024). A functional unit is a screen, a flow, a
// page object, a page-object method, a step or a setup record. Each unit carries a code set and a
// test set whose entries keep the highest authority that offered them: declared, then static, then
// observed. The selection rules built on the links only add tests; the fallback replacement is a
// shadow report and never narrows a plan.
package funclink

import (
	"sort"
	"strconv"
)

// Authority ranks a link's source, highest first (MCJ-V0-020).
type Authority int

const (
	Observed Authority = 1 << iota
	Static
	Declared
)

func (a Authority) String() string {
	switch a {
	case Declared:
		return "declared"
	case Static:
		return "static"
	case Observed:
		return "observed"
	}
	return "unknown"
}

// MarshalText writes the authority name.
func (a Authority) MarshalText() ([]byte, error) { return []byte(a.String()), nil }

// highest is the highest authority in an offer mask.
func highest(mask Authority) Authority {
	for _, a := range []Authority{Declared, Static, Observed} {
		if mask&a != 0 {
			return a
		}
	}
	return 0
}

// Kind is a functional-unit kind (Definitions: Functional unit).
type Kind string

const (
	KindScreen     Kind = "screen"
	KindFlow       Kind = "flow"
	KindPageObject Kind = "page-object"
	KindMethod     Kind = "method"
	KindStep       Kind = "step"
	KindSetup      Kind = "setup"
)

// Span is a repository path and an inclusive 1-based line range. Start 0 means the whole file.
type Span struct {
	Path  string `json:"path"`
	Start int    `json:"start_line,omitempty"`
	End   int    `json:"end_line,omitempty"`
}

func (s Span) String() string {
	if s.Start == 0 {
		return s.Path
	}
	return s.Path + ":" + strconv.Itoa(s.Start) + "-" + strconv.Itoa(s.End)
}

// Overlaps reports whether two spans share a line of one path; a whole-file span overlaps every
// span of its path.
func (s Span) Overlaps(o Span) bool {
	if s.Path != o.Path {
		return false
	}
	if s.Start == 0 || o.Start == 0 {
		return true
	}
	return s.Start <= o.End && o.Start <= s.End
}

// contains reports whether s holds every line of o.
func (s Span) contains(o Span) bool {
	if s.Path != o.Path {
		return false
	}
	if s.Start == 0 {
		return true
	}
	return o.Start != 0 && s.Start <= o.Start && o.End <= s.End
}

func lessSpan(a, b Span) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	return a.End < b.End
}

// CodeEntry is one code-set entry with the authority that keeps it and every authority offering it.
type CodeEntry struct {
	Span      Span        `json:"span"`
	Authority Authority   `json:"authority"`
	Offered   []Authority `json:"offered"`
}

// TestEntry is one test-set entry. Test identities are opaque: each source names tests in its own
// identity space, and joining spaces is the caller's.
type TestEntry struct {
	Test      string      `json:"test"`
	Authority Authority   `json:"authority"`
	Offered   []Authority `json:"offered"`
}

// Unit is one functional unit with its code set and test set.
type Unit struct {
	Kind  Kind        `json:"kind"`
	ID    string      `json:"id"`
	Code  []CodeEntry `json:"code"`
	Tests []TestEntry `json:"tests"`
}

// Ref names a unit.
type Ref struct {
	Kind Kind   `json:"kind"`
	ID   string `json:"id"`
}

func (r Ref) key() string { return string(r.Kind) + " " + r.ID }

func lessRef(a, b Ref) bool {
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.ID < b.ID
}

// SurfaceKind is a test-side surface kind (MCJ-V0-023).
type SurfaceKind string

const (
	SurfaceMethod     SurfaceKind = "page-object-method"
	SurfaceStepHelper SurfaceKind = "step-helper"
	SurfaceBinding    SurfaceKind = "selector-binding"
	SurfaceSetup      SurfaceKind = "setup-record"
	SurfaceFlow       SurfaceKind = "flow-declaration"
	// SurfacePageObject is the rest of a page object's span: a change there that no method or
	// binding span holds selects every test of the page object.
	SurfacePageObject SurfaceKind = "page-object"
)

// Surface is one test-side surface with the tests that use it.
type Surface struct {
	Kind      SurfaceKind `json:"kind"`
	ID        string      `json:"id"`
	Unit      Ref         `json:"unit"`
	Span      Span        `json:"span"`
	Tests     []string    `json:"tests"`
	Authority Authority   `json:"authority"`
}

// Disagreement is one entry that the unit's highest authority and a lower authority do not both
// offer (MCJ-V0-020, MCJ-V0-011). The higher authority's tag is kept; the lower entry is kept at
// its own authority, so a disagreement never removes a test.
type Disagreement struct {
	Unit   Ref       `json:"unit"`
	Set    string    `json:"set"` // code or tests
	Item   string    `json:"item"`
	Higher Authority `json:"higher"`
	Lower  Authority `json:"lower"`
	// Only is the authority that offered the item: higher or lower.
	Only string `json:"only"`
}

type unitAcc struct {
	ref   Ref
	code  map[Span]Authority
	tests map[string]Authority
}

// Builder accumulates offers from every source. An offer only adds to a mask, so a lower authority
// never overwrites a higher one.
type Builder struct {
	units    map[string]*unitAcc
	surfaces map[string]*surfaceAcc
}

type surfaceAcc struct {
	s     Surface
	tests map[string]bool
}

// NewBuilder returns an empty builder.
func NewBuilder() *Builder {
	return &Builder{units: map[string]*unitAcc{}, surfaces: map[string]*surfaceAcc{}}
}

func (b *Builder) unit(r Ref) *unitAcc {
	u := b.units[r.key()]
	if u == nil {
		u = &unitAcc{ref: r, code: map[Span]Authority{}, tests: map[string]Authority{}}
		b.units[r.key()] = u
	}
	return u
}

// Unit registers a unit with no entries, so a unit read from a source is listed even when empty.
func (b *Builder) Unit(r Ref) { b.unit(r) }

// AddCode offers one code-set entry.
func (b *Builder) AddCode(r Ref, s Span, a Authority) {
	if s.Path == "" || a == 0 {
		return
	}
	b.unit(r).code[s] |= a
}

// AddTest offers one test-set entry.
func (b *Builder) AddTest(r Ref, test string, a Authority) {
	if test == "" || a == 0 {
		return
	}
	b.unit(r).tests[test] |= a
}

// AddSurface offers one test-side surface and its tests; the tests also join the unit's test set.
func (b *Builder) AddSurface(kind SurfaceKind, id string, r Ref, s Span, tests []string, a Authority) {
	b.unit(r)
	k := string(kind) + " " + id + " " + r.key() + " " + s.String()
	acc := b.surfaces[k]
	if acc == nil {
		acc = &surfaceAcc{s: Surface{Kind: kind, ID: id, Unit: r, Span: s}, tests: map[string]bool{}}
		b.surfaces[k] = acc
	}
	if a > acc.s.Authority {
		acc.s.Authority = a
	}
	for _, t := range tests {
		if t != "" {
			acc.tests[t] = true
			b.AddTest(r, t, a)
		}
	}
}

// Links is the built join: units, test-side surfaces and authority disagreements, sorted.
type Links struct {
	Units         []Unit         `json:"units"`
	Surfaces      []Surface      `json:"surfaces"`
	Disagreements []Disagreement `json:"disagreements"`
}

func offered(mask Authority) []Authority {
	out := []Authority{}
	for _, a := range []Authority{Declared, Static, Observed} {
		if mask&a != 0 {
			out = append(out, a)
		}
	}
	return out
}

// Build returns the sorted links.
func (b *Builder) Build() *Links {
	l := &Links{Units: []Unit{}, Surfaces: []Surface{}, Disagreements: []Disagreement{}}
	keys := make([]string, 0, len(b.units))
	for k := range b.units {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		acc := b.units[k]
		u := Unit{Kind: acc.ref.Kind, ID: acc.ref.ID, Code: []CodeEntry{}, Tests: []TestEntry{}}
		for s, m := range acc.code {
			u.Code = append(u.Code, CodeEntry{Span: s, Authority: highest(m), Offered: offered(m)})
		}
		sort.Slice(u.Code, func(i, j int) bool { return lessSpan(u.Code[i].Span, u.Code[j].Span) })
		for t, m := range acc.tests {
			u.Tests = append(u.Tests, TestEntry{Test: t, Authority: highest(m), Offered: offered(m)})
		}
		sort.Slice(u.Tests, func(i, j int) bool { return u.Tests[i].Test < u.Tests[j].Test })
		l.Units = append(l.Units, u)
		codeItems := map[string]Authority{}
		for s, m := range acc.code {
			codeItems[s.String()] = m
		}
		l.Disagreements = append(l.Disagreements, disagree(acc.ref, "code", codeItems)...)
		l.Disagreements = append(l.Disagreements, disagree(acc.ref, "tests", acc.tests)...)
	}
	sort.SliceStable(l.Units, func(i, j int) bool {
		return lessRef(Ref{l.Units[i].Kind, l.Units[i].ID}, Ref{l.Units[j].Kind, l.Units[j].ID})
	})
	skeys := make([]string, 0, len(b.surfaces))
	for k := range b.surfaces {
		skeys = append(skeys, k)
	}
	sort.Strings(skeys)
	for _, k := range skeys {
		acc := b.surfaces[k]
		s := acc.s
		s.Tests = sortedSet(acc.tests)
		l.Surfaces = append(l.Surfaces, s)
	}
	return l
}

// disagree compares the set's highest present authority with each lower one present.
func disagree(r Ref, set string, items map[string]Authority) []Disagreement {
	var all Authority
	for _, m := range items {
		all |= m
	}
	top := highest(all)
	out := []Disagreement{}
	names := make([]string, 0, len(items))
	for n := range items {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, lower := range []Authority{Static, Observed} {
		if lower >= top || all&lower == 0 {
			continue
		}
		for _, n := range names {
			m := items[n]
			switch {
			case m&top != 0 && m&lower == 0:
				out = append(out, Disagreement{Unit: r, Set: set, Item: n, Higher: top, Lower: lower, Only: "higher"})
			case m&top == 0 && m&lower != 0:
				out = append(out, Disagreement{Unit: r, Set: set, Item: n, Higher: top, Lower: lower, Only: "lower"})
			}
		}
	}
	return out
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Unit returns the unit a ref names, or nil.
func (l *Links) Unit(r Ref) *Unit {
	i := sort.Search(len(l.Units), func(i int) bool { return !lessRef(Ref{l.Units[i].Kind, l.Units[i].ID}, r) })
	if i < len(l.Units) && l.Units[i].Kind == r.Kind && l.Units[i].ID == r.ID {
		return &l.Units[i]
	}
	return nil
}
