package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// STR-V0-7313: the public default is bounded; JSON argv requires --full.
func TestDryRunDefaultsToBoundedSummary(t *testing.T) {
	root, base, _ := sessionFixture(t, "")
	var stdout, stderr bytes.Buffer
	manifest := filepath.Join(t.TempDir(), "plan.json")
	code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || !strings.HasPrefix(stdout.String(), "corvint-test: DRY-RUN") || stdout.Len() > 4096 {
		t.Fatalf("unbounded/default output: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--dry-run", "--full", "--manifest", manifest}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("full output: exit=%d %s %s", code, &stdout, &stderr)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("output projection changed manifest: %v", err)
	}
	for _, args := range [][]string{{"test", "--base", base, "--full"}, {"test", "--base", base, "--dry-run", "--summary", "--full"}, {"test", "--base", base, "--dry-run", "--pins", "--full"}} {
		if _, err := parseTestOptions(args[1:]); err == nil {
			t.Fatalf("invalid full form admitted: %v", args)
		}
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
