package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v: %s", args, err, out)
	}
}

// module is a committed one-package module whose TestFail fails.
func module(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n\ngo 1.22\n")
	write(t, root, "p/p.go", "package p\n\nfunc F() int { return 1 }\n")
	write(t, root, "p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n\nfunc TestFail(t *testing.T) { t.Fail() }\n")
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	return root
}

func TestCleanRefusesEditsAndUntrackedGo(t *testing.T) {
	root := module(t)
	if err := clean(root, []string{"p"}); err != nil {
		t.Fatalf("clean checkout refused: %v", err)
	}
	write(t, root, "notes.txt", "untracked, not Go\n")
	write(t, root, "q/q_test.go", "package q\n")
	if err := clean(root, []string{"p"}); err != nil {
		t.Fatalf("untracked files outside the collected dirs refused: %v", err)
	}
	write(t, root, "p/extra_test.go", "package p\n")
	if err := clean(root, []string{"p"}); err == nil || !strings.Contains(err.Error(), "p/extra_test.go") {
		t.Fatalf("untracked test file accepted: %v", err)
	}
	os.Remove(filepath.Join(root, "p", "extra_test.go"))
	write(t, root, "p/p.go", "package p\n\nfunc F() int { return 2 }\n")
	if err := clean(root, []string{"p"}); err == nil {
		t.Fatal("tracked edit accepted")
	}
}

func TestFailingTestsNeedsAResultPerPackage(t *testing.T) {
	root := module(t)
	failed, pkgFail, err := failingTests(root, "example.com/m", []string{"p"})
	if err != nil {
		t.Fatal(err)
	}
	if !failed["p.TestFail"] || failed["p.TestPass"] || !pkgFail {
		t.Fatalf("failed=%v pkgFail=%v", failed, pkgFail)
	}
	if _, _, err := failingTests(root, "example.com/m", []string{"p", "absent"}); err == nil {
		t.Fatal("a package go test never ran was accepted")
	}
}

func TestRunnerBindsBuildSettings(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "")
	plain, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFLAGS", "-tags=linecovtest")
	tagged, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	if plain == tagged || !strings.Contains(tagged, "linecovtest") {
		t.Fatalf("runner ignores GOFLAGS: %q vs %q", plain, tagged)
	}
}

func TestKnownRequirements(t *testing.T) {
	root := t.TempDir()
	if known, err := knownRequirements(filepath.Join(root, "absent.tsv")); err != nil || known != nil {
		t.Fatalf("missing table: %v %v", known, err)
	}
	write(t, root, "r.tsv", "id\tfile\nAHI-004\ta.md\nARTIFACT-GO-V0-001\tb.md\n")
	known, err := knownRequirements(filepath.Join(root, "r.tsv"))
	if err != nil || !known["AHI-004"] || !known["ARTIFACT-GO-V0-001"] || known["SHA-256"] {
		t.Fatalf("known=%v err=%v", known, err)
	}
}
