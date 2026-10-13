package dogfoodflow

import (
	"bytes"
	"context"
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
