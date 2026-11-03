package linecov

import (
	"errors"
	"sort"
	"time"
)

// SegmentKind names the boundary a collector drew around one segment
// (docs/specs/map-coverage-join-v0.md, Definitions).
type SegmentKind string

const (
	KindStep   SegmentKind = "step"   // a runner step such as a Playwright test.step
	KindMethod SegmentKind = "method" // one call of a page-object method
	KindSetup  SegmentKind = "setup"  // one setup unit (SKH setup record)
	KindTest   SegmentKind = "test"   // the whole test: no finer boundary
)

// SegmentKinds lists every kind a collector qualifies with a fault-injection
// case (MCJ-V0-014).
var SegmentKinds = []SegmentKind{KindStep, KindMethod, KindSetup, KindTest}

// LevelUnknown is the level of a record that declares none (MCJ-V0-007).
const LevelUnknown = "unknown"

// Segment is one bounded part of one test run (MCJ-V0-001). Its reach is
// the blocks whose Segs name it.
type Segment struct {
	Test int32 // index into Index.Tests
	Kind SegmentKind
	// ID is the segment identity: an SKH-V0 record or method ID, a setup
	// record ID, or a step title and anchor. Empty for a whole-test segment
	// and for blocks a segmented record ran outside every segment.
	ID string `json:",omitempty"`
	// Bound reports that the identity was found at the record's commit and
	// anchor blob (MCJ-V0-002). An unbound segment joins no unit and no
	// equivalence class.
	Bound bool `json:",omitempty"`
	// Prior is the bound setup record the segment starts from, when the
	// collector knows one (MCJ-V0-005 condition 3).
	Prior   string        `json:",omitempty"`
	Units   []string      `json:",omitempty"` // opaque functional-unit ids
	Elapsed time.Duration `json:",omitempty"`
}

// Label is the segment's identity, or a placeholder naming why it has none.
func (s Segment) Label() string {
	switch {
	case s.ID != "":
		return s.ID
	case s.Kind == KindTest:
		return "(whole test)"
	default:
		return "(outside every segment)"
	}
}

// SegmentRecord is one segment of a Record as a collector reports it: its
// identity and the cover profile of the blocks it executed.
type SegmentRecord struct {
	Kind    SegmentKind
	ID      string
	Bound   bool
	Prior   string
	Units   []string
	Elapsed time.Duration
	Profile []byte
}

// BlockRef names one block: Index.Files[File].Blocks[Block].
type BlockRef struct{ File, Block int32 }

// checkSegments refuses an index whose segments or block references do not
// agree, so a damaged index decides nothing.
func (ix *Index) checkSegments() error {
	prev := int32(-1)
	for _, s := range ix.Segments {
		if s.Test < 0 || int(s.Test) >= len(ix.Tests) || s.Test < prev || s.Test > prev+1 {
			return errors.New("linecov: segments out of test order")
		}
		prev = s.Test
	}
	if len(ix.Segments) > 0 && int(prev) != len(ix.Tests)-1 {
		return errors.New("linecov: a test has no segment")
	}
	for _, f := range ix.Files {
		for _, b := range f.Blocks {
			for _, t := range b.Tests {
				if t < 0 || int(t) >= len(ix.Tests) {
					return errors.New("linecov: block names an absent test")
				}
			}
			if len(ix.Segments) == 0 {
				if b.Segs != nil {
					return errors.New("linecov: block names a segment of an unsegmented index")
				}
				continue
			}
			if len(b.Tests) > 0 && len(b.Segs) == 0 {
				return errors.New("linecov: executed block names no segment")
			}
			owners := map[int32]bool{}
			for i, s := range b.Segs {
				if s < 0 || int(s) >= len(ix.Segments) || i > 0 && s <= b.Segs[i-1] {
					return errors.New("linecov: block names an absent segment")
				}
				owners[ix.Segments[s].Test] = true
			}
			if len(owners) != len(b.Tests) {
				return errors.New("linecov: block segments and tests disagree")
			}
			for _, t := range b.Tests {
				if !owners[t] {
					return errors.New("linecov: block segments and tests disagree")
				}
			}
		}
	}
	for _, d := range ix.Divergent {
		if d.Class == "" {
			return errors.New("linecov: divergent class without identity")
		}
	}
	return nil
}

