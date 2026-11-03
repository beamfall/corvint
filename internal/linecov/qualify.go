package linecov

import (
	"fmt"
	"sort"
	"strings"
)

// Divergence is one equivalence class a shadow miss marked (MCJ-V0-018). It
// stays in each next index while every file of its reach keeps its blob.
type Divergence struct {
	Class                    string
	Excluded, Representative string
	Change                   string
	Files                    []FileBlob
}

// Miss is an excluded test that failed while its representative passed
// (MCJ-V0-017).
type Miss struct {
	Class                    string
	Excluded, Representative string
	Change                   string
	Files                    []FileBlob
}

// ShadowOutcome is the miss detector's verdict on one shadow run.
type ShadowOutcome struct {
	Misses []Miss
	// Unverified lists excluded tests whose run, or whose representative's
	// run, left no result: the shadow proves nothing for them, and like a
	// miss this blocks promotion.
	Unverified []string
}

// DetectMisses compares a shadow report with the results of the run that
// executed every test anyway: results maps a test identity (package.Name)
// to whether it passed; an absent test did not run.
func DetectMisses(sh ShadowReport, change string, results map[string]bool) ShadowOutcome {
	var out ShadowOutcome
	files := map[string][]FileBlob{}
	for _, c := range sh.Classes {
		files[c.Key] = c.Files
	}
	unverified := map[string]bool{}
	for _, ex := range sh.Exclusions {
		passed, ran := results[ex.Test]
		if !ran {
			unverified[ex.Test] = true
			continue
		}
		seen := map[string]bool{}
		for _, v := range ex.Via {
			repPassed, repRan := results[v.Representative]
			switch {
			case !repRan:
				unverified[ex.Test] = true
			case !passed && repPassed && !seen[v.Class]:
				seen[v.Class] = true
				out.Misses = append(out.Misses, Miss{Class: v.Class, Excluded: ex.Test, Representative: v.Representative, Change: change, Files: files[v.Class]})
			}
		}
	}
	out.Unverified = sortedKeys(unverified)
	return out
}

// Divergence is the index entry the miss leaves.
func (m Miss) Divergence() Divergence {
	return Divergence{Class: m.Class, Excluded: m.Excluded, Representative: m.Representative, Change: m.Change, Files: m.Files}
}

// TicketDraft is the `bugs` ticket a miss files (MCJ-V0-018). Filing it is
// the caller's write to the task store; this package writes nothing.
type TicketDraft struct {
	Title  string
	Labels []string
	Body   string
}

// Ticket drafts the miss's ticket with both test identities and the change.
func (m Miss) Ticket() TicketDraft {
	return TicketDraft{
		Title:  fmt.Sprintf("Shared-step miss: %s failed while representative %s passed", m.Excluded, m.Representative),
		Labels: []string{"agent-memory", "bugs"},
		Body: fmt.Sprintf("MCJ-V0-018 shadow miss.\n\nExcluded test: %s\nRepresentative: %s\nChange: %s\nEquivalence class: %s\n\n"+
			"The class is divergent in the next index until a blob in its reach changes, and shared-step promotion is blocked until the cause is explained.\n",
			m.Excluded, m.Representative, m.Change, m.Class),
	}
}

// CarryDivergent sets next.Divergent to the entries of prev and the new
// misses whose every reach file still has the same blob in next; one whose
// file changed or left the index is dropped (MCJ-V0-018). A miss without a
// class or a reach file is refused: it could never be carried or released.
func CarryDivergent(next *Index, prev []Divergence, misses []Miss) error {
	all := append([]Divergence(nil), prev...)
	for _, m := range misses {
		if m.Class == "" || len(m.Files) == 0 {
			return fmt.Errorf("linecov: miss of %s by %s names no class or reach file", m.Excluded, m.Representative)
		}
		all = append(all, m.Divergence())
	}
	byClass := map[string]Divergence{}
	for _, d := range all {
		if _, dup := byClass[d.Class]; dup {
			continue
		}
		keep := true
		for _, f := range d.Files {
			i, ok := next.byPath[f.Path]
			if !ok || next.Files[i].OID != f.OID {
				keep = false
				break
			}
		}
		if keep {
			byClass[d.Class] = d
		}
	}
	next.Divergent = next.Divergent[:0]
	for _, d := range byClass {
		next.Divergent = append(next.Divergent, d)
	}
	sort.Slice(next.Divergent, func(a, b int) bool { return next.Divergent[a].Class < next.Divergent[b].Class })
	if len(next.Divergent) == 0 {
		next.Divergent = nil
	}
	next.reindex()
	return nil
}

// PromotionChanges is the MCJ-V0-017 bar: at least this many CI changes that
// touch a segment-covered block, with zero misses.
const PromotionChanges = 1000

// Qualification is the evidence a promotion from shadow needs (MCJ-V0-017).
type Qualification struct {
	Changes    int // shadow CI changes touching at least one segment-covered block
	Misses     int
	Unverified int
	Divergent  int                  // classes still divergent in the current index
	Faults     map[SegmentKind]bool // MCJ-V0-014 per-kind result
}

