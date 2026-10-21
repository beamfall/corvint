package changee2e

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAll(t *testing.T, root, message string) string {
	t.Helper()
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", message)
	return gitTest(t, root, "rev-parse", "HEAD")
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// fixtureRepo commits the AMAP fixture with the CCE overlay (a servable tee-sheet app, a fixture
// page object and `.corvint/e2e-blocks.json`) and returns the root and the base revision, whose
// tee sheet shows the slot as "Pending".
func fixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	copyTree(t, "../appmap/testdata/fixture", root)
	copyTree(t, "testdata/overlay", root)
	gitTest(t, root, "init", "-q", "-b", "main")
	return root, commitAll(t, root, "base")
}

// commitFiles writes files (a nil value removes the path) and commits them.
func commitFiles(t *testing.T, root string, files map[string]*string, message string) string {
	t.Helper()
	for p, data := range files {
		target := filepath.Join(root, filepath.FromSlash(p))
		if data == nil {
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(*data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return commitAll(t, root, message)
}

func overlayText(t *testing.T, p string) *string {
	t.Helper()
	return overlayTextFrom(t, "testdata/overlay", p)
}

func overlayTextFrom(t *testing.T, dir, p string) *string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	return &s
}

func text(s string) *string { return &s }

// commitBehaviourChange makes the tee sheet show the booked slot as "Booked" (the one-screen diff).
func commitBehaviourChange(t *testing.T, root string) string {
	t.Helper()
	return commitFiles(t, root, map[string]*string{"app/components/teesheet.html": overlayText(t, "app/components/teesheet.head.html")}, "book")
}

func diffOpts(root, base, head string) DiffOptions {
	return DiffOptions{Root: root, Base: base, Head: head, Manifest: "appmap.json"}
}

var bg = context.Background()
