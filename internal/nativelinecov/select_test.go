// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// LPCV-V0-8207: an uninstrumented literal inside a multiline expression must
// retain tests whose executable coverage shares an opening or closing brace line.
func TestSelectionGapBraceLines(t *testing.T) {
	for _, c := range []struct {
		name  string
		body  Body
		line  int
		block linecov.Block
		want  bool
	}{
		{"opening brace", Body{Open: 1, Close: 3}, 2, linecov.Block{Start: 1, End: 1, EndCol: 2}, true},
		{"closing brace", Body{Open: 1, Close: 3}, 2, linecov.Block{Start: 3, End: 3, EndCol: 2}, true},
		{"overlaps opening brace", Body{Open: 3, Close: 5}, 4, linecov.Block{Start: 1, End: 3, EndCol: 2}, true},
		{"exclusive end before opening brace", Body{Open: 3, Close: 5}, 4, linecov.Block{Start: 1, End: 3, EndCol: 1}, false},
		{"after closing brace", Body{Open: 1, Close: 3}, 2, linecov.Block{Start: 4, End: 4, EndCol: 2}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.block.Tests = []int32{0}
			ix := &linecov.Index{Files: []linecov.File{{Path: "src", OID: "old", Blocks: []linecov.Block{c.block}}}}
			diff := linecov.FileDiff{Path: "src", OldOID: "old", NewOID: "new", Hunks: []linecov.Hunk{{Start: c.line, End: c.line}}}
			e := Analyze(ix, []linecov.FileDiff{diff},
				map[string]Bodies{"src": {OID: "old", Spans: []Body{c.body}}},
				map[string]EligibleChange{"src": {OldOID: "old", NewOID: "new", Hunks: diff.Hunks}})
			if e.Fallback != "" {
				t.Fatalf("unexpected fallback: %s", e.Fallback)
			}
			if e.Tests[0] != c.want || e.Blocks[linecov.BlockRef{File: 0, Block: 0}] != c.want {
				t.Fatalf("gap selection tests=%v blocks=%v, want retained=%v", e.Tests, e.Blocks, c.want)
			}
		})
	}
}