// Promotable reports whether shared-step exclusion may leave shadow, and why
// not. Even then it needs a recorded owner acceptance, which this does not
// stand in for.
func (q Qualification) Promotable() (bool, string) {
	var why []string
	if q.Misses > 0 {
		why = append(why, fmt.Sprintf("%d misses", q.Misses))
	}
	if q.Divergent > 0 {
		why = append(why, fmt.Sprintf("%d divergent classes", q.Divergent))
	}
	if q.Unverified > 0 {
		why = append(why, fmt.Sprintf("%d unverified exclusions", q.Unverified))
	}
	if q.Changes < PromotionChanges {
		why = append(why, fmt.Sprintf("%d of %d changes", q.Changes, PromotionChanges))
	}
	for _, k := range SegmentKinds {
		if !q.Faults[k] {
			why = append(why, fmt.Sprintf("no passing fault injection for %s segments", k))
		}
	}
	return len(why) == 0, strings.Join(why, "; ")
}

// FaultCase is one MCJ-V0-014 fault injection: a change, the tests that must
// stay selected and never be shadow-excluded, and, for a change reaching only
// equivalent segments, the representative and the tests it must exclude.
type FaultCase struct {
	Name           string
	Kind           SegmentKind
	Diffs          []FileDiff
	Keep           []string // package.Name
	Representative string
	Excluded       []string
}

// FaultResult is one case's outcome.
type FaultResult struct {
	Name   string
	Kind   SegmentKind
	Pass   bool
	Detail string `json:",omitempty"`
}

// Fault runs one case through Analyze, Join and Shadow with every reached
// test selected. read is the base-side file reader of Analyze. A case passes
// only when it names the tests it proves (Keep, or the representative and
// Excluded) and each is reached through a segment of the case's kind.
func (ix *Index) Fault(c FaultCase, read func(string) ([]byte, error)) FaultResult {
	res := FaultResult{Name: c.Name, Kind: c.Kind}
	e := ix.Analyze(c.Diffs, read)
	if e.Fallback != "" {
		res.Detail = "fallback: " + e.Fallback
		return res
	}
	j := ix.Join(e)
	sh := ix.Shadow(j, nil)
	reached := map[string]bool{}
	ofKind := map[string]bool{} // reached through a segment of the case's kind
	for _, t := range j.Tests {
		reached[t.ID] = true
		if len(t.Segments) == 0 && c.Kind == KindTest {
			ofKind[t.ID] = true // an unsegmented index: every test is whole
		}
		for _, s := range t.Segments {
			if ix.Segments[s].Kind == c.Kind {
				ofKind[t.ID] = true
			}
		}
	}
	skip := map[string]string{}
	for _, ex := range sh.Exclusions {
		reps := map[string]bool{}
		for _, v := range ex.Via {
			reps[v.Representative] = true
		}
		skip[ex.Test] = strings.Join(sortedKeys(reps), ",")
	}
	var bad []string
	// An empty case proves nothing, and a case proves its kind only through
	// segments of that kind.
	// An exclude case's Keep entries are extra controls of any kind.
	proves := c.Keep
	if c.Representative != "" {
		proves = append([]string{c.Representative}, c.Excluded...)
		if len(c.Excluded) == 0 {
			bad = append(bad, "exclude case names no test to exclude")
		}
	} else if len(c.Keep) == 0 {
		bad = append(bad, "keep case names no test to keep")
	}
	for _, t := range proves {
		if reached[t] && !ofKind[t] {
			bad = append(bad, fmt.Sprintf("%s not reached through a %s segment", t, c.Kind))
		}
	}
	for _, t := range c.Keep {
		if !reached[t] {
			bad = append(bad, t+" not selected")
		} else if skip[t] != "" {
			bad = append(bad, t+" excluded")
		}
	}
	if c.Representative != "" {
		if !reached[c.Representative] || skip[c.Representative] != "" {
			bad = append(bad, "representative "+c.Representative+" not kept")
		}
		for _, t := range c.Excluded {
			if skip[t] != c.Representative {
				bad = append(bad, fmt.Sprintf("%s not excluded by %s (got %q)", t, c.Representative, skip[t]))
			}
		}
		// All but the representative: no other reached test may run in its
		// place, and none may be excluded unexpectedly.
		want := map[string]bool{}
		for _, t := range c.Excluded {
			want[t] = true
		}
		for t := range skip {
			if !want[t] {
				bad = append(bad, t+" excluded unexpectedly")
			}
		}
	} else if len(skip) > 0 {
		bad = append(bad, fmt.Sprintf("unexpected exclusions %v", sortedKeys(boolSet(skip))))
	}
	sort.Strings(bad)
	res.Pass = len(bad) == 0
	res.Detail = strings.Join(bad, "; ")
	return res
}

func boolSet(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// FaultMatrix folds case results per kind: a kind passes only with at least
// one keep case and one exclude case, all passing (MCJ-V0-014).
func FaultMatrix(cases []FaultCase, results []FaultResult) map[SegmentKind]bool {
	keep, excl, fail := map[SegmentKind]bool{}, map[SegmentKind]bool{}, map[SegmentKind]bool{}
	for i, c := range cases {
		if i >= len(results) || !results[i].Pass {
			fail[c.Kind] = true
			continue
		}
		if c.Representative != "" {
			excl[c.Kind] = true
		} else {
			keep[c.Kind] = true
		}
	}
	out := map[SegmentKind]bool{}
	for _, k := range SegmentKinds {
		out[k] = keep[k] && excl[k] && !fail[k]
	}
	return out
}
