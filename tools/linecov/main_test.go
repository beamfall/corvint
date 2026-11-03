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
	// An untracked file in a module package the collected tests depend on
	// can run at init and change which blocks they reach.
	write(t, root, "d/d.go", "package d\n\nfunc G() int { return 1 }\n")
	write(t, root, "p/dep_test.go", "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/d\"\n)\n\nfunc TestDep(t *testing.T) { d.G() }\n")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "dep")
	if err := clean(root, []string{"p"}); err != nil {
		t.Fatalf("clean checkout with a dependency refused: %v", err)
	}
	write(t, root, "d/init.go", "package d\n\nvar _ = G()\n")
	if err := clean(root, []string{"p"}); err == nil || !strings.Contains(err.Error(), "d/init.go") {
		t.Fatalf("untracked file in a dependency accepted: %v", err)
	}
	os.Remove(filepath.Join(root, "d", "init.go"))
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
	t.Setenv("GOENV", filepath.Join(root, "goenv"))
	t.Setenv("GOFLAGS", "")
	plain, _, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOFLAGS", "-tags=linecovtest")
	tagged, flags, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	if plain == tagged || !strings.Contains(tagged, "linecovtest") || flags != "-tags=linecovtest" {
		t.Fatalf("runner ignores GOFLAGS: %q vs %q (%q)", plain, tagged, flags)
	}
	// Flags saved with `go env -w` are effective without the variable, so the
	// flags collection builds under must carry them too.
	t.Setenv("GOFLAGS", "")
	run(t, root, "go", "env", "-w", "GOFLAGS=-tags=savedtag")
	saved, flags, err := runner(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saved, "savedtag") || flags != "-tags=savedtag" {
		t.Fatalf("runner ignores saved GOFLAGS: %q (%q)", saved, flags)
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

// TestCINarrowsWholePackages collects a two-package module, changes one
// function, and checks ci turns the changed package into a -run slice of the
// test that reaches it, drops the untouched package, and keeps given slices.
func TestCINarrowsWholePackages(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n\ngo 1.22\n")
	write(t, root, "p/p.go", "package p\n\nfunc F() int {\n\treturn 1\n}\n\nfunc G() int {\n\treturn 2\n}\n")
	write(t, root, "p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { F() }\n\nfunc TestB(t *testing.T) { G() }\n")
	write(t, root, "q/q.go", "package q\n\nfunc H() int { return 3 }\n")
	write(t, root, "q/q_test.go", "package q\n\nimport \"testing\"\n\nfunc TestQ(t *testing.T) { H() }\n")
	commit := func() {
		run(t, root, "git", "add", ".")
		run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	}
	run(t, root, "git", "init", "-q")
	commit()
	base := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	index := filepath.Join(t.TempDir(), "index")
	if err := collect([]string{"-root", root, "-out", index, "-parallel", "2"}); err != nil {
		t.Fatal(err)
	}
	write(t, root, "p/p.go", "package p\n\nfunc F() int {\n\treturn 10\n}\n\nfunc G() int {\n\treturn 2\n}\n")
	commit()
	in := t.TempDir()
	write(t, in, "packages", "example.com/m/p\nexample.com/m/q\n")
	write(t, in, "slices", "-run ^TestX$ example.com/m/r\n")
	out := t.TempDir()
	if err := ci([]string{"-root", root, "-index", index, "-base", base, "-packages", filepath.Join(in, "packages"), "-slices", filepath.Join(in, "slices"), "-out", out}); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got := read("packages"); got != "" {
		t.Errorf("packages = %q, want none", got)
	}
	if got, want := read("slices"), "-run ^(TestA)$ example.com/m/p\n-run ^TestX$ example.com/m/r\n"; got != want {
		t.Errorf("slices = %q, want %q\nplan: %s", got, want, read("plan.json"))
	}
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
