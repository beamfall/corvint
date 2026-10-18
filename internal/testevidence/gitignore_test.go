package testevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// gitRepository makes a real Git repository whose status ignores user config.
func gitRepository(t *testing.T) (string, func(...string) string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	git("init", "-q")
	return root, git
}

func retainedDocument(t *testing.T, root string) []byte {
	t.Helper()
	testFile := filepath.Join(root, "adds.test.ts")
	if err := os.WriteFile(testFile, []byte("test('adds')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("test('adds')\n"))
	return fmt.Appendf(nil, `{"receipt":{"kind":"unit","identity":{"testFileDigests":{%q:%q}},"tests":[{"name":"adds","state":"passed"}]}}`+"\n", testFile, hex.EncodeToString(digest[:]))
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// LPCV-V0-084: the first retention in a repository without .corvint writes
// .corvint/.gitignore, so status stays clean and discovery still selects the
// retained document.
func TestLPCVV0084FirstRetentionKeepsGitStatusClean(t *testing.T) {
	root, git := gitRepository(t)
	document := retainedDocument(t, root)
	git("add", "adds.test.ts")
	git("commit", "-q", "-m", "test file")
	name, err := Retain(root, "corvint-js-test-provider", document)
	if err != nil {
		t.Fatal(err)
	}
	if status := git("status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("retention dirtied the worktree:\n%s", status)
	}
	if got := readText(t, filepath.Join(root, ".corvint", ".gitignore")); got != "*\n" {
		t.Fatalf(".corvint/.gitignore = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".corvint", "test-evidence", ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("a created .corvint also got an evidence .gitignore: %v", err)
	}
	discovered, err := testvaliditydoc.Discover(root)
	if err != nil || discovered.Discovery == nil || discovered.Discovery.Evidence != testvaliditydoc.EvidenceDirectory+"/"+name {
		t.Fatalf("discovery after retention: %+v %v", discovered.Discovery, err)
	}
}

// LPCV-V0-084: an existing .corvint may hold tracked project files, so only the
// evidence directory is ignored, and an existing .corvint/.gitignore is never
// overwritten, on the first retention or a later one.
func TestLPCVV0084ExistingCorvintAndIgnoreAreKept(t *testing.T) {
	root, git := gitRepository(t)
	document := retainedDocument(t, root)
	if err := os.Mkdir(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	const own = "# project-owned\nscratch/\n"
	for path, body := range map[string]string{".corvint/config.json": "{}\n", ".corvint/.gitignore": own} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-q", "-m", "project corvint config")
	for range 2 {
		if _, err := Retain(root, "corvint-go-test-provider", document); err != nil {
			t.Fatal(err)
		}
	}
	if got := readText(t, filepath.Join(root, ".corvint", ".gitignore")); got != own {
		t.Fatalf("existing .corvint/.gitignore was overwritten: %q", got)
	}
	if got := readText(t, filepath.Join(root, ".corvint", "test-evidence", ".gitignore")); got != "*\n" {
		t.Fatalf("evidence .gitignore = %q", got)
	}
	if status := git("status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("retention dirtied the worktree:\n%s", status)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "test-evidence", ".gitignore"), []byte("*\n# edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Retain(root, "corvint-go-test-provider", document); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, filepath.Join(root, ".corvint", "test-evidence", ".gitignore")); !strings.HasSuffix(got, "# edited\n") {
		t.Fatalf("evidence .gitignore was overwritten: %q", got)
	}
}

// LPCV-V0-084: an ignore file that cannot be created fails the retention
// instead of leaving the worktree dirty unnoticed.
func TestLPCVV0084IgnoreFailureFailsRetention(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	root := t.TempDir()
	evidence := filepath.Join(root, ".corvint", "test-evidence")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evidence, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evidence, 0o700) })
	if _, err := Retain(root, "p", []byte("{}")); err == nil || !strings.Contains(err.Error(), ".gitignore cannot be created") {
		t.Fatalf("unwritable evidence directory: %v", err)
	}
}
