package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-448: an aggregate local-outcome receipt left by an earlier commit's
// run (a sealed aggregate stage) is bound to another target, so the next
// change at a new HEAD runs; a receipt for HEAD that no report binds still
// refuses, naming the marker file and the move-aside recovery.
func TestDCWV0448StaleAggregateReceiptForAnotherTargetIsIgnored(t *testing.T) {
	root, base, expected, receipt := overLimitFixture(t)
	gitDir := filepath.Join(root, ".git")
	marker := filepath.Join(gitDir, "corvint", "local-outcome.json")
	for _, dir := range []string{filepath.Dir(marker), filepath.Join(root, ".corvint")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(marker, receipt, 0o600); err != nil {
		t.Fatal(err)
	}
	// The pass after the aggregate stage wrote a plain report that binds no
	// local outcome (a bind pass never records one).
	if err := os.WriteFile(filepath.Join(root, ".corvint", "dogfood-report.json"), []byte("{\"profile\":\"corvint-dogfood-change/0\",\"localOutcomeEvidenceSha256\":null}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".corvint/dogfood-report.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	change := func() string {
		stderr := new(bytes.Buffer)
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int { return 1 }}
		_, _ = Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps}, stderr)
		return stderr.String()
	}
	// HEAD is the receipt's target: the unbound receipt stays a marker and the
	// refusal names it with a recovery.
	got := change()
	if !strings.Contains(got, "REFUSE aggregate-enrollment-required") || !strings.Contains(got, "marker: "+marker) || !strings.Contains(got, "fix: mv "+marker+" "+marker+".stale") {
		t.Fatalf("same-target receipt refusal does not name the file and recovery:\n%s", got)
	}
	// A later commit (the next stage) leaves the receipt bound to the earlier
	// target: it is stale evidence, not this change's enrollment.
	testGit(t, root, "add", ".gitignore")
	testGit(t, root, "commit", "-qm", "next stage")
	if head := testGit(t, root, "rev-parse", "HEAD"); head == expected.Target {
		t.Fatal("fixture did not move HEAD")
	}
	if got := change(); strings.Contains(got, "aggregate-enrollment-required") {
		t.Fatalf("stale receipt for another target refused the next stage:\n%s", got)
	}
	// dogfood check keeps the target-free guard (DCW-V0-374).
	if present, err := AggregateMarkerPresent(root, gitDir); err != nil || !present {
		t.Fatalf("AggregateMarkerPresent=%v err=%v", present, err)
	}
}
