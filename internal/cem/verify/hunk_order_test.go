package verify

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1750HunkOrder is CEM-V1-750: a map that names every patch hunk in
// patch order passes the coverage check, and the same hunks in another order
// refuse with invalid-field.
func TestCEMV1750HunkOrder(t *testing.T) {
	parsed := &patch.Patch{Hunks: []*patch.Hunk{
		{ID: "hunk:sha256:a", DisplayPath: "a.txt", OldRange: wire.Range{Start: 1, Count: 1}, NewRange: wire.Range{Start: 1, Count: 2}},
		{ID: "hunk:sha256:b", DisplayPath: "b.txt", OldRange: wire.Range{Start: 3, Count: 1}, NewRange: wire.Range{Start: 3, Count: 1}},
	}}
	mapHunk := func(h *patch.Hunk) wire.Hunk {
		return wire.Hunk{ID: h.ID, Path: h.DisplayPath, OldRange: h.OldRange, NewRange: h.NewRange}
	}
	inOrder := &wire.Map{Hunks: []wire.Hunk{mapHunk(parsed.Hunks[0]), mapHunk(parsed.Hunks[1])}}
	if err := checkHunkCoverage(inOrder, parsed); err != nil {
		t.Fatalf("patch-order map refused: %v", err)
	}
	reversed := &wire.Map{Hunks: []wire.Hunk{mapHunk(parsed.Hunks[1]), mapHunk(parsed.Hunks[0])}}
	err := checkHunkCoverage(reversed, parsed)
	if got := cemcode.CodeOf(err); got != cemcode.InvalidField {
		t.Fatalf("reversed map: code %q (%v), want %q", got, err, cemcode.InvalidField)
	}
}
