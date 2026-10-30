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

// mergedForwardRepo builds a lane that merged its mainline forward past its
// base: main B, lane commit, main commit M, lane merge X of M, lane commit.
func mergedForwardRepo(t *testing.T) (root, base, mainline, merge string) {
	t.Helper()
	root = t.TempDir()
	commit := func(name, text string) string {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
		testGit(t, root, "add", name)
		testGit(t, root, "commit", "-q", "-m", name)
		return testGit(t, root, "rev-parse", "HEAD")
	}
	testGit(t, root, "init", "-q", "-b", "main")
	base = commit("a.txt", "base\n")
	testGit(t, root, "checkout", "-q", "-b", "lane")
	commit("lane.txt", "one\n")
	testGit(t, root, "checkout", "-q", "main")
	mainline = commit("main.txt", "main\n")
	testGit(t, root, "checkout", "-q", "lane")
	testGit(t, root, "merge", "-q", "--no-ff", "-m", "merge main", "main")
	merge = testGit(t, root, "rev-parse", "HEAD")
	commit("lane.txt", "two\n")
	return root, base, mainline, merge
}

// TestMergedForwardDCWV0171NamesTheMergedMainline covers V1-2171: a lane that
// merged its mainline past an unmoved BASE is named with the merged commit as
// the base to rerun with; a BASE at that commit, and a merged seal commit (a
// sealed lane or stacked predecessor), are not.
func TestMergedForwardDCWV0171NamesTheMergedMainline(t *testing.T) {
	root, base, mainline, merge := mergedForwardRepo(t)
	head := testGit(t, root, "rev-parse", "HEAD")
	var stderr bytes.Buffer
	f := &flow{ctx: context.Background(), prefix: "dogfood-change", root: root, stderr: &stderr, base: base, target: head}
	if gotMerge, gotParent := f.mergedForward(); gotMerge != merge || gotParent != mainline {
		t.Fatalf("mergedForward = %s %s, want %s %s", gotMerge, gotParent, merge, mainline)
	}
	f.noteMergedForward()
	for _, want := range []string{"dogfood-change: NOTE merged-forward merge=" + merge + " merged=" + mainline, "rerun dogfood-change, dogfood-check and dogfood-seal with BASE=" + mainline, "corvint.dogfood.anchor refs/remotes/origin/main", "DCW-V0-171"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("note %q lacks %q", stderr.String(), want)
		}
	}

	// A sealed branch from the merged mainline ends in its seal commit; merging
	// it is the stacked or batched route, not a merged-forward mainline.
	testGit(t, root, "checkout", "-q", "-b", "sealed", mainline)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte("{}\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".corvint/change.cem.json")
	testGit(t, root, "commit", "-q", "-m", "bind")
	bind := testGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint/changes"), 0o777); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bind+".cem.json")
	testGit(t, root, "commit", "-q", "-m", "seal")
	testGit(t, root, "checkout", "-q", "lane")
	testGit(t, root, "merge", "-q", "--no-ff", "-m", "merge sealed", "sealed")
	f.target = testGit(t, root, "rev-parse", "HEAD")
	if gotMerge, gotParent := f.mergedForward(); gotMerge != merge || gotParent != mainline {
		t.Fatalf("with a newer sealed merge: mergedForward = %s %s, want the mainline merge %s %s", gotMerge, gotParent, merge, mainline)
	}
	f.base = mainline
	stderr.Reset()
	f.noteMergedForward()
	if gotMerge, _ := f.mergedForward(); gotMerge != "" || stderr.Len() != 0 {
		t.Fatalf("BASE at the merged mainline: mergedForward = %q, note %q", gotMerge, stderr.String())
	}
}

// TestChangeDCWV0171NotesOrRefusesBeforeAnyStep covers V1-2171 through the
// public Change entry: without an anchor the merged-forward note is printed
// before the first step runs; with corvint.dogfood.anchor naming the mainline,
// the run refuses base-not-anchored naming the anchored BASE and runs no step.
func TestChangeDCWV0171NotesOrRefusesBeforeAnyStep(t *testing.T) {
	root, base, mainline, _ := mergedForwardRepo(t)
	var stderr bytes.Buffer
	calls, notedFirst := 0, false
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, _ []string, _, _ io.Writer) int {
		if calls == 0 {
			notedFirst = strings.Contains(stderr.String(), "NOTE merged-forward")
		}
		calls++
		return 1
	}}
	options := ChangeOptions{Root: root, Base: base, Steps: steps, Outcome: "passed"}
	if _, err := Change(context.Background(), options, &stderr); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || !notedFirst {
		t.Fatalf("unanchored: calls=%d noted before the first step=%v stderr %q", calls, notedFirst, stderr.String())
	}

	testGit(t, root, "config", "--local", "corvint.dogfood.anchor", "refs/heads/main")
	stderr.Reset()
	calls = 0
	code, err := Change(context.Background(), options, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	out := stderr.String()
	if code != 2 || calls != 0 || !strings.HasPrefix(out, "dogfood-change: REFUSE base-not-anchored\n") || !strings.Contains(out, "rerun with BASE="+mainline) || strings.Contains(out, "NOTE merged-forward") {
		t.Fatalf("anchored: exit %d calls %d stderr %q", code, calls, out)
	}
}
