package plansnapshot

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func fixture(t *testing.T) (string, Receipt) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Test")
	git(t, root, "config", "user.email", "test@example.invalid")
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0600)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0600)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "target")
	paths := []string{"a.go"}
	return root, Receipt{Schema: Schema, Base: base, Commit: git(t, root, "rev-parse", "HEAD"), Tree: git(t, root, "rev-parse", "HEAD^{tree}"), Paths: paths, Digest: PathDigest(paths)}
}

func TestSnapshotImmutableBytesAndCleanup(t *testing.T) {
	t.Run("AFP-V0-019 TestSnapshotImmutableBytesAndCleanup", func(t *testing.T) {
		root, r := fixture(t)
		os.WriteFile(filepath.Join(root, "a.go"), []byte("hostile worktree bytes"), 0600)
		os.WriteFile(filepath.Join(root, "untracked.go"), []byte("package extra"), 0600)
		dir, cleanup, err := r.Materialize(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		body, err := os.ReadFile(filepath.Join(dir, "a.go"))
		if err != nil || string(body) != "package a\n" {
			t.Fatalf("%s %v", body, err)
		}
		if _, err = os.Stat(filepath.Join(dir, "untracked.go")); !os.IsNotExist(err) {
			t.Fatal("untracked input admitted")
		}
		cleanup()
		if _, err = os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("scratch leaked")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, cleanup, err := r.Materialize(ctx, root); err == nil || cleanup != nil {
			t.Fatal("cancelled snapshot admitted")
		}
	})
}

func TestSnapshotRejectsIncompleteMismatchedAndStale(t *testing.T) {
	t.Run("AFP-V0-019 TestSnapshotRejectsIncompleteMismatchedAndStale", func(t *testing.T) {
		root, r := fixture(t)
		for name, change := range map[string]func(*Receipt){
			"missing": func(r *Receipt) { r.Tree = "" }, "tree": func(r *Receipt) { r.Tree = r.Commit },
			"digest":     func(r *Receipt) { r.Digest = strings.Repeat("0", 64) },
			"incomplete": func(r *Receipt) { r.Paths = []string{}; r.Digest = PathDigest(r.Paths) },
			"wrong-base": func(r *Receipt) { r.Base = r.Tree },
			"duplicate":  func(r *Receipt) { r.Paths = []string{"a.go", "a.go"}; r.Digest = PathDigest(r.Paths) },
			"escape":     func(r *Receipt) { r.Paths = []string{"../a.go"}; r.Digest = PathDigest(r.Paths) },
		} {
			t.Run(name, func(t *testing.T) {
				bad := r
				change(&bad)
				if bad.Validate(context.Background(), root) == nil {
					t.Fatal("admitted")
				}
			})
		}
		git(t, root, "commit", "--allow-empty", "-qm", "advance")
		if r.Validate(context.Background(), root) == nil {
			t.Fatal("stale snapshot admitted")
		}
	})
}

func TestSnapshotStrictWire(t *testing.T) {
	t.Run("AFP-V0-019 TestSnapshotStrictWire", func(t *testing.T) {
		_, r := fixture(t)
		raw, _ := json.Marshal(r)
		if _, err := Decode(raw); err != nil {
			t.Fatal(err)
		}
		for _, raw := range [][]byte{[]byte("null"), []byte("{}"), append([]byte(`{"overlayDigest":"x",`), raw[1:]...), append([]byte(`{"schema":"x",`), raw[1:]...), bytesRepeat(MaxBytes + 1)} {
			if _, err := Decode(raw); err == nil {
				t.Fatal("invalid wire admitted")
			}
		}
	})
}
func bytesRepeat(n int) []byte { return []byte(strings.Repeat(" ", n)) }

func TestSnapshotRejectsLinksAndIgnoresArchiveAttributes(t *testing.T) {
	t.Run("AFP-V0-019 TestSnapshotRejectsLinksAndIgnoresArchiveAttributes", func(t *testing.T) {
		root, r := fixture(t)
		os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("a.go export-ignore\n"), 0600)
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "attributes")
		r.Commit = git(t, root, "rev-parse", "HEAD")
		r.Tree = git(t, root, "rev-parse", "HEAD^{tree}")
		r.Paths = []string{".gitattributes", "a.go"}
		r.Digest = PathDigest(r.Paths)
		dir, cleanup, err := r.Materialize(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if _, err := os.Stat(filepath.Join(dir, "a.go")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("a.go", filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "link")
		r.Commit = git(t, root, "rev-parse", "HEAD")
		r.Tree = git(t, root, "rev-parse", "HEAD^{tree}")
		r.Paths = append(r.Paths, "link")
		r.Digest = PathDigest(r.Paths)
		if _, cleanup, err := r.Materialize(context.Background(), root); err == nil || cleanup != nil {
			t.Fatal("symlink admitted")
		}
	})
}
