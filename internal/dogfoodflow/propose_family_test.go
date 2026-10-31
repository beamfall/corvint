package dogfoodflow

import (
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// TestDCWV0404FallbackFamilyCountsEveryOtherEdit: a hunk naming no ID takes
// the family its file's other edits name most often, counted across those
// edits, not each edit's winner unioned.
func TestDCWV0404FallbackFamilyCountsEveryOtherEdit(t *testing.T) {
	hunk := func(id, path string, lines ...string) *patch.Hunk {
		body := make([]patch.BodyLine, len(lines))
		for i, line := range lines {
			body[i] = patch.BodyLine{Prefix: '+', Payload: []byte(line)}
		}
		return &patch.Hunk{ID: id, DisplayPath: path, Body: body}
	}
	bare := hunk("h0", "spec.md", "no requirement here")
	p := &proposal{derived: map[string]*patch.Hunk{"h0": bare}}
	for _, other := range []*patch.Hunk{
		hunk("h1", "spec.md", "`ASD-V0-001` alone"),
		hunk("h2", "spec.md", "`DCW-V0-001` and `DCW-V0-002`"),
		hunk("h3", "spec.md", "`DCW-V0-003` and `DCW-V0-004`"),
		hunk("h4", "other.md", "`CEM-V1-001` `CEM-V1-002` `CEM-V1-003` `CEM-V1-004` `CEM-V1-005`"),
	} {
		p.derived[other.ID] = other
	}
	got := p.hunkFamilies(bare)
	if len(got) != 1 || !got["DCW-V0"] {
		t.Fatalf("hunkFamilies = %v, want only DCW-V0", got)
	}
}
