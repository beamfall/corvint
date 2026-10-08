package store

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func khRaceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitBeforePaths forwards cat-file input; before the first write after
// the commit question (the path lines) it runs race once, so a commit lands
// exactly between the commit answer and the path questions.
type commitBeforePaths struct {
	io.WriteCloser
	writes int
	once   sync.Once
	race   func()
}

func (w *commitBeforePaths) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		w.once.Do(w.race)
	}
	return w.WriteCloser.Write(p)
}

func (w *commitBeforePaths) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

// TestKHNV0015_CommitRaceResolvesOneCommit: a commit that moves HEAD after
// the pin's commit is resolved and before its paths are asked cannot mix two
// commits into one answer. The paths are asked as <commit oid>:<path>, so the
// pin returns the earlier commit with that commit's blob, although HEAD and
// HEAD:a.go have both moved by the time the answer is read (KHN-V0-001).
// The interleaving is forced through the askAtCommit seam, not by timing.
func TestKHNV0015_CommitRaceResolvesOneCommit(t *testing.T) {
	root := t.TempDir()
	khRaceGit(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	khRaceGit(t, root, "add", "a.go")
	khRaceGit(t, root, "commit", "-q", "-m", "base")
	oldHead, oldBlob := khRaceGit(t, root, "rev-parse", "HEAD"), khRaceGit(t, root, "rev-parse", "HEAD:a.go")

	c := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objecttype)")
	c.Dir = root
	c.Env = gitEnvironment()
	stdin, err := c.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	raced := false
	w := &commitBeforePaths{WriteCloser: stdin, race: func() {
		if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // moved\n"), 0o644); err != nil {
			t.Error(err)
			return
		}
		// This runs on askAtCommit's writer goroutine, so it reports with
		// t.Error rather than t.Fatal.
		commit := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-am", "moved")
		commit.Dir = root
		if out, err := commit.CombinedOutput(); err != nil {
			t.Errorf("race commit: %v\n%s", err, out)
			return
		}
		raced = true
	}}
	commit, objs, err := askAtCommit(w, bufio.NewReader(stdout), "HEAD", []string{"a.go"})
	if werr := c.Wait(); err == nil && werr != nil {
		err = werr
	}
	if err != nil {
		t.Fatal(err)
	}
	newHead, newBlob := khRaceGit(t, root, "rev-parse", "HEAD"), khRaceGit(t, root, "rev-parse", "HEAD:a.go")
	if !raced || newHead == oldHead || newBlob == oldBlob {
		t.Fatalf("the race commit did not land: raced=%v %s %s", raced, newHead, newBlob)
	}
	if commit != oldHead || len(objs) != 1 || objs[0].oid != oldBlob || objs[0].kind != "blob" {
		t.Fatalf("pin mixed commits: commit %s blob %+v; want %s %s", commit, objs, oldHead, oldBlob)
	}
}
