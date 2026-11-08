// SPDX-License-Identifier: Apache-2.0
package runnerselection

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
)

// LPCV-V0-952, RSP-V0-012: only whole coverage lines can prove that a
// line-only edit touches no uninstrumented code. This is syntax-independent.
func TestLPCVV0952PartialCoverageLines(t *testing.T) {
	for _, tc := range []struct {
		name  string
		block testevidence.Range
	}{
		// case 1: return 1 -- the return is unexecuted, but the test ran default.
		{"case-expression-before-return", testevidence.Range{Start: 8, StartColumn: 9, End: 8, EndColumn: 17}},
		{"unknown-prefix-construct", testevidence.Range{Start: 8, StartColumn: 4, End: 9, EndColumn: 1}},
		{"unknown-suffix-construct", testevidence.Range{Start: 5, StartColumn: 1, End: 8, EndColumn: 4}},
		{"gap-between-blocks", testevidence.Range{Start: 8, StartColumn: 1, End: 8, EndColumn: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixture()
			source(&in)
			record(&in).Executed.Items[0].Unexecuted = []testevidence.Range{tc.block}
			if tc.name == "gap-between-blocks" {
				record(&in).Executed.Items[0].Unexecuted = append(record(&in).Executed.Items[0].Unexecuted,
					testevidence.Range{Start: 8, StartColumn: 6, End: 9, EndColumn: 1})
			}
			selected(t, in, "rule:b")
		})
	}
	in := fixture()
	source(&in)
	// Positive control: the whole changed line is inside an unexecuted block.
	record(&in).Executed.Items[0].Unexecuted = []testevidence.Range{{Start: 8, StartColumn: 1, End: 9, EndColumn: 1}}
	excluded(t, in)
}

// RSP-V0-012, LPCV-V0-953: boundaries never certify body-only replacement
// hunks, even when the body itself has no executed blocks.
func TestRSPV0012BodyBoundaryReplacements(t *testing.T) {
	for _, tc := range []struct {
		name string
		body testevidence.Range
		line int32
	}{
		{"uncalled-signature", testevidence.Range{Start: 5, StartColumn: 28, End: 10, EndColumn: 2}, 5},
		{"closing-boundary", testevidence.Range{Start: 5, StartColumn: 28, End: 10, EndColumn: 2}, 10},
		{"unknown-opening-form", testevidence.Range{Start: 5, StartColumn: 1, End: 10, EndColumn: 1}, 5},
		{"unknown-trailing-form", testevidence.Range{Start: 5, StartColumn: 1, End: 10, EndColumn: 9}, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixture()
			source(&in)
			r := record(&in)
			r.Regions.Items[0].Bodies[0].Range = tc.body
			r.Loaded.Items = []testevidence.Blob{r.Executed.Items[0].Blob}
			in.Changes[0].Hunks = []Hunk{{Start: tc.line, End: tc.line}}
			p := selected(t, in, "rule:c")
			if p.Changes[0].Class != DeclarationEdit {
				t.Fatalf("boundary certified body-only: %+v", p.Changes)
			}
		})
	}
}

// RSP-V0-055: the closed prerequisites are agreeing collections or an explicit
// admitted single-collection qualification; unknown quality cannot exclude.
func TestRSPV0055CollectionReadiness(t *testing.T) {
	in := fixture()
	in.Runners[0].Qualification.SingleCollectionQualified = false
	selected(t, in, "collection-count-insufficient")
	in.Runners[0].Qualification.SingleCollectionQualified = true
	excluded(t, in)
	in.Runners[0].Qualification.Admitted = false
	full(t, in, "qualification-not-admitted")

	in = fixture()
	in.Runners[0].Qualification.SingleCollectionQualified = false
	a := *record(&in)
	b := a
	b.Sources = append([]testevidence.Producer{}, a.Sources...)
	b.Sources[0].Run = "run-2"
	u, err := testevidence.Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	*record(&in) = u
	excluded(t, in)
	b.Reads.Items = []testevidence.Blob{blob(&in, "unrelated-template")}
	u, err = testevidence.Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	*record(&in) = u
	selected(t, in, "nondeterministic")
	in.Runners[0].Qualification.SingleCollectionQualified = true
	selected(t, in, "nondeterministic")

	in = fixture()
	record(&in).Reads.State = testevidence.Incomplete
	record(&in).Reads.Reason = "unknown-observation"
	selected(t, in, "component-missing:reads")
}
