package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// entryRun runs finish-lane on the preflight repository with a counting
// step runner and returns its exit code, stdout, stderr and step count.
func entryRun(t *testing.T, options FinishLaneOptions) (int, string, string, int) {
	t.Helper()
	calls := 0
	options.Change.Steps = Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), options, &stdout, &stderr)
	if err != nil {
		t.Fatalf("FinishLane: %v\n%s", err, &stderr)
	}
	return code, stdout.String(), stderr.String(), calls
}

// TestDCWV0730RerunHintNamesTheEntry (V1-3883, V1-3930): an unsorted intent
// manifest and an unquoted -run alternation are refused before any step, and
// the fix line reruns the command the operator ran with the same inputs:
// inputs-check with its flags, or the wrapper's make target with its variables;
// a finish-lane without an entry keeps its own hint.
func TestDCWV0730RerunHintNamesTheEntry(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	unsorted := writeInput(t, "docs/b.md\ndocs/a.md\n")
	sorted := writeInput(t, "#no-intent-declared\n")
	verify := writeInput(t, "go version\n")
	alternation := writeInput(t, "go test -count=1 -run TestA|TestB ./pre\n")
	vars := " CITATIONS=" + plan + " INTENTS=" + unsorted + " VERIFY=" + verify + " OUTCOME=passed"
	for _, tc := range []struct {
		name, entry, intents, verify, inline string
		preflight                            bool
		want                                 string
	}{
		{"inputs-check", "", unsorted, verify, "", true, ", then rerun corvint dogfood inputs-check --base " + base + " --citations " + plan + " --intents " + unsorted + " --verify " + verify + " --outcome passed\n"},
		{"make dogfood-inputs-check", "dogfood-inputs-check", unsorted, verify, "", true, ", then rerun make dogfood-inputs-check BASE=" + base + vars + "\n"},
		{"make dogfood-closeout", "dogfood-closeout", unsorted, verify, "", true, ", then rerun make dogfood-closeout BASE=" + base + vars + "\n"},
		{"make dogfood-finish", "dogfood-finish", unsorted, verify, "", false, ", then rerun make dogfood-finish BASE=" + base + vars + "\n"},
		{"inline verify", "dogfood-closeout", unsorted, "", "go version", true, ", then rerun make dogfood-closeout BASE=" + base + " CITATIONS=" + plan + " INTENTS=" + unsorted + " OUTCOME=passed with the same DOGFOOD_VERIFY\n"},
		{"finish-lane", "", unsorted, verify, "", false, ", then rerun corvint dogfood finish-lane --base " + base + "\n"},
		{"alternation closeout", "dogfood-closeout", sorted, alternation, "", true, "quote the -run alternation as one word: go test -count=1 -run 'TestA|TestB' ./pre (DCW-V0-403; quoting rule: docs/DOGFOOD.md Inputs, DOGFOOD_VERIFY_FILE), then rerun make dogfood-closeout BASE=" + base + " CITATIONS=" + plan + " INTENTS=" + sorted + " VERIFY=" + alternation + " OUTCOME=passed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr, calls := entryRun(t, FinishLaneOptions{Preflight: tc.preflight, Entry: tc.entry, Change: ChangeOptions{
				Root: root, Base: base, Citations: plan, IntentsFile: tc.intents, VerifyFile: tc.verify, Verify: tc.inline, Outcome: "passed"}})
			if code != 2 || calls != 0 || stdout != "" || !strings.Contains(stderr, tc.want) {
				t.Fatalf("code=%d steps=%d stdout=%q stderr=\n%s\nwant a line ending %q", code, calls, stdout, stderr, tc.want)
			}
			if tc.entry != "" && strings.Contains(stderr, "rerun corvint dogfood") {
				t.Fatalf("an entry run still names a corvint subcommand:\n%s", stderr)
			}
		})
	}
}

// TestDCWV0731StaticPreCheck (NEW-B2): the static pre-check refuses an input
// defect with the full pre-flight's refusal, runs no step, takes no operation
// lock and never refreshes the plan; over valid inputs it reports STATIC PASS.
func TestDCWV0731StaticPreCheck(t *testing.T) {
	root, base := preflightRepository(t, nil)
	plan := writeInput(t, "1\tpre.go\t3:3\timplementation\n2\tpre.go\t20:20\timplementation\n")
	verify := writeInput(t, "go version\n")
	unsorted := writeInput(t, "docs/b.md\ndocs/a.md\n")
	sorted := writeInput(t, "#no-intent-declared\n")
	options := func(intents string) FinishLaneOptions {
		return FinishLaneOptions{Preflight: true, Static: true, Entry: "dogfood-inputs-check", Change: ChangeOptions{
			Root: root, Base: base, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}
	}
	code, stdout, stderr, calls := entryRun(t, options(unsorted))
	if code != 2 || calls != 0 || stdout != "" || !strings.HasPrefix(stderr, "dogfood-inputs-check: REFUSE intent-manifest-unsorted\n") ||
		!strings.Contains(stderr, "then rerun make dogfood-inputs-check BASE="+base) {
		t.Fatalf("unsorted: code=%d steps=%d stdout=%q stderr=\n%s", code, calls, stdout, stderr)
	}
	before, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr, calls = entryRun(t, options(sorted))
	want := "dogfood-inputs-check: inputs-check STATIC PASS plan, verify, intents and outcome inputs; the full inputs-check still runs the prepare pass\n"
	if code != 0 || calls != 0 || stdout != want || stderr != "" {
		t.Fatalf("valid: code=%d steps=%d stdout=%q stderr=%q", code, calls, stdout, stderr)
	}
	after, err := os.ReadFile(plan)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("the static pre-check rewrote the plan: err=%v", err)
	}
	if _, err := os.Stat(root + "/.git/corvint"); !os.IsNotExist(err) {
		t.Fatalf("the static pre-check wrote under the Git directory: %v", err)
	}
}
