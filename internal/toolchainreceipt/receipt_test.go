package toolchainreceipt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeIsDeterministicAndSensitiveToEveryEntryShape(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a\n", 0o644)
	writeFile(t, filepath.Join(root, "pkg", "tool"), "tool", 0o755)
	if err := os.Symlink("../src/a.go", filepath.Join(root, "pkg", "source")); err != nil {
		t.Fatal(err)
	}

	baseline := receipt(t, root)
	if repeated := receipt(t, root); repeated != baseline {
		t.Fatalf("repeated=%#v baseline=%#v", repeated, baseline)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	originalRootMode := rootInfo.Mode() & stableModeMask
	if err := os.Chmod(root, originalRootMode|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if specialMode := receipt(t, root); specialMode == baseline {
		t.Fatal("root sticky-mode mutation did not change receipt")
	}
	if err := os.Chmod(root, originalRootMode); err != nil {
		t.Fatal(err)
	}
	if restored := receipt(t, root); restored != baseline {
		t.Fatalf("restored root mode=%#v baseline=%#v", restored, baseline)
	}

	writeFile(t, filepath.Join(root, "src", "a.go"), "package changed\n", 0o644)
	content := receipt(t, root)
	if content == baseline {
		t.Fatal("content mutation did not change receipt")
	}
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a\n", 0o644)
	if restored := receipt(t, root); restored != baseline {
		t.Fatalf("restored=%#v baseline=%#v", restored, baseline)
	}

	if err := os.Chmod(filepath.Join(root, "pkg", "tool"), 0o644); err != nil {
		t.Fatal(err)
	}
	if mode := receipt(t, root); mode == baseline {
		t.Fatal("mode mutation did not change receipt")
	}
	if err := os.Chmod(filepath.Join(root, "pkg", "tool"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(root, "pkg", "source")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("tool", filepath.Join(root, "pkg", "source")); err != nil {
		t.Fatal(err)
	}
	if link := receipt(t, root); link == baseline {
		t.Fatal("link mutation did not change receipt")
	}

	writeFile(t, filepath.Join(root, "new"), "new", 0o600)
	if added := receipt(t, root); added == baseline {
		t.Fatal("added entry did not change receipt")
	}
}

func TestTreeRejectsSymlinkRoot(t *testing.T) {
	parent := t.TempDir()
	realRoot := filepath.Join(parent, "real")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(parent, "link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := Tree(linkRoot); err == nil {
		t.Fatal("symlink root accepted")
	}
}

func TestRootIdentityCheckRejectsSubstitution(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, filepath.Join(parent, "old-root")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unchanged(root, before); err == nil {
		t.Fatal("substituted root retained authority")
	}
}

func receipt(t *testing.T, root string) Result {
	t.Helper()
	result, err := Tree(root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
