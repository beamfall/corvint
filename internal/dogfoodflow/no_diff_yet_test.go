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

// TestChangeDCWV0383ReportsNoDiffYetAtChangeStart pins the change-start run
// (V1-3121): with no citation plan or outcome and HEAD's tree equal to BASE's,
// whether HEAD is BASE or an empty commit on it, dogfood change records the
// pre-change context, never runs cem prepare, writes an incomplete report whose
// later steps are NOT_PRODUCED no-diff-yet, names the inputs still owed and
// exits 0. A failing pre-change row still fails, and a run with a citation plan
// keeps every step.
func TestChangeDCWV0383ReportsNoDiffYetAtChangeStart(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	var calls []string
	queryStatus := 0
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
		calls = append(calls, strings.Join(args, " "))
		switch {
		case len(args) > 0 && args[0] == "query":
			return queryStatus
		case len(args) > 0 && (args[0] == "impact" || args[0] == "dogfood-observe"):
			return 0
		}
		return 1
	}}
	run := func(options ChangeOptions) (int, string, string) {
		t.Helper()
		calls = nil
		options.Root, options.Base, options.Steps = root, base, steps
		var stderr bytes.Buffer
		code, err := Change(context.Background(), options, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String(), string(report)
	}
	prepared := func() bool {
		for _, call := range calls {
			if strings.HasPrefix(call, "cem prepare") {
				return true
			}
		}
		return false
	}
	for _, label := range []string{"head-is-base", "empty-commit"} {
		if label == "empty-commit" {
			testGit(t, root, "commit", "-q", "--allow-empty", "-m", "empty")
		}
		code, stderr, report := run(ChangeOptions{})
		if code != 0 || prepared() {
			t.Fatalf("%s: code=%d prepared=%t stderr:\n%s", label, code, prepared(), stderr)
		}
		for _, want := range []string{
			"dogfood-change: PRECHANGE no-diff-yet base=" + base,
			"corvint dogfood propose-citations " + base + " > plan.tsv",
			"make dogfood-inputs-check BASE=" + base,
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("%s: stderr lacks %q:\n%s", label, want, stderr)
			}
		}
		if strings.Contains(stderr, "FAIL") {
			t.Fatalf("%s: no-diff-yet run reported a failure:\n%s", label, stderr)
		}
		// DCW-V0-921: each absent agent receipt is named with the command that writes it.
		for _, want := range []string{
			`agent receipt prechange-query absent: write it before the first edit: corvint query --task "THE CHANGE" --limit 1 > `,
			"agent receipt prechange-impact absent: write it before the first edit: corvint impact PATH... --limit 10 > ",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("%s: stderr lacks %q:\n%s", label, want, stderr)
			}
		}
		if !strings.Contains(report, `"complete": false`) || !strings.Contains(report, `{"name": "coordination-time-query", "status": "PRODUCED", "reason": "none"}`) {
			t.Fatalf("%s: report:\n%s", label, report)
		}
		for _, name := range noDiffYetSteps {
			if !strings.Contains(report, `{"name": "`+name+`", "status": "NOT_PRODUCED", "reason": "no-diff-yet"}`) {
				t.Fatalf("%s: report lacks %s no-diff-yet:\n%s", label, name, report)
			}
		}
	}
	// DCW-V0-921: a receipt the agent wrote is not named.
	if err := os.MkdirAll(filepath.Join(root, ".git", "corvint"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "corvint", "prechange-query.json"), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, stderr, _ := run(ChangeOptions{}); strings.Contains(stderr, "agent receipt prechange-query absent") ||
		!strings.Contains(stderr, "agent receipt prechange-impact absent") {
		t.Fatalf("a written prechange-query receipt was still named absent, or the impact one was not:\n%s", stderr)
	}
	queryStatus = 1
	if code, stderr, _ := run(ChangeOptions{}); code == 0 || !strings.Contains(stderr, "FAIL not-complete") {
		t.Fatalf("a failing pre-change query passed: code=%d\n%s", code, stderr)
	}
	queryStatus = 0
	plan := filepath.Join(t.TempDir(), "plan.tsv")
	if err := os.WriteFile(plan, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	if code, stderr, _ := run(ChangeOptions{Citations: plan}); code == 0 || !prepared() || strings.Contains(stderr, "no-diff-yet") {
		t.Fatalf("a run with a citation plan took the no-diff-yet path: code=%d prepared=%t\n%s", code, prepared(), stderr)
	}
}