// Segment returns segment i of the index, synthesized when it stores none.
func (ix *Index) Segment(i int32) Segment { return ix.segs[i] }

// TestSegments returns the segment indexes of test t in run order.
func (ix *Index) TestSegments(t int32) []int32 {
	out := make([]int32, 0, ix.segFirst[t+1]-ix.segFirst[t])
	for s := ix.segFirst[t]; s < ix.segFirst[t+1]; s++ {
		out = append(out, s)
	}
	return out
}

// BlockSegments returns the segments whose reach holds block b.
func (ix *Index) BlockSegments(b BlockRef) []int32 {
	return ix.blockSegs(&ix.Files[b.File].Blocks[b.Block])
}

func (ix *Index) blockSegs(b *Block) []int32 {
	if b.Segs != nil || len(ix.Segments) > 0 {
		return b.Segs
	}
	out := make([]int32, len(b.Tests))
	for i, t := range b.Tests {
		out[i] = ix.segFirst[t]
	}
	return out
}

// BlocksAt returns the blocks of path that touch lines [lo, hi], and whether
// the index instruments path at blob oid; a caller treats a false as no
// record (MCJ-V0-021).
func (ix *Index) BlocksAt(path, oid string, lo, hi int) ([]BlockRef, bool) {
	i, ok := ix.byPath[path]
	if !ok || ix.Files[i].OID != oid {
		return nil, false
	}
	var out []BlockRef
	for bi, b := range ix.Files[i].Blocks {
		if int(b.Start) > hi {
			break
		}
		if int(b.End) >= lo {
			out = append(out, BlockRef{int32(i), int32(bi)})
		}
	}
	return out, true
}

// TestID is the stable identity of test t, "package.Name" (LPCV-V0-022).
func (ix *Index) TestID(t int32) string {
	return ix.Tests[t].Package + "." + ix.Tests[t].Name
}

func (ix *Index) level(t int32) string {
	if l := ix.Tests[t].Level; l != "" {
		return l
	}
	return LevelUnknown
}

// TestReach is how a change reaches one test (MCJ-V0-004).
type TestReach struct {
	Test     int32   `json:"-"`
	ID       string  // package.Name
	Level    string  // MCJ-V0-007; unknown when undeclared
	Segments []int32 `json:"-"` // ascending: the segments the change reaches
	// Through lists the identities of Segments in run order.
	Through []string `json:",omitempty"`
	// Direct reports that the test's own source changed, so no segment
	// match can stand in for it (MCJ-V0-006).
	Direct bool     `json:",omitempty"`
	Units  []string `json:",omitempty"` // units of the reached bound segments
}

// Join is the MCJ-V0-004 join of one change: each changed block to the
// segments whose reach holds it, and each segment to its test and units.
type Join struct {
	Fallback string `json:",omitempty"` // the change kept the static plan
	Tests    []TestReach
	Units    []string   `json:",omitempty"` // affected functional units, sorted
	Changed  []BlockRef `json:"-"`          // the changed blocks, sorted
	// Uncovered lists the changed blocks no recorded test executed: where
	// functionality links must fill the gap (MCJ-V0-021).
	Uncovered []BlockRef `json:"-"`
}

