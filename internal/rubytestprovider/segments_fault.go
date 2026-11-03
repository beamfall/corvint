// SPDX-License-Identifier: Apache-2.0

package rubytestprovider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
)

// SegmentEffect maps a diff to its effect on a Ruby segment index
// (MCJ-V0-044). linecov.Analyze reads Go only, so a Ruby index needs its own
// line mapping. A changed line maps only when it is an executable line some
// recorded example ran; any other change keeps the static plan: an added or
// deleted file, a file the index does not record at the diff's old blob, or
// a line no example ran (a declaration, comment or load-time line, whose
// effect on callers line coverage cannot see).
func SegmentEffect(ix *linecov.Index, diffs []linecov.FileDiff) linecov.Effect {
	e := linecov.Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{},
		Blocks: map[linecov.BlockRef]bool{}, Direct: map[int32]bool{}}
	fail := func(format string, args ...any) linecov.Effect {
		return linecov.Effect{Fallback: fmt.Sprintf(format, args...)}
	}
	for _, d := range diffs {
		if d.OldOID == "" || d.NewOID == "" {
			return fail("added or deleted file %s", d.Path)
		}
		e.Touched[d.Path] = true
		for _, h := range d.Hunks {
			lo, hi := h.Start, h.End
			if hi < lo { // an insertion: either neighbouring line may run it
				hi = lo + 1
			}
			for l := lo; l <= hi; l++ {
				refs, ok := ix.BlocksAt(d.Path, d.OldOID, l, l)
				if !ok {
					return fail("index does not record %s at %s", d.Path, d.OldOID)
				}
				ran := false
				for _, b := range refs {
					blk := ix.Files[b.File].Blocks[b.Block]
					if len(blk.Tests) == 0 {
						continue
					}
					ran = true
					e.Blocks[b] = true
					for _, t := range blk.Tests {
						e.Tests[t] = true
					}
				}
				if !ran {
					return fail("line %s:%d ran in no recorded example", d.Path, l)
				}
			}
		}
	}
	return e
}

// SegmentFault runs one MCJ-V0-014 fault case over a Ruby segment index,
// with the verdict rules of linecov.Index.Fault and SegmentEffect in place of
// the Go analysis (MCJ-V0-044).
func SegmentFault(ix *linecov.Index, c linecov.FaultCase) linecov.FaultResult {
	res := linecov.FaultResult{Name: c.Name, Kind: c.Kind}
	e := SegmentEffect(ix, c.Diffs)
	if e.Fallback != "" {
		res.Detail = "fallback: " + e.Fallback
		return res
	}
	j := ix.Join(e)
	sh := ix.Shadow(j, nil)
	reached, ofKind := map[string]bool{}, map[string]bool{}
	for _, t := range j.Tests {
		reached[t.ID] = true
		for _, s := range t.Segments {
			if ix.Segment(s).Kind == c.Kind {
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
		skip[ex.Test] = strings.Join(keys(reps), ",")
	}
	var bad []string
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
		want := map[string]bool{}
		for _, t := range c.Excluded {
			want[t] = true
			if skip[t] != c.Representative {
				bad = append(bad, fmt.Sprintf("%s not excluded by %s (got %q)", t, c.Representative, skip[t]))
			}
		}
		for t := range skip {
			if !want[t] {
				bad = append(bad, t+" excluded unexpectedly")
			}
		}
	} else if len(skip) > 0 {
		names := map[string]bool{}
		for t := range skip {
			names[t] = true
		}
		bad = append(bad, fmt.Sprintf("unexpected exclusions %v", keys(names)))
	}
	sort.Strings(bad)
	res.Pass = len(bad) == 0
	res.Detail = strings.Join(bad, "; ")
	return res
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
