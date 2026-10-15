package dogfoodflow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-061: only a one-parent commit titled as the bind commit that changes
// nothing but the sidecar resumes at rebind; any other commit is the change.
func TestFinishLaneDCWV0061RecognizesOnlyTheBindCommit(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		t.Helper()
		testGit(t, root, "add", "-A")
		testGit(t, root, "commit", "-q", "-m", message)
		return testGit(t, root, "rev-parse", "HEAD")
	}
	testGit(t, root, "init", "-q")
	write("a.txt", "a\n")
	commit("base")
	write("a.txt", "b\n")
	change := commit(bindSubject)
	write(finishLaneSidecar, "{}\n")
	bind := commit(bindSubject)
	write(finishLaneSidecar, "{\"x\":1}\n")
	write("a.txt", "c\n")
	mixed := commit(bindSubject)
	write(finishLaneSidecar, "{\"x\":2}\n")
	retitled := commit("bind")

	var stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", root: root, stderr: &stderr}}
	for commit, want := range map[string]bool{change: false, bind: true, mixed: false, retitled: false} {
		if got := lane.isBindCommit(commit); got != want {
			t.Fatalf("isBindCommit(%s) = %v, want %v", commit, got, want)
		}
	}

	// Before the bind commit, a stopped run's sidecar is the one path that may
	// differ; at the bind commit nothing may.
	clean := func(resume bool) (int, string) {
		stderr.Reset()
		lane.base = change
		lane.target = bind
		code, err := boundary(context.Background(), func() int { lane.requireCleanWorktree(resume, false); return 0 }, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String()
	}
	write(finishLaneSidecar, "{\"x\":3}\n")
	if code, out := clean(false); code != 0 || out != "" {
		t.Fatalf("sidecar-only exit=%d stderr=%q", code, out)
	}
	if code, out := clean(true); code != 2 || out != "dogfood-finish-lane: REFUSE dirty-worktree\n  fix: git restore --source=HEAD --staged --worktree -- .corvint/change.cem.json .corvint/changes/"+bind+".cem.json, commit or remove every other change, then rerun corvint dogfood finish-lane --base "+change+"\n" {
		t.Fatalf("resume exit=%d stderr=%q", code, out)
	}
	write("notes.txt", "draft\n")
	if code, out := clean(false); code != 2 || !strings.HasPrefix(out, "dogfood-finish-lane: REFUSE dirty-worktree\n  fix: commit the change ") {
		t.Fatalf("dirty exit=%d stderr=%q", code, out)
	}
}

// DCW-V0-067 (V1-1397): a finish-lane run whose context ends during a pass
// names the interrupted step and the reasons kept so far instead of exiting
// with no output.
func TestFinishLaneDCWV0067NamesTheInterruptedStep(t *testing.T) {
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
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The first step command the prepare pass runs ends the context, as a
	// SIGINT or SIGTERM delivered to the CLI does.
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		cancel()
		return 1
	}}
	var stdout, stderr bytes.Buffer
	_, err := FinishLane(ctx, FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("FinishLane error = %v, want ErrInterrupted", err)
	}
	if want := "dogfood-finish-lane: prepare INTERRUPTED\ndogfood-finish-lane: NOT_PRODUCED rows=0\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q (stderr %q)", stdout.String(), want, stderr.String())
	}
}

// DCW-V0-068 (V1-1397): a sidecar removed while the bind pass cites it stops
// finish-lane at bind with the map-unavailable row and a fix line naming the
// map path, not only the CEM command's fixed "cannot read CEM map".
func TestFinishLaneDCWV0068NamesTheUnreadableMap(t *testing.T) {
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
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, finishLaneSidecar)
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, stderr io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Dir(sidecar), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(sidecar, []byte("{\n  \"hunks\": [\n    {\n      \"disposition\": \"unknown\",\n      \"id\": \"hunk:a\",\n      \"path\": \"a.txt\"\n    }\n  ]\n}\n"), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			// The operator removes the sidecar while the pass runs.
			if err := os.Remove(sidecar); err != nil {
				return 1
			}
			_, _ = io.WriteString(stderr, `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}`+"\n")
			return 2
		}
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
	if err != nil || code == 0 {
		t.Fatalf("FinishLane = (%d, %v), want a failing exit", code, err)
	}
	if !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare PASS hunks=1\n") || !strings.Contains(stdout.String(), "dogfood-finish-lane: bind FAIL exit=") ||
		!strings.Contains(stdout.String(), "  bind cem-cite NOT_PRODUCED map-unavailable\n") {
		t.Fatalf("stdout does not name the bind step and code:\n%s", stdout.String())
	}
	if want := "  cem-cite: map-unavailable\n    fix: the CEM map " + root + "/" + finishLaneSidecar + " could not be read ("; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
	}
}
