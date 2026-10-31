package dogfoodflow

import (
	"fmt"
	"strings"
	"testing"
)

// stagedPlan writes hunks selectors with rowsPerHunk citation rows each, then
// the given deferral rows.
func stagedPlan(hunks, rowsPerHunk, deferrals int) []byte {
	plan := new(strings.Builder)
	for h := 1; h <= hunks; h++ {
		for r := 0; r < rowsPerHunk; r++ {
			fmt.Fprintf(plan, "%d\tintent.md\t%d:%d\tspecification\n", h, r+1, r+1)
		}
	}
	for d := 0; d < deferrals; d++ {
		fmt.Fprintf(plan, "%s\tdocs/d%d.md\tdeferred-intent\n", deferredRowKind, d)
	}
	return []byte(plan.String())
}

// DCW-V0-446: finish-lane splits a plan of more than 256 rows into the staged
// route's plans; the last stage names as many hunks as fit with every deferral
// row, the earlier stages keep the plan's leading rows, and a plan of at most
// 256 rows stays on the ordinary path.
func TestFinishLaneDCWV0446PlanStagesSplitsOversizedPlan(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		hunks, rows, deferrals   int
		sizes                    []int
		firstLead, finalLeadHunk string
	}{
		{"300 single rows", 300, 1, 0, []int{44, 256}, "1\t", "45\t"},
		{"deferrals go last", 300, 1, 2, []int{46, 256}, "1\t", "47\t"},
		{"600 single rows", 600, 1, 0, []int{88, 256, 256}, "1\t", "345\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stages, detail := planStages(stagedPlan(tc.hunks, tc.rows, tc.deferrals))
			if detail != "" {
				t.Fatalf("refused: %s", detail)
			}
			sizes := []int{}
			for _, stage := range stages {
				sizes = append(sizes, len(stage))
			}
			if fmt.Sprint(sizes) != fmt.Sprint(tc.sizes) {
				t.Fatalf("stage sizes %v, want %v", sizes, tc.sizes)
			}
			final := stages[len(stages)-1]
			if !strings.HasPrefix(stages[0][0], tc.firstLead) || !strings.HasPrefix(final[0], tc.finalLeadHunk) {
				t.Fatalf("first stage starts %q, final stage starts %q", stages[0][0], final[0])
			}
			if got := stageRows(stages, true) - stageRows(stages, false); got != tc.deferrals || stageRows([][]string{final}, true)-stageRows([][]string{final}, false) != tc.deferrals {
				t.Fatalf("deferral rows not all in the final stage: %d", got)
			}
		})
	}
	if stages, detail := planStages(stagedPlan(256, 1, 0)); stages != nil || detail != "" {
		t.Fatalf("a 256-row plan was staged: %d %q", len(stages), detail)
	}
	// 216 two-row hunks then 144 one-row hunks: a greedy forward packing of
	// the earlier stages (256/64 rows) left only 232 hunks for its second
	// stage and refused a plan that binds; the longest run before the final
	// stage keeps more than 256 hunks unknown there.
	mixed := new(strings.Builder)
	for h := 1; h <= 360; h++ {
		for r := 0; r < 1+btoi(h <= 216); r++ {
			fmt.Fprintf(mixed, "%d\tintent.md\t%d:%d\tspecification\n", h, r+1, r+1)
		}
	}
	stages, detail := planStages([]byte(mixed.String()))
	if detail != "" {
		t.Fatalf("mixed plan refused: %s", detail)
	}
	sizes := []int{}
	for _, stage := range stages {
		sizes = append(sizes, len(stage))
	}
	if fmt.Sprint(sizes) != "[64 256 256]" || !strings.HasPrefix(stages[0][0], "1\t") {
		t.Fatalf("mixed plan stage sizes %v, first row %q", sizes, stages[0][0])
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DCW-V0-447: a plan no staging can bind is refused before any step runs: one
// hunk with more than 256 rows, or a plan whose hunks need more than 256 rows
// once at most 256 of them remain.
func TestFinishLaneDCWV0447PlanStagesRefusesUnstageablePlan(t *testing.T) {
	for _, tc := range []struct {
		name, plan, want string
	}{
		{"one hunk over the bound", string(stagedPlan(1, 257, 0)), `hunk selector "1" has 257 rows`},
		{"200 hunks of two rows", string(stagedPlan(200, 2, 0)), "stage 1 of 2 would cite a subset while only 200 hunks remain"},
		{"deferrals fill the last stage", string(stagedPlan(1, 1, 256)), "leave no room"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stages, detail := planStages([]byte(tc.plan))
			if stages != nil || !strings.Contains(detail, tc.want) {
				t.Fatalf("stages=%d detail=%q, want %q", len(stages), detail, tc.want)
			}
		})
	}
	hunks := []map[string]string{{"id": "h-one"}, {"id": "h-two"}}
	split := [][]string{{"1\tintent.md\t1:1\tspecification"}, {"h-one\tintent.md\t2:2\tspecification", "2\tintent.md\t1:1\tspecification"}}
	if got := stageHunkConflict(split, hunks); !strings.Contains(got, "hunk h-one is named in stages 1 and 2") {
		t.Fatalf("conflict %q", got)
	}
	if got := stageHunkConflict(split[1:], hunks); got != "" {
		t.Fatalf("one stage reported a conflict: %q", got)
	}
}
