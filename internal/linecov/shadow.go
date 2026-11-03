package linecov

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ShadowMode is the only mode shared-step exclusion runs in until the
// MCJ-V0-017 bar is met and the owner accepts promotion: it is computed and
// reported, and every test still runs.
const ShadowMode = "shadow"

// ShadowReport is the MCJ-V0-005/006 analysis of one change. It names the
// tests the plan would exclude as covered-by-representative; nothing reads it
// to change a selection (MCJ-V0-017).
type ShadowReport struct {
	Mode       string
	Classes    []Class     `json:",omitempty"` // classes the change reached
	Exclusions []Exclusion `json:",omitempty"`
}

// Class is one equivalence class of segments (MCJ-V0-005).
type Class struct {
	Key      string // digest of identity, prior state, later identities and reach
	Identity string
	Reach    string     // digest of the block set
	Blocks   int        // size of the block set
	Files    []FileBlob // files of the block set at the index commit
	Members  []string   // tests owning a member segment, sorted
	// Representative is the selected member with the smallest recorded
	// elapsed time, ties by test identity (MCJ-V0-006). Empty when no
	// member is selected.
	Representative string `json:",omitempty"`
	Divergent      bool   `json:",omitempty"` // MCJ-V0-018: excludes nothing
}

// FileBlob pins one file of a reach to its blob.
type FileBlob struct{ Path, OID string }

// Exclusion is one test shadow would exclude, with the evidence MCJ-V0-016
// requires for each segment through which the change reaches it.
type Exclusion struct {
	Test string
	Via  []Via
}

// Via is one reached segment of an excluded test and its representative.
type Via struct {
	Segment        string // the shared segment identity
	Class          string // Class.Key
	Representative string
	Reach          string // block-set digest, equal for both segments by construction
	Blocks         int
}

// segReach is the lazily built reach of every segment, read only by the
// shadow analysis, so an index without bound segments never builds it.
type segReach struct {
	once   sync.Once
	blocks [][]BlockRef
}

func (ix *Index) reachOf(s int32) []BlockRef {
	ix.reach.once.Do(func() {
		ix.reach.blocks = make([][]BlockRef, len(ix.segs))
		for fi := range ix.Files {
			for bi := range ix.Files[fi].Blocks {
				for _, seg := range ix.blockSegs(&ix.Files[fi].Blocks[bi]) {
					ix.reach.blocks[seg] = append(ix.reach.blocks[seg], BlockRef{int32(fi), int32(bi)})
				}
			}
		}
	})
	return ix.reach.blocks[s]
}

// classKey returns the MCJ-V0-005 class of segment s, or "" when s can join
// none: unbound or empty identity (condition 1), an incomplete record
// (MCJ-V0-015), an order-sensitive suite record, a prior or later segment
// without a bound identity (conditions 3 and 4). The key covers the identity,
// the prior state, every later identity through the end of the test, and the
// exact block set (condition 2).
func (ix *Index) classKey(s int32) (key, reach string, blocks int) {
	seg := ix.segs[s]
	t := ix.Tests[seg.Test]
	if !seg.Bound || seg.ID == "" || t.Incomplete || t.Name == InOrder {
		return "", "", 0
	}
	first, end := ix.segFirst[seg.Test], ix.segFirst[seg.Test+1]
	ids := func(lo, hi int32) (string, bool) {
		var b strings.Builder
		for i := lo; i < hi; i++ {
			o := ix.segs[i]
			if !o.Bound || o.ID == "" {
				return "", false
			}
			b.WriteString(o.ID)
			b.WriteByte(0)
		}
		return b.String(), true
	}
	prior := "setup\x00" + seg.Prior
	if seg.Prior == "" {
		before, ok := ids(first, s)
		if !ok {
			return "", "", 0
		}
		prior = "seq\x00" + before
	}
	after, ok := ids(s+1, end)
	if !ok {
		return "", "", 0
	}
	rs := ix.reachOf(s)
	if len(rs) == 0 {
		return "", "", 0 // reached nothing: no block set to compare
	}
	h := sha256.New()
	for _, r := range rs {
		f := ix.Files[r.File]
		b := f.Blocks[r.Block]
		fmt.Fprintf(h, "%s:%d.%d,%d.%d\n", f.Path, b.Start, b.StartCol, b.End, b.EndCol)
	}
	reach = hex.EncodeToString(h.Sum(nil))
	k := sha256.Sum256([]byte(seg.ID + "\x00\x01" + prior + "\x01" + after + "\x01" + reach))
	return hex.EncodeToString(k[:]), reach, len(rs)
}

