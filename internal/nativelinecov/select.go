// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"fmt"
	"path"
	"slices"

	"github.com/Beamfall/corvint/internal/linecov"
)

// Body is a source provider's executable-body boundary. Changed lines must be
// inside the braces; coverage on brace lines may still belong to the body.
// Boundaries must come from parsing the exact base blob, never coverage hits:
// coverage alone cannot distinguish executable bodies from declarations.
type Body struct{ Open, Close int }
type Bodies struct {
	OID   string
	Spans []Body
}

// EligibleChange is a source provider's classification of an exact edit as
// body-local in both revisions. The provider must rule out macros (including
// expansion effects), declarations and structural changes using both source
// blobs; textual containment or coverage hits alone cannot establish this.
// Hunks is the complete classified diff, not a subset of its eligible lines.
// This is an in-process input, not a persisted coverage or wire format.
type EligibleChange struct {
	OldOID, NewOID string
	Hunks          []linecov.Hunk
}

// Analyze is experimental line-selection analysis given base-bound executable
// bodies and source-provider classifications bound to both revisions and hunks.
// A missing boundary or classification is a static fallback. This helper
// does not authorize exclusion: Build leaves all native packages unordered.
// LPCV-V0-8207.
func Analyze(ix *linecov.Index, diffs []linecov.FileDiff, bodies map[string]Bodies, eligible map[string]EligibleChange) linecov.Effect {
	e := linecov.Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}, Blocks: map[linecov.BlockRef]bool{}, Direct: map[int32]bool{}}
	fail := func(s string) linecov.Effect { return linecov.Effect{Fallback: s} }
	for _, d := range diffs {
		if d.OldOID == "" || d.NewOID == "" {
			return fail("added or deleted native source")
		}
		var f *linecov.File
		fi := 0
		for i := range ix.Files {
			if ix.Files[i].Path == d.Path {
				f = &ix.Files[i]
				fi = i
				break
			}
		}
		if f == nil || f.OID != d.OldOID {
			return fail("missing or stale native coverage")
		}
		bs, ok := bodies[d.Path]
		if !ok || bs.OID != d.OldOID || len(bs.Spans) == 0 {
			return fail("missing or stale executable body boundaries")
		}
		for _, b := range bs.Spans {
			if b.Open < 1 || b.Close <= b.Open {
				return fail("invalid executable body boundaries")
			}
		}
		change, ok := eligible[d.Path]
		if !ok || change.OldOID != d.OldOID || change.NewOID != d.NewOID || !slices.Equal(change.Hunks, d.Hunks) {
			return fail("missing or stale native edit classification")
		}
		e.Touched[path.Dir(d.Path)] = true
		if len(d.Hunks) == 0 {
			return fail("missing native source hunks")
		}
		for _, h := range d.Hunks {
			lo, hi := h.Start, h.End
			insert := hi < lo
			if lo < 1 || hi < lo-1 {
				return fail("invalid native hunk")
			}
			if insert {
				hi = lo + 1
			}
			body := Body{}
			for _, b := range bs.Spans {
				if (lo > b.Open && hi < b.Close || insert && lo >= b.Open && hi <= b.Close) && (body.Close == 0 || b.Close-b.Open < body.Close-body.Open) {
					body = b
				}
			}
			if body.Close == 0 {
				return fail(fmt.Sprintf("declaration-level edit %s:%d", d.Path, lo))
			}
			// For each changed executable line without an instrumented line entry,
			// retain all tests entering its containing body (case/else/loop gaps).
			gap := false
			for n := lo; n <= hi; n++ {
				covered := false
				for bi, b := range f.Blocks {
					end := int(b.End)
					if b.EndCol == 1 {
						end--
					}
					if n >= int(b.Start) && n <= end {
						covered = true
						e.Blocks[linecov.BlockRef{File: int32(fi), Block: int32(bi)}] = true
						for _, id := range b.Tests {
							e.Tests[id] = true
						}
					}
				}
				if !covered {
					gap = true
				}
			}
			if gap {
				for bi, b := range f.Blocks {
					end := int(b.End)
					if b.EndCol == 1 {
						end--
					}
					// Line-only boundaries cannot disown executable coverage
					// sharing a brace line, including a block starting before it.
					if int(b.Start) <= body.Close && end >= body.Open {
						e.Blocks[linecov.BlockRef{File: int32(fi), Block: int32(bi)}] = true
						for _, id := range b.Tests {
							e.Tests[id] = true
						}
					}
				}
			}
		}
	}
	return e
}
