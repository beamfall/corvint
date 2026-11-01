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

// TestDCWV0882InputsCheckRefusesUnsortedIntentsFirst (V1-3290; ASD-V0-840, V1-3922): inputs-check
// and finish-lane refuse an unsorted, duplicated or non-canonical --intents
// manifest before running any step, naming the line and, for an ordering
// fault, the code intent-manifest-unsorted and the sort -u command that fixes it; a sorted manifest passes on.
func TestDCWV0882InputsCheckRefusesUnsortedIntentsFirst(t *testing.T) {
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
	// The sorted manifest below names these intents; inputs-check refuses an
	// intent that is not a committed spec blob before any step (DCW-V0-945).
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", name+".md"), []byte("# Intent\n\n## Requirements\n\n- `INT-V0-001`: "+name+".\n"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", "docs")
	testGit(t, root, "commit", "-q", "-am", "change")
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	verify := filepath.Join(scratch, "verify.txt")
	intents := filepath.Join(scratch, "intents")
	for path, data := range map[string]string{plan: "1\ta.txt\t1:1\tspecification\n", verify: "go test -count=1 -run TestA ./internal/a\n"} {
		if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, manifest string
		preflight      bool
		want           []string
	}{
		{"unsorted pre-flight", "docs/specs/b.md\ndocs/specs/a.md\n", true, []string{
			"dogfood-inputs-check: REFUSE intent-manifest-unsorted\n",
			"  input: --intents " + intents + "\n",
			`  line 2 "docs/specs/a.md" does not sort after line 1 "docs/specs/b.md"`,
			"  fix: LC_ALL=C sort -u -o " + intents + " " + intents + ", then rerun corvint dogfood finish-lane --base " + base + "\n",
		}},
		{"duplicate finish-lane", "docs/a.md\ndocs/a.md\n", false, []string{"dogfood-finish-lane: REFUSE intent-manifest-unsorted\n", "LC_ALL=C sort -u -o "}},
		// ASD-V0-840 (V1-3922): the two committed intents of the passing
		// manifest below, swapped, are refused as unsorted, not as missing.
		{"swapped complete pre-flight", "docs/b.md\ndocs/a.md\n", true, []string{"dogfood-inputs-check: REFUSE intent-manifest-unsorted\n", `  line 2 "docs/a.md" does not sort after line 1 "docs/b.md"`}},
		{"non-canonical", "./docs/a.md\n", true, []string{`  line 1 "./docs/a.md" is not a canonical repository-relative path`, "  fix: " + intentScopeHint}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(intents, []byte(tc.manifest), 0o666); err != nil {
				t.Fatal(err)
			}
			calls := 0
			steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
				calls++
				return 1
			}}
			var stdout, stderr bytes.Buffer
			code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: tc.preflight, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}, &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(tc.want[0], "intent-manifest-unsorted") && strings.Contains(stderr.String(), "missing-intent-scope") {
				t.Errorf("an ordering fault is named missing-intent-scope:\n%s", stderr.String())
			}
			if code != 2 || calls != 0 {
				t.Errorf("exit=%d step calls=%d; want 2 before any step:\n%s", code, calls, stderr.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
		})
	}
	// A sorted manifest is not refused here; the run reaches its steps.
	if err := os.WriteFile(intents, []byte("docs/a.md\ndocs/b.md\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stderr bytes.Buffer
	if _, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: true, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}, io.Discard, &stderr); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || strings.Contains(stderr.String(), "REFUSE missing-intent-scope") {
		t.Errorf("sorted manifest refused or ran no step (calls=%d):\n%s", calls, stderr.String())
	}
}
