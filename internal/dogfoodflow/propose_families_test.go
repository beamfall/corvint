package dogfoodflow

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestProposeCitationsDWCV0752FamilyFallbacksMatchThePerHunkScan
// (DWC-V0-752): the file-wide and change-wide family fallbacks, computed once
// per change, equal the per-hunk scan they replace for every hunk, a hunk
// reached under its range key as well as its ID is counted once, and a change
// that names no requirement leaves both fallbacks empty.
func TestProposeCitationsDWCV0752FamilyFallbacksMatchThePerHunkScan(t *testing.T) {
	hunk := func(id, path string, lines ...string) *patch.Hunk {
		h := &patch.Hunk{ID: id, DisplayPath: path}
		for _, line := range lines {
			h.Body = append(h.Body, patch.BodyLine{Prefix: line[0], Payload: []byte(line[1:])})
		}
		return h
	}
	// perHunk is the scan before V1-2360: the same file's edits, else the
	// whole change's.
	perHunk := func(p *proposal, h *patch.Hunk) (map[string]bool, bool) {
		if edits, around := hunkText(h); len(dominantFamilies(edits)) > 0 || len(dominantFamilies(around)) > 0 {
			t.Fatalf("hunk %s names a family itself; the fallbacks are not reached", h.ID)
		}
		var file, all []byte
		for key, other := range p.derived {
			if key != other.ID {
				continue
			}
			edits, _ := hunkText(other)
			all = append(all, edits...)
			if other.DisplayPath == h.DisplayPath {
				file = append(file, edits...)
			}
		}
		if families := dominantFamilies(file); len(families) > 0 {
			return families, false
		}
		families := dominantFamilies(all)
		return families, len(families) > 0
	}
	a := hunk("h1", "a.go", "+// AAA-V0-001 AAA-V0-002", " context")
	aBare := hunk("h2", "a.go", "+plain", " plain")
	b := hunk("h3", "b.go", "-// BBB-V0-001", "+// BBB-V0-002 AAA-V0-003")
	c := hunk("h4", "c.go", "+nothing", " nothing")
	derived := map[string]*patch.Hunk{"h1": a, "h2": aBare, "h3": b, "h4": c, rangeKey("b.go", 1, 1): b}
	for _, tc := range []struct {
		name    string
		derived map[string]*patch.Hunk
		hunks   []*patch.Hunk
	}{
		{"change with requirements", derived, []*patch.Hunk{aBare, c}},
		{"change with none", map[string]*patch.Hunk{"h2": aBare, "h4": c}, []*patch.Hunk{aBare, c}},
	} {
		p := &proposal{derived: tc.derived}
		var first string
		for _, h := range tc.hunks {
			want, wantChange := perHunk(p, h)
			got, gotChange := p.hunkFamiliesScope(h)
			if !reflect.DeepEqual(got, want) || gotChange != wantChange {
				t.Fatalf("%s: hunk %s: got %v %v, want %v %v", tc.name, h.ID, got, gotChange, want, wantChange)
			}
			if p.fileFamilies == nil {
				t.Fatalf("%s: the fallbacks were not computed for the change", tc.name)
			}
			if first == "" {
				first = fmt.Sprintf("%p", p.fileFamilies)
			} else if now := fmt.Sprintf("%p", p.fileFamilies); now != first {
				t.Fatalf("%s: the fallbacks were computed again for hunk %s", tc.name, h.ID)
			}
		}
		if tc.name == "change with none" && (len(p.changeFamilies) != 0 || len(p.fileFamilies["a.go"]) != 0) {
			t.Fatalf("%s: fallbacks %v %v", tc.name, p.fileFamilies, p.changeFamilies)
		}
	}
	// A counted check of the aliased hunk: BBB-V0 is named twice and AAA-V0
	// three times in all, so the change-wide fallback is AAA-V0 only; counting
	// the range-key alias as well would tie them.
	p := &proposal{derived: derived}
	if got, change := p.hunkFamiliesScope(c); !reflect.DeepEqual(got, map[string]bool{"AAA-V0": true}) || !change {
		t.Fatalf("change-wide fallback %v %v", got, change)
	}
}
