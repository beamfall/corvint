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

// TestASDV0841InputsCheckRefusesStaleCitationRows (V1-3924): a citation plan
// that still carries a row --refresh marked "[stale: re-keyed from ..., review]"
// is refused by inputs-check before any step, naming the row; the same plan
// with the mark removed passes on to the steps.
func TestASDV0841InputsCheckRefusesStaleCitationRows(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	verify := filepath.Join(scratch, "verify.txt")
	intents := filepath.Join(scratch, "intents")
	for path, data := range map[string]string{verify: "go test -count=1 -run TestA ./internal/a\n", intents: "#no-intent-declared\n"} {
		if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	run := func(content string) (int, int, string) {
		t.Helper()
		if err := os.WriteFile(plan, []byte(content), 0o666); err != nil {
			t.Fatal(err)
		}
		calls := 0
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			calls++
			return 1
		}}
		var stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: true, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}, io.Discard, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, calls, stderr.String()
	}
	note := "kept unknown: the test fixture has no governing line"
	code, calls, stderr := run("1\ta.txt\t1:1\tspecification\n2\tunknown\t-\t" + staleNotePrefix + "sha256:abc, review] " + note + "\n")
	for _, want := range []string{"dogfood-inputs-check: REFUSE citation-row-stale-review rows=1 plan=" + plan + "\n", "  stale: line 2 2\n", "delete its leading [stale: re-keyed from ID, review] mark"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if code != 2 || calls != 0 {
		t.Errorf("exit=%d step calls=%d; want 2 before any step:\n%s", code, calls, stderr)
	}
	_, calls, stderr = run("1\ta.txt\t1:1\tspecification\n2\tunknown\t-\t" + note + "\n")
	if calls == 0 || strings.Contains(stderr, "citation-row-stale-review") {
		t.Errorf("reviewed plan refused or ran no step (calls=%d):\n%s", calls, stderr)
	}
}