// Shadow computes the shared-step exclusions of a join. selected reports
// whether a test runs in the plan; nil means every reached test does. A test
// is reported only when the change reaches it through segments alone, its
// own source is unchanged, and every reached segment has an equivalent in a
// selected representative other than itself (MCJ-V0-006).
func (ix *Index) Shadow(j Join, selected func(test int32) bool) ShadowReport {
	r := ShadowReport{Mode: ShadowMode}
	if j.Fallback != "" {
		return r
	}
	if selected == nil {
		reached := map[int32]bool{}
		for _, t := range j.Tests {
			reached[t.Test] = true
		}
		selected = func(t int32) bool { return reached[t] }
	}
	// Candidate classes: those of the reached segments.
	type info struct {
		key, reach string
		blocks     int
	}
	keyOf := map[int32]info{}
	wanted := map[string]bool{} // identities whose classes are needed
	for _, t := range j.Tests {
		for _, s := range t.Segments {
			if id := ix.segs[s].ID; id != "" && ix.segs[s].Bound {
				wanted[id] = true
			}
		}
	}
	if len(wanted) == 0 {
		return r
	}
	classes := map[string]*Class{}
	members := map[string]map[int32]bool{}
	for s := range ix.segs {
		seg := ix.segs[s]
		if !wanted[seg.ID] {
			continue
		}
		k, reach, n := ix.classKey(int32(s))
		keyOf[int32(s)] = info{k, reach, n}
		if k == "" {
			continue
		}
		c := classes[k]
		if c == nil {
			c = &Class{Key: k, Identity: seg.ID, Reach: reach, Blocks: n, Divergent: ix.diverged[k]}
			seen := map[string]bool{}
			for _, br := range ix.reachOf(int32(s)) {
				f := ix.Files[br.File]
				if !seen[f.Path] {
					seen[f.Path] = true
					c.Files = append(c.Files, FileBlob{f.Path, f.OID})
				}
			}
			classes[k] = c
			members[k] = map[int32]bool{}
		}
		members[k][seg.Test] = true
	}
	rep := map[string]int32{}
	reachedClass := map[string]bool{}
	for _, t := range j.Tests {
		for _, s := range t.Segments {
			if k := keyOf[s].key; k != "" {
				reachedClass[k] = true
			}
		}
	}
	for k, c := range classes {
		if !reachedClass[k] {
			continue
		}
		best := int32(-1)
		for t := range members[k] {
			c.Members = append(c.Members, ix.TestID(t))
			if selected(t) && (best < 0 || ix.cheaper(t, best)) {
				best = t
			}
		}
		sort.Strings(c.Members)
		if best >= 0 {
			rep[k] = best
			c.Representative = ix.TestID(best)
		}
		r.Classes = append(r.Classes, *c)
	}
	sort.Slice(r.Classes, func(a, b int) bool { return r.Classes[a].Key < r.Classes[b].Key })
	for _, t := range j.Tests {
		if t.Direct || len(t.Segments) == 0 || !selected(t.Test) {
			continue
		}
		ex := Exclusion{Test: t.ID}
		ok := true
		for _, s := range t.Segments {
			in := keyOf[s]
			c := classes[in.key]
			by, has := rep[in.key]
			if in.key == "" || c == nil || c.Divergent || !has || by == t.Test {
				ok = false
				break
			}
			ex.Via = append(ex.Via, Via{Segment: ix.segs[s].ID, Class: in.key, Representative: ix.TestID(by), Reach: in.reach, Blocks: in.blocks})
		}
		if ok {
			r.Exclusions = append(r.Exclusions, ex)
		}
	}
	return r
}

// cheaper orders representatives: a recorded elapsed time before none, then
// the smaller time, then the test identity (MCJ-V0-006, LPCV-V0-022). Test
// indexes follow package and name order.
func (ix *Index) cheaper(a, b int32) bool {
	ea, eb := ix.Tests[a].Elapsed, ix.Tests[b].Elapsed
	switch {
	case (ea > 0) != (eb > 0):
		return ea > 0
	case ea != eb:
		return ea < eb
	}
	return a < b
}
