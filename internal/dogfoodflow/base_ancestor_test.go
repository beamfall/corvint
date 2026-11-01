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

// nonAncestorRepository commits base A, a mainline commit B on top of it, and
// a branch commit C on A, checked out: B is not an ancestor of HEAD C, and
// their merge-base is A.
func nonAncestorRepository(t *testing.T) (root, a, b, plan string) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q", "-b", "main")
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "A")
	a = testGit(t, root, "rev-parse", "HEAD")
	write("main.txt", "mainline\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "B")
	b = testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "checkout", "-q", "-b", "lane", a)
	write("a.txt", "lane\n")
	testGit(t, root, "commit", "-q", "-am", "C")
	plan = filepath.Join(t.TempDir(), "plan.tsv")
	if err := os.WriteFile(plan, []byte("1\tunknown\tinsufficient-evidence\tnote\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	return root, a, b, plan
}

func wantBaseNotAncestor(t *testing.T, label, stderr, prefix, a, b string) {
	t.Helper()
	for _, want := range []string{
		prefix + ": REFUSE base-not-ancestor base=" + b + " merge-base=" + a + "\n",
		"a reseal would bind a revert of them; nothing was read or written (DCW-V0-946)",
		"  fix: rerun with BASE=" + a + ", the commit HEAD branched from, or rebase this branch onto " + b,
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("%s: refusal lacks %q:\n%s", label, want, stderr)
		}
	}
}

// TestDCWV0946ProposeRefusesNonAncestorBase (V1-3633): propose-citations, with
// and without --refresh, refuses a BASE HEAD does not contain, naming the
// merge-base, and prints no draft.
func TestDCWV0946ProposeRefusesNonAncestorBase(t *testing.T) {
	root, a, b, plan := nonAncestorRepository(t)
	for label, refresh := range map[string]string{"draft": "", "refresh": plan} {
		var stdout, stderr bytes.Buffer
		code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: b, Refresh: refresh}, &stdout, &stderr)
		if err != nil || code != 2 || stdout.Len() != 0 {
			t.Fatalf("%s: code=%d err=%v stdout=%q stderr:\n%s", label, code, err, &stdout, &stderr)
		}
		wantBaseNotAncestor(t, label, stderr.String(), "dogfood-propose-citations", a, b)
	}
	// The merge-base itself is admitted.
	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: a}, &stdout, &stderr); err != nil || code != 0 {
		t.Fatalf("merge-base BASE: code=%d err=%v stderr:\n%s", code, err, &stderr)
	}
}

// TestDCWV0946FinishLaneRefusesNonAncestorBase (V1-3633): finish-lane and its
// inputs-check pre-flight refuse a BASE HEAD does not contain, naming the
// merge-base, before any step runs.
func TestDCWV0946FinishLaneRefusesNonAncestorBase(t *testing.T) {
	root, a, b, plan := nonAncestorRepository(t)
	for _, preflight := range []bool{false, true} {
		calls := 0
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			calls++
			return 0
		}}
		var stdout, stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: preflight, Change: ChangeOptions{Root: root, Base: b, Steps: steps, Citations: plan, Verify: "go version", Outcome: "passed"}}, &stdout, &stderr)
		if err != nil || code != 2 || calls != 0 {
			t.Fatalf("preflight=%t: code=%d err=%v calls=%d stderr:\n%s", preflight, code, err, calls, &stderr)
		}
		prefix := "dogfood-finish-lane"
		if preflight {
			prefix = "dogfood-inputs-check"
		}
		wantBaseNotAncestor(t, prefix, stderr.String(), prefix, a, b)
	}
}
