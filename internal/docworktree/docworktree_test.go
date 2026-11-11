package docworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// DMF-V1-005: porcelain records count against HEAD when staged or unstaged and against the index
// only when unstaged; a rename's source record is skipped and the read predicate filters.
func TestDMFV1005ParseStatus(t *testing.T) {
	raw := []byte(" M docs/a.md\x00M  docs/b.md\x00R  docs/new.md\x00docs/old.md\x00 M src/x.go\x00")
	head := parse(raw, Head, Under("docs"))
	if head.State != Dirty || head.Changed != 3 || !slices.Equal(head.Paths, []string{"docs/a.md", "docs/b.md", "docs/new.md"}) {
		t.Fatalf("head %+v", head)
	}
	index := parse(raw, Index, nil)
	if index.State != Dirty || index.Changed != 2 || !slices.Equal(index.Paths, []string{"docs/a.md", "src/x.go"}) {
		t.Fatalf("index %+v", index)
	}
	if got := parse(raw, Head, Set("README.md")); got.State != Clean || got.Changed != 0 || got.Paths != nil {
		t.Fatalf("unread paths counted %+v", got)
	}
	var many []byte
	for _, name := range []string{"f", "e", "d", "c", "b", "a"} {
		many = append(many, []byte(" M "+name+"\x00")...)
	}
	if got := parse(many, Head, nil); got.Changed != 6 || !slices.Equal(got.Paths, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("bounded paths %+v", got)
	}
}

// DMF-V1-005: a real repository with one edited tracked page reports dirty with that page; an
// untracked file never counts, and a directory that is not a repository is unknown with a reason.
func TestDMFV1005ObserveRepository(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("docs/page.md", "one\n")
	git("add", "-A")
	git("-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "init")
	ctx := context.Background()
	if got := Observe(ctx, root, Head, nil); got.State != Clean {
		t.Fatalf("clean %+v", got)
	}
	write("docs/page.md", "two\n")
	write("docs/untracked.md", "new\n")
	if got := Observe(ctx, root, Head, Under("docs")); got.State != Dirty || got.Changed != 1 || !slices.Equal(got.Paths, []string{"docs/page.md"}) {
		t.Fatalf("dirty %+v", got)
	}
	git("add", "docs/page.md")
	if got := Observe(ctx, root, Index, nil); got.State != Clean {
		t.Fatalf("staged edit counted against the index %+v", got)
	}
	if got := Observe(ctx, root, Head, nil); got.State != Dirty {
		t.Fatalf("staged edit not counted against HEAD %+v", got)
	}
	if got := Observe(ctx, t.TempDir(), Head, nil); got.State != Unknown || got.Reason == "" {
		t.Fatalf("not a repository %+v", got)
	}
}
