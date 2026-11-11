// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo makes a repository at a temporary root holding files, commits
// them, and returns its root and the Git handle.
func gitRepo(t *testing.T, files map[string]string) (string, Git) {
	t.Helper()
	executable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	for name, content := range files {
		writeFile(t, root, name, content)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	return root, Git{Executable: executable, Root: root}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// The commit tree and the working tree name each file by its Git blob ID;
// the working tree rehashes changed and untracked files, leaves ignored ones
// out, and listing writes nothing, not even the index (product invariant 4).
func TestTIVV1005TreesNameFilesByBlobID(t *testing.T) {
	root, git := gitRepo(t, map[string]string{"go.mod": "module x\n", "a_test.go": "package x\n", ".gitignore": "out/\n"})
	ctx := context.Background()
	head, err := git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := NewCommitTree(ctx, git, head)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := committed.Files()
	if want := runGit(t, root, "rev-parse", "HEAD:a_test.go"); files["a_test.go"] != want {
		t.Fatalf("commit tree blob %s, git says %s", files["a_test.go"], want)
	}
	if data, err := committed.Read("go.mod", 100); err != nil || string(data) != "module x\n" {
		t.Fatalf("commit tree read %q %v", data, err)
	}
	writeFile(t, root, "a_test.go", "package x\n\nfunc TestA() {}\n")
	writeFile(t, root, "b_test.go", "package x\n")
	writeFile(t, root, "out/c_test.go", "package x\n")
	index, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	work, err := NewWorkTree(ctx, git)
	if err != nil {
		t.Fatal(err)
	}
	files, _ = work.Files()
	if files["a_test.go"] != BlobID("sha1", []byte("package x\n\nfunc TestA() {}\n")) || files["b_test.go"] != BlobID("sha1", []byte("package x\n")) {
		t.Fatalf("work tree blobs %v", files)
	}
	if files["go.mod"] != runGit(t, root, "rev-parse", "HEAD:go.mod") {
		t.Fatal("unchanged tracked file does not keep its index blob")
	}
	if _, ok := files["out/c_test.go"]; ok {
		t.Fatal("ignored file listed")
	}
	if after, _ := os.ReadFile(filepath.Join(root, ".git", "index")); !bytes.Equal(index, after) {
		t.Fatal("listing the working tree rewrote the index")
	}
}
