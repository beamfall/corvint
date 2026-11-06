package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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
		" more)\n  full plan: rerun with --full for the JSON argv\n",
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

func TestDryRunCIKeepsPinnedJSONDefault(t *testing.T) {
	t.Parallel()
	m := sessiontest.Manifest{Scope: "BOUNDED", Placement: "universe", ShardCount: 1}
	var out, errout bytes.Buffer
	if code := testDryRun(m, "sha256:fixture", sessiontest.Declared{}, t.TempDir(), testOptions{profile: "ci", dryRun: true}, &out, &errout); code != 0 {
		t.Fatalf("exit=%d %s", code, &errout)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("CI lost the pinned JSON wire: %s", &out)
	}
}

// STR-V0-7313: the context bound includes stderr's whole-package advice.
func TestDryRunSummaryBoundsBothOutputStreams(t *testing.T) {
	t.Parallel()
	m := sessiontest.Manifest{Scope: "FULL", Placement: "universe", ShardCount: 1}
	for i := 0; i < 400; i++ {
		m.Units = append(m.Units, sessiontest.Unit{Adapter: "go", Slice: "whole", Unit: fmt.Sprintf("go:example.test/m/pkg%03d", i), Reason: fmt.Sprintf("whole: DIRECT_SOURCE_CHANGE via pkg%03d/source.go", i)})
	}
	var out, errout bytes.Buffer
	if code := testDryRun(m, "sha256:fixture", sessiontest.Declared{}, t.TempDir(), testOptions{profile: "local", dryRun: true}, &out, &errout); code != 0 {
		t.Fatalf("exit=%d %s", code, &errout)
	}
	if out.Len()+errout.Len() > 4096 {
		t.Fatalf("summary streams total %d bytes (stdout=%d stderr=%d)", out.Len()+errout.Len(), out.Len(), errout.Len())
	}
	if !strings.Contains(out.String()+errout.String(), "HEAVY_WHOLE_REASON") {
		t.Fatal("bounded summary lost the whole-package reason")
	}
}

// STR-V0-7313: exercise the command entry point so the stale-binary notice
// emitted before planning counts against the combined stdout/stderr budget.
func TestDryRunStaleBinarySharesSummaryBudget(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	files := map[string]string{
		"self.go":                   "package self\n\nfunc Self() int { return 2 }\n",
		sessiontest.DeclarationPath: `{"profile":"corvint-test-commands/0","go":{"env":{"GOTOOLCHAIN":"local"},"flags":["-count=1"]},"languages":{"shell":{"none":"no suite"},"typescript":{"none":"no suite"}}}` + "\n",
	}
	for i := 0; i < 100; i++ {
		prefix := fmt.Sprintf("package%03d%s/", i, strings.Repeat("x", 60))
		files[prefix+"source.go"] = "package fixture\n\nfunc Value() int { return 1 }\n"
		files[prefix+"source_test.go"] = "package fixture\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n"
	}
	commitSelfBinaryFiles(t, root, files)
	for _, flags := range [][]string{nil, {"--summary"}} {
		var out, errout bytes.Buffer
		args := append([]string{"--root", root, "test", "--base", built, "--dry-run"}, flags...)
		code := runContext(context.Background(), args, strings.NewReader(""), &out, &errout)
		if code != 0 {
			t.Fatalf("%v exit=%d %s%s", flags, code, &out, &errout)
		}
		if !strings.Contains(errout.String(), "corvint-test: DEGRADED self-binary-stale: ") || !strings.Contains(out.String(), "(+") || !strings.Contains(out.String(), "HEAVY_WHOLE_REASON") || !strings.Contains(out.String(), "full plan: rerun with --full") {
			t.Fatalf("%v missing notice or summary: %s%s", flags, &out, &errout)
		}
		if out.Len()+errout.Len() > testSummaryLimit {
			t.Fatalf("%v combined=%d stdout=%d stderr=%d", flags, out.Len()+errout.Len(), out.Len(), errout.Len())
		}
	}
}

// STR-V0-7313: valid multi-component paths can consume the summary budget
// before the fixed sections (and can even exceed it in the notice alone).
func TestDryRunLongStalePathsStayWithinSummaryBudget(t *testing.T) {
	root, built := selfBinaryRepository(t)
	stubSelfBuildInfo(t, selfBinaryInfo("example.test/self", built))
	for _, depth := range []int{4, 7} {
		files := map[string]string{
			sessiontest.DeclarationPath: `{"profile":"corvint-test-commands/0","go":{"env":{"GOTOOLCHAIN":"local"},"flags":["-count=1"]},"languages":{"shell":{"none":"no suite"},"typescript":{"none":"no suite"}}}` + "\n",
		}
		for i := range 3 {
			path := fmt.Sprintf("long%d/", i) + strings.Repeat(strings.Repeat("x", 240)+"/", depth) + "source.go"
			files[path] = "package fixture\n\nfunc Value() int { return 1 }\n"
		}
		commitSelfBinaryFiles(t, root, files)
		var out, errout bytes.Buffer
		code := runContext(context.Background(), []string{"--root", root, "test", "--base", built, "--dry-run"}, strings.NewReader(""), &out, &errout)
		if code != 0 {
			t.Fatalf("depth=%d exit=%d %s%s", depth, code, &out, &errout)
		}
		if out.Len()+errout.Len() > testSummaryLimit {
			t.Errorf("depth=%d combined=%d stdout=%d stderr=%d; limit=%d", depth, out.Len()+errout.Len(), out.Len(), errout.Len(), testSummaryLimit)
		}
		for _, want := range []string{"corvint-test: DEGRADED self-binary-stale:", "corvint-test: DRY-RUN", "HEAVY_WHOLE_REASON", "truncated"} {
			if !strings.Contains(out.String()+errout.String(), want) {
				t.Errorf("depth=%d missing %q", depth, want)
			}
		}
		for _, flags := range [][]string{{"--full"}, {"--format", "json"}} {
			out.Reset()
			errout.Reset()
			args := append([]string{"--root", root, "test", "--base", built, "--dry-run"}, flags...)
			code := runContext(context.Background(), args, strings.NewReader(""), &out, &errout)
			if code != 0 || !json.Valid(out.Bytes()) || strings.Contains(errout.String(), "truncated") || errout.Len() <= testSummaryLimit/2 {
				t.Errorf("depth=%d flags=%v exit=%d: full form lost JSON or original notice", depth, flags, code)
			}
		}
	}
}

// STR-V0-7313: every section must respect even a nearly exhausted budget.
func TestDryRunSummaryEverySectionHonorsBudget(t *testing.T) {
	m := sessiontest.Manifest{Scope: "FULL", Placement: "universe", ShardCount: 1,
		Reason: strings.Repeat("reason 界 ", 100), Incomplete: []string{"incomplete"},
		Units: []sessiontest.Unit{{Adapter: "go", Slice: "whole", Unit: "go:fixture", Reason: "whole cause"}}}
	for _, budget := range []int{-1, 0, 1, 30, 100, 300, 600, 4096} {
		got := testDryRunSummaryBudget(m, "sha256:fixture", nil, 0, budget)
		if len(got) > max(0, budget) {
			t.Errorf("budget=%d got %d bytes", budget, len(got))
		}
		if !utf8.ValidString(got) || budget >= 100 && !strings.HasSuffix(got, "\n") {
			t.Errorf("budget=%d invalid UTF-8 or partial summary line: %q", budget, got)
		}
		if budget >= 100 && budget <= 300 && !strings.Contains(got, "truncated") {
			t.Errorf("budget=%d missing explicit truncation marker: %q", budget, got)
		}
	}
}
