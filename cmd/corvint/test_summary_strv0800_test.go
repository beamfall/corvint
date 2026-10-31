package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestSTRV0800DryRunSummaryIsBounded pins V1-3382: a FULL dry run over
// hundreds of packages printed about 66 KB of JSON, while --dry-run --summary
// prints at most 4 KiB naming the scope, the fallback reason, the unit counts
// by adapter and slice, the incomplete entries and as many selected units as
// fit, then the rest as a count. --summary without --dry-run is refused.
func TestSTRV0800DryRunSummaryIsBounded(t *testing.T) {
	t.Parallel()
	m := sessiontest.Manifest{
		Scope: "FULL", Placement: "universe", ShardCount: 1,
		Reason:     "plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH .github/forward-rows/verdict.sh " + strings.Repeat("x", 600),
		Checks:     []string{"a", "b", "c", "d"},
		Incomplete: []string{"first incomplete entry", strings.Repeat("y", 900)},
	}
	for index := range 400 {
		slice := "whole"
		if index%10 == 0 {
			slice = "selected"
		}
		m.Units = append(m.Units, sessiontest.Unit{Adapter: "go", Slice: slice, Unit: fmt.Sprintf("go:github.com/Beamfall/corvint/internal/package%03d", index)})
	}
	m.Units = append(m.Units, sessiontest.Unit{Adapter: "declared", Slice: "whole", Unit: "declared:node"},
		sessiontest.Unit{Adapter: "go", Slice: "whole", Shard: 1, Unit: "go:other-shard"})
	got := testDryRunSummary(m, "sha256:abc", make([]sessiontest.Planned, 3), 0)
	if len(got) > testSummaryLimit {
		t.Fatalf("summary is %d bytes, over %d:\n%s", len(got), testSummaryLimit, got)
	}
	for _, want := range []string{
		"corvint-test: DRY-RUN scope=FULL placement=universe shard=0/1 manifest=sha256:abc\n",
		"  reason: plan.unknown entry outside the AFP-V0-028 allowlist: UNINDEXED_SOURCE_PATH .github/forward-rows/verdict.sh xxx",
		"...\n",
		"  units: 401; declared 1 (whole=1), go 400 (selected=40 whole=360); invocations=3\n",
		"  checks still required: 4; no-suite languages: 0; incomplete: 2\n",
		"  incomplete: first incomplete entry yyy",
		"  selected: go:github.com/Beamfall/corvint/internal/package000 [selected] go:github.com/Beamfall/corvint/internal/package001 ",
		" more)\n  full plan: rerun without --summary for the JSON argv\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "other-shard") {
		t.Errorf("summary names another shard's unit:\n%s", got)
	}

	code, stdout, stderr := runTestPlanCLI(t, "test", "--base", strings.Repeat("a", 40), "--summary")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--summary applies only to --dry-run (STR-V0-800)") {
		t.Errorf("--summary without --dry-run: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
