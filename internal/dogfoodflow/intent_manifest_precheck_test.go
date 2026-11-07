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
// and finish-lane refuse a non-canonical --intents manifest before running any
// step, naming the line and, for an ordering fault no sort can repair, the code
// intent-manifest-unsorted and the sort -u command; a sorted manifest passes
// on. A manifest that only needs sorting is canonicalized (DWC-V0-7600).
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
		{"unsorted pre-flight", "docs/specs/b.md\ndocs/specs/a.md\n./x\n", true, []string{
			"dogfood-inputs-check: REFUSE intent-manifest-unsorted\n",
			"  input: --intents " + intents + "\n",
			`  line 2 "docs/specs/a.md" does not sort after line 1 "docs/specs/b.md"`,
			// DCW-V0-730 (V1-3930): the pre-flight names inputs-check.
			"  fix: LC_ALL=C sort -u -o " + intents + " " + intents + ", then rerun corvint dogfood inputs-check --base " + base +
				" --citations " + plan + " --intents " + intents + " --verify " + verify + " --outcome passed\n",
		}},
		// DWC-V0-7600 (V1-4788): an ordering fault with a line no sort can
		// repair is still refused as unsorted, with the sort fix.
		{"duplicate finish-lane", "docs/a.md\ndocs/a.md\n./x\n", false, []string{"dogfood-finish-lane: REFUSE intent-manifest-unsorted\n", "LC_ALL=C sort -u -o "}},
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

// TestDWCV07600FinishLaneCanonicalizesIntentManifest (V1-4788): an untracked
// --intents manifest whose only fault is order or duplicates is rewritten in
// place to its LC_ALL=C sort -u form and the run goes on to its steps, for both
// the pre-flight and the finish; a manifest the repository tracks, or one
// whose tracked state Git cannot read, is not rewritten, so its ordering
// refusal stands.
func TestDWCV07600FinishLaneCanonicalizesIntentManifest(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	for name, data := range map[string]string{"a.txt": "a\n", "docs/a.md": "# A\n\n## Requirements\n\n- `INT-V0-001`: a.\n", "docs/b.md": "# B\n\n## Requirements\n\n- `INT-V0-002`: b.\n", "intents.txt": "docs/b.md\ndocs/a.md\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	verify := filepath.Join(scratch, "verify.txt")
	for path, data := range map[string]string{plan: "1\ta.txt\t1:1\tspecification\n", verify: "go version\n"} {
		if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	run := func(intents string, preflight bool) (int, int, string) {
		t.Helper()
		calls := 0
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			calls++
			return 1
		}}
		var stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: preflight, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, VerifyFile: verify, Outcome: "passed"}}, io.Discard, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, calls, stderr.String()
	}
	for _, preflight := range []bool{true, false} {
		// Each run starts from the committed tree, without the report the
		// previous run left.
		testGit(t, root, "clean", "-fdxq")
		intents := filepath.Join(scratch, "intents")
		if err := os.WriteFile(intents, []byte("docs/b.md\ndocs/a.md\ndocs/b.md\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		_, calls, stderr := run(intents, preflight)
		got, err := os.ReadFile(intents)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "docs/a.md\ndocs/b.md\n" || calls == 0 || strings.Contains(stderr, "REFUSE intent-manifest-unsorted") ||
			!strings.Contains(stderr, "intents canonicalized: LC_ALL=C sort -u rewrote --intents "+intents+" from 3 to 2 lines") {
			t.Errorf("preflight=%v: manifest %q, step calls %d:\n%s", preflight, got, calls, stderr)
		}
		if info, err := os.Stat(intents); err != nil {
			t.Error(err)
		} else if info.Mode().Perm() != 0o640 {
			t.Errorf("preflight=%v: the rewrite changed the file mode to %v", preflight, info.Mode())
		}
	}
	tracked := filepath.Join(root, "intents.txt")
	code, calls, stderr := run(tracked, true)
	if got, _ := os.ReadFile(tracked); string(got) != "docs/b.md\ndocs/a.md\n" || code != 2 || calls != 0 ||
		!strings.Contains(stderr, "REFUSE intent-manifest-unsorted") || !strings.Contains(stderr, "LC_ALL=C sort -u -o ") {
		t.Errorf("tracked manifest: code=%d calls=%d manifest=%q:\n%s", code, calls, got, stderr)
	}
	// An untracked manifest inside the worktree is rewritten only while Git
	// can read the index: with the index corrupt, whether it is tracked is
	// unknown, so the file is kept and the ordering refusal stands.
	untracked := filepath.Join(root, "lane-intents.txt")
	if err := os.WriteFile(untracked, []byte("docs/b.md\ndocs/a.md\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(root, ".git", "index")
	saved, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, []byte("not an index\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	code, calls, stderr = run(untracked, true)
	if got, _ := os.ReadFile(untracked); string(got) != "docs/b.md\ndocs/a.md\n" || code != 2 || calls != 0 || !strings.Contains(stderr, "REFUSE intent-manifest-unsorted") ||
		!strings.Contains(stderr, "intents not canonicalized: whether the repository tracks --intents "+untracked+" is unknown (git ls-files --error-unmatch exited 128)") {
		t.Errorf("unreadable index: code=%d calls=%d manifest=%q:\n%s", code, calls, got, stderr)
	}
	if err := os.WriteFile(index, saved, 0o666); err != nil {
		t.Fatal(err)
	}
	_, _, stderr = run(untracked, true)
	if got, _ := os.ReadFile(untracked); string(got) != "docs/a.md\ndocs/b.md\n" || strings.Contains(stderr, "REFUSE intent-manifest-unsorted") ||
		!strings.Contains(stderr, "intents canonicalized: LC_ALL=C sort -u rewrote --intents "+untracked+" from 2 to 2 lines") {
		t.Errorf("untracked manifest in the worktree: manifest %q:\n%s", got, stderr)
	}
}