// Join joins an Effect to the index's segments and units. A fallback effect
// joins nothing and keeps its reason.
func (ix *Index) Join(e Effect) Join {
	if e.Fallback != "" {
		return Join{Fallback: e.Fallback}
	}
	j := Join{}
	reach := map[int32]map[int32]bool{}
	for b := range e.Blocks {
		j.Changed = append(j.Changed, b)
		blk := &ix.Files[b.File].Blocks[b.Block]
		if len(blk.Tests) == 0 {
			j.Uncovered = append(j.Uncovered, b)
			continue
		}
		for _, s := range ix.blockSegs(blk) {
			t := ix.segs[s].Test
			if reach[t] == nil {
				reach[t] = map[int32]bool{}
			}
			reach[t][s] = true
		}
	}
	sortRefs(j.Changed)
	sortRefs(j.Uncovered)
	ids := make([]int32, 0, len(e.Tests))
	for t := range e.Tests {
		ids = append(ids, t)
	}
	for t := range reach {
		if !e.Tests[t] {
			ids = append(ids, t) // cannot happen for an Analyze effect; kept, never dropped
		}
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	units := map[string]bool{}
	for _, t := range ids {
		r := TestReach{Test: t, ID: ix.TestID(t), Level: ix.level(t), Direct: e.Direct[t]}
		us := map[string]bool{}
		for s := range reach[t] {
			r.Segments = append(r.Segments, s)
		}
		sort.Slice(r.Segments, func(a, b int) bool { return r.Segments[a] < r.Segments[b] })
		for _, s := range r.Segments {
			seg := ix.segs[s]
			r.Through = append(r.Through, seg.Label())
			if seg.Bound {
				for _, u := range seg.Units {
					us[u] = true
				}
			}
		}
		r.Units = sortedKeys(us)
		for u := range us {
			units[u] = true
		}
		j.Tests = append(j.Tests, r)
	}
	j.Units = sortedKeys(units)
	return j
}

func sortRefs(refs []BlockRef) {
	sort.Slice(refs, func(a, b int) bool {
		return refs[a].File < refs[b].File || refs[a].File == refs[b].File && refs[a].Block < refs[b].Block
	})
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Plan reasons beyond the PackagePlan.Why values selected, stale and
// unrecorded (MCJ-V0-016).
const (
	ReasonWhole      = "whole"       // the package runs whole
	ReasonNotReached = "not-reached" // no rule keeps the test
	// ReasonRepresentative is the MCJ-V0-006 exclusion. In shadow it is
	// reported as WouldSkip and counted in ShadowCounts, never applied.
	ReasonRepresentative = "covered-by-representative"
)

// PlanTest is one head test's decision and why (MCJ-V0-016).
type PlanTest struct {
	Package, Name string
	Level         string
	Reason        string
	Through       []string   `json:",omitempty"` // segment identities the change reaches
	WouldSkip     *Exclusion `json:",omitempty"` // shadow only (MCJ-V0-017)
}

// PlanReport is the per-test plan with counts by level and reason
// (MCJ-V0-016, extending LPCV-V0-961).
type PlanReport struct {
	Tests        []PlanTest
	Counts       map[string]map[string]int // level -> reason -> tests
	ShadowCounts map[string]int            // level -> tests shadow would skip
	Shadow       ShadowReport
}

// Report lists every head test of the planned packages with its reason. It
// reads plans and never changes them: a shadow exclusion is reported as
// WouldSkip while the test keeps its reason.
func (ix *Index) Report(plans map[string]PackagePlan, headTests map[string][]string, j Join, sh ShadowReport) PlanReport {
	r := PlanReport{Counts: map[string]map[string]int{}, ShadowCounts: map[string]int{}, Shadow: sh}
	through := map[string][]string{}
	for _, t := range j.Tests {
		through[t.ID] = t.Through
	}
	skip := map[string]*Exclusion{}
	for i := range sh.Exclusions {
		skip[sh.Exclusions[i].Test] = &sh.Exclusions[i]
	}
	pkgs := make([]string, 0, len(plans))
	for p := range plans {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		p := plans[pkg]
		names := append([]string(nil), headTests[pkg]...)
		sort.Strings(names)
		for _, name := range names {
			pt := PlanTest{Package: pkg, Name: name, Level: LevelUnknown}
			if id, ok := ix.byName[pkg+"\x00"+name]; ok {
				pt.Level = ix.level(id)
			}
			switch why := p.Why[name]; {
			case why != "":
				pt.Reason = why
			case p.Mode == Whole:
				pt.Reason = ReasonWhole
			default:
				pt.Reason = ReasonNotReached
			}
			if pt.Reason == "selected" {
				id := pkg + "." + name
				pt.Through = through[id]
				if ex := skip[id]; ex != nil {
					pt.WouldSkip = ex
					r.ShadowCounts[pt.Level]++
				}
			}
			if r.Counts[pt.Level] == nil {
				r.Counts[pt.Level] = map[string]int{}
			}
			r.Counts[pt.Level][pt.Reason]++
			r.Tests = append(r.Tests, pt)
		}
	}
	return r
}
