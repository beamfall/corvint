package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// source wraps callback bodies in a file; each case marks the lines it expects reported with
// a trailing "// FAIL" comment.
func source(body string) string {
	return "package p\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n\t\"path/filepath\"\n)\n\nvar _ = os.Getpid\n\n" + body
}

func want(text string) []int {
	var lines []int
	for i, line := range strings.Split(text, "\n") {
		if strings.HasSuffix(line, "// FAIL") {
			lines = append(lines, i+1)
		}
	}
	return lines
}

// ASD-V0-2476 (V1-4511): each SkipDir return a callback can reach for a file is reported, and
// each one reached only for a directory, the root or a directory read error is not.
func TestASDV2476ReportsUnguardedSkipDir(t *testing.T) {
	cases := map[string]string{
		"path test only": `func a(root string) error {
	return fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
		if path == ".git" {
			return fs.SkipDir // FAIL
		}
		return nil
	})
}
`,
		"name test only": `func a(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if d.Name() == ".git" || d.Name() == "node_modules" {
			return filepath.SkipDir // FAIL
		}
		return nil
	})
}
`,
		"else of IsDir, negated IsDir, IsDir in a disjunction, another value's IsDir": `func a(root string, other os.FileInfo) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		} else {
			return filepath.SkipDir // FAIL
		}
		if d.IsDir() || p == "x" {
			return filepath.SkipDir // FAIL
		}
		if p == ".git" && other.IsDir() {
			return filepath.SkipDir // FAIL
		}
		// The early return below guards what follows it, so it comes last.
		if !d.IsDir() {
			return filepath.SkipDir // FAIL
		}
		return nil
	})
}
`,
		"callback through a variable, blank entry, filepath.Walk": `func a(root string) error {
	walk := func(p string, d fs.DirEntry, err error) error {
		if p == ".git" {
			return fs.SkipDir // FAIL
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return err
	}
	if err := filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		return filepath.SkipDir // FAIL
	}); err != nil {
		return err
	}
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if info.Name() == "vendor" {
			return filepath.SkipDir // FAIL
		}
		return nil
	})
}
`,
		"guarded forms": `func a(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if (p == "a" && (d.IsDir())) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			for _, skip := range []string{"x"} {
				if p == skip {
					return filepath.SkipDir
				}
			}
		}
		if !d.IsDir() {
			return nil
		} else {
			return filepath.SkipDir
		}
		switch {
		case d.IsDir() && d.Name() == "testdata":
			return filepath.SkipDir
		case p == "y":
			return filepath.SkipDir // FAIL
		}
		if p == ".corvint/index" {
			if !d.IsDir() {
				return nil
			}
			return filepath.SkipDir
		}
		return nil
	})
}
`,
		"not a walk callback, nested literal": `func a(root string, visit func(string, fs.DirEntry, error) error) error {
	custom := func(p string, d fs.DirEntry, err error) error {
		return filepath.SkipDir
	}
	_ = custom
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		inner := func() error { return filepath.SkipDir }
		_ = inner
		return nil
	})
}
`,
	}
	for name, body := range cases {
		text := source(body)
		got, err := check("p.go", []byte(text))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if expected := want(text); !slices.Equal(got, expected) {
			t.Errorf("%s: reported lines %v, want %v", name, got, expected)
		}
	}
}

// ASD-V0-2476: a walk and SkipDir are recognized under the name the file's imports give
// path/filepath and io/fs, an alias or a dot import's bare name, and a package of another path
// imported under the name filepath names neither.
func TestASDV2476ResolvesImportAliases(t *testing.T) {
	cases := map[string]string{
		"aliased walk and SkipDir": `package p

import (
	iofs "io/fs"
	fp "path/filepath"
)

func a(root string) error {
	return fp.WalkDir(root, func(p string, d iofs.DirEntry, err error) error {
		if p == ".git" {
			return fp.SkipDir // FAIL
		}
		if d.IsDir() {
			return iofs.SkipDir
		}
		return nil
	})
}
`,
		"aliased SkipDir in a plain walk": `package p

import (
	iofs "io/fs"
	"path/filepath"
)

func a(root string) error {
	return filepath.WalkDir(root, func(p string, d iofs.DirEntry, err error) error {
		if d.Name() == "vendor" {
			return iofs.SkipDir // FAIL
		}
		return nil
	})
}
`,
		"dot import": `package p

import (
	. "io/fs"
	"os"
)

func a(root string) error {
	return WalkDir(os.DirFS(root), ".", func(p string, d DirEntry, err error) error {
		if p == ".git" {
			return SkipDir // FAIL
		}
		if d.IsDir() {
			return SkipDir
		}
		return nil
	})
}
`,
		"another package named filepath": `package p

import (
	"io/fs"

	filepath "example.com/walker"
)

func a(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		return fs.SkipDir
	})
}
`,
	}
	for name, text := range cases {
		got, err := check("p.go", []byte(text))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if expected := want(text); !slices.Equal(got, expected) {
			t.Errorf("%s: reported lines %v, want %v", name, got, expected)
		}
	}
}

// ASD-V0-2476: run lists tracked non-test files outside testdata, prints a FAIL line with the
// fix for each finding and exits 1, and exits 0 when none remain.
func TestASDV2476RunScansTrackedNonTestFiles(t *testing.T) {
	root := t.TempDir()
	bad := source("func a(root string) error {\n\treturn fs.WalkDir(os.DirFS(root), \".\", func(path string, entry fs.DirEntry, err error) error {\n\t\tif path == \".git\" {\n\t\t\treturn fs.SkipDir\n\t\t}\n\t\treturn nil\n\t})\n}\n")
	write := func(name, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("p/p_test.go", bad)
	write("p/testdata/q.go", bad)
	write("p/untracked.go", bad)
	write("p/ok.go", "package p\n")
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "maintenance.auto=false"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "p/p_test.go", "p/testdata/q.go", "p/ok.go")
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 0 || stdout.String() != "walk-skipdir-check: ok\n" {
		t.Fatalf("test, testdata and untracked files: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	write("p/walk.go", bad)
	git("add", "p/walk.go")
	stdout.Reset()
	stderr.Reset()
	if code := run(root, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "walk-skipdir-check: FAIL p/walk.go:14: ") ||
		!strings.Contains(stderr.String(), "fix: return SkipDir only under ENTRY.IsDir() and nil for a file") {
		t.Fatalf("tracked finding: code=%d stderr=%q", code, stderr.String())
	}
}
