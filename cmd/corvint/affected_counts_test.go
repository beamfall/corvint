// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// SLF-V1-008 "Driver units excluded from the test count": a plan with three
// selected test units and two driver units prints tests=3 drivers=2, keeps
// the frozen summary line's selected= total, and lists no driver unit.
func TestAffectedHeadlineCountsTestsAndDriversApart_SLFV1008(t *testing.T) {
	receipt := affectedCompactReceipt{Revision: strings.Repeat("a", 40)}
	for _, id := range []string{"go:example.test/a", "go:example.test/b", "typescript:vitest:src/a.test.ts"} {
		receipt.Plan.Selected = append(receipt.Plan.Selected, affectedCompactSelection{UnitID: id})
	}
	receipt.Plan.Selected = append(receipt.Plan.Selected,
		affectedCompactSelection{UnitID: "driver:make:Makefile:check", Kind: affected.SelectionKindDriverTarget},
		affectedCompactSelection{UnitID: "driver:workflow:.github/workflows/ci.yml:test", Kind: affected.SelectionKindCIJob})
	receipt.Summary = affectedSummary{Line: "scope=BOUNDED dirty=1 selected=5 excluded=0 unknown=0", Scope: affected.ScopeBounded, Selected: 5}
	headline := renderAffectedHeadline(receipt)
	first, _, _ := strings.Cut(headline, "\n")
	if !strings.HasPrefix(first, affectedHeadlinePrefix+"scope=BOUNDED dirty=1 selected=5 ") || !strings.HasSuffix(first, " tests=3 drivers=2") {
		t.Fatalf("headline first line = %q", first)
	}
	if strings.Contains(headline, "driver:") {
		t.Fatalf("headline lists a driver unit:\n%s", headline)
	}

	plan := affected.Plan{Selected: []affected.Selection{
		{UnitID: "go:example.test/a"}, {UnitID: "go:example.test/b"}, {UnitID: "python:tests.test_x"},
		{UnitID: "driver:make:Makefile:check", Kind: affected.SelectionKindDriverTarget},
		{UnitID: "driver:workflow:.github/workflows/ci.yml:test"},
	}}
	if tests, drivers := plan.SelectionCounts(); tests != 3 || drivers != 2 {
		t.Fatalf("SelectionCounts = %d, %d", tests, drivers)
	}
}
