// SPDX-License-Identifier: Apache-2.0

package testduration

import (
	"math"
	"slices"
	"testing"
)

func ms(value int64) *int64 { return &value }

// PTD-V1-003, PTD-V1-007: a reported time is rounded to whole milliseconds; a report without one
// is NOT_MEASURED and not invalid; a negative, non-numeric, non-finite or above-24-hour one is
// NOT_MEASURED and invalid. Nothing is estimated.
func TestPTDV1007InvalidAndMissingValues(t *testing.T) {
	for _, c := range []struct {
		name    string
		got     Measure
		want    *int64
		source  string
		invalid bool
	}{
		{"jest 3 ms", FromMilliseconds(3, SourceJest), ms(3), SourceJest, false},
		{"vitest fraction rounds", FromMilliseconds(2.6, SourceVitest), ms(3), SourceVitest, false},
		{"go elapsed seconds", FromSeconds(0.004, SourceGo), ms(4), SourceGo, false},
		{"junit text", FromSecondsText(" 0.25 ", SourcePytest), ms(250), SourcePytest, false},
		{"zero is measured", FromMilliseconds(0, SourceRSpec), ms(0), SourceRSpec, false},
		{"24 hours is valid", FromMilliseconds(MaxMilliseconds, SourceGo), ms(MaxMilliseconds), SourceGo, false},
		{"negative", FromMilliseconds(-5, SourceJest), nil, NotMeasured, true},
		{"above 24 hours", FromSeconds(24*60*60+1, SourceGo), nil, NotMeasured, true},
		{"NaN", FromMilliseconds(math.NaN(), SourceJest), nil, NotMeasured, true},
		{"infinite seconds", FromSeconds(math.Inf(1), SourceGo), nil, NotMeasured, true},
		{"non-numeric text", FromSecondsText("fast", SourcePytest), nil, NotMeasured, true},
		{"missing text", FromSecondsText("", SourcePytest), nil, NotMeasured, false},
		{"missing", Missing(), nil, NotMeasured, false},
	} {
		if (c.got.Milliseconds == nil) != (c.want == nil) || (c.want != nil && *c.got.Milliseconds != *c.want) || c.got.Source != c.source || c.got.Invalid != c.invalid {
			t.Errorf("%s: %+v", c.name, c.got)
		}
	}
}

// PTD-V1-006: a retried test records each attempt and their sum; a missing attempt leaves the
// test NOT_MEASURED rather than summing part of it, and an invalid attempt makes it invalid.
func TestPTDV1006AttemptSum(t *testing.T) {
	sum, attempts := Attempts([]Measure{FromMilliseconds(900, SourcePlaywright), FromMilliseconds(700, SourcePlaywright)}, SourcePlaywright)
	if sum.Milliseconds == nil || *sum.Milliseconds != 1600 || sum.Source != SourcePlaywright || !slices.Equal(attempts, []int64{900, 700}) {
		t.Fatalf("retry %+v %v", sum, attempts)
	}
	if sum, attempts := Attempts([]Measure{FromMilliseconds(900, SourcePlaywright), Missing()}, SourcePlaywright); sum.Milliseconds != nil || sum.Source != NotMeasured || sum.Invalid || attempts != nil {
		t.Fatalf("missing attempt %+v %v", sum, attempts)
	}
	if sum, _ := Attempts([]Measure{FromMilliseconds(-1, SourcePlaywright)}, SourcePlaywright); !sum.Invalid {
		t.Fatalf("invalid attempt %+v", sum)
	}
	if sum, _ := Attempts(nil, SourcePlaywright); sum.Source != NotMeasured {
		t.Fatalf("no attempt %+v", sum)
	}
}

// PTD-V1-009, PTD-V1-005: the cost figure sums measured per-test durations only; one unmeasured
// test labels it incomplete with the selected and full-suite counts and no ratio; the median of
// figures is withheld when any is incomplete.
func TestPTDV1009CostFigureUsesOnlyMeasured(t *testing.T) {
	complete := Cost([]*int64{ms(100), ms(50)}, []*int64{ms(100), ms(50), ms(150), ms(300)})
	if complete.Incomplete || complete.Ratio == nil || *complete.Ratio != 0.25 || complete.SelectedMs != 150 || complete.FullMs != 600 || complete.Label != "complete" {
		t.Fatalf("complete figure %+v", complete)
	}
	partial := Cost([]*int64{ms(100)}, []*int64{ms(100), nil, ms(300)})
	if !partial.Incomplete || partial.Ratio != nil || partial.SelectedNotMeasured != 0 || partial.FullNotMeasured != 1 ||
		partial.Label != "incomplete: 0 selected and 1 full-suite tests NOT_MEASURED" || partial.FullMs != 400 {
		t.Fatalf("partial figure %+v", partial)
	}
	if median, ok := Median([]Figure{complete, Cost([]*int64{ms(1)}, []*int64{ms(2)})}); !ok || *median != 0.375 {
		t.Fatalf("median %v %v", median, ok)
	}
	if median, ok := Median([]Figure{complete, partial}); ok || median != nil {
		t.Fatalf("median over an incomplete figure %v %v", median, ok)
	}
	if empty := Cost(nil, nil); !empty.Incomplete {
		t.Fatalf("a figure with no measured full suite is complete: %+v", empty)
	}
}
