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
	if !strings.HasPrefix(first, affectedHeadlinePrefix+"scope=BOUNDED dirty=1 selected=5 tests=3 drivers=2 excluded=0 unknown=0 revision=") {
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

// SLF-V1-008 on a maximum-length headline: a full revision, a full base and
// long unknown-reason counts push the first line past the 200-byte bound
// (AFP-V0-3642), and the cut still leaves both counts, because they follow the
// summary line's selected= total rather than the revision and base.
func TestAffectedHeadlineCountsSurviveTheLineBound_SLFV1008(t *testing.T) {
	receipt := affectedCompactReceipt{Revision: strings.Repeat("a", 40), Range: affectedRange{Base: strings.Repeat("b", 40)}}
	receipt.Plan.Selected = []affectedCompactSelection{
		{UnitID: "go:example.test/a"},
		{UnitID: "driver:make:Makefile:check", Kind: affected.SelectionKindDriverTarget},
	}
	line := "scope=UNKNOWN dirty=12 selected=2 excluded=1234 unknown=40"
	for _, reason := range []string{"go:build-constraint-variants", "shell:foreign-invocation-unmodelled", "UNSELECTED_BINARY_CONSUMER", "typescript:dynamic-import"} {
		line += " " + reason + "=10"
	}
	receipt.Summary = affectedSummary{Line: line, Scope: affected.ScopeUnknown, Selected: 2, Unknown: 40}
	for index := range 40 {
		receipt.Plan.Unknown = append(receipt.Plan.Unknown, affected.Unknown{Reason: "go:build-constraint-variants", Detail: strings.Repeat("d", index+150)})
	}
	first, _, _ := strings.Cut(renderAffectedHeadline(receipt), "\n")
	if len(first) != affectedHeadlineMaxLineBytes || !strings.HasSuffix(first, "...") {
		t.Fatalf("the fixture line was not cut: %d bytes %q", len(first), first)
	}
	if !strings.Contains(first, " selected=2 tests=1 drivers=1 ") {
		t.Fatalf("the cut removed the counts: %q", first)
	}
}
