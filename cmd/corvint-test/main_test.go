package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/changee2e"
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

func fixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	for _, src := range []string{"../../internal/appmap/testdata/fixture", "../../internal/changee2e/testdata/overlay"} {
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, rel), data, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) string {
		gitTest(t, root, "add", "-A")
		gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", msg)
		return gitTest(t, root, "rev-parse", "HEAD")
	}
	gitTest(t, root, "init", "-q")
	base := commit("base")
	head, err := os.ReadFile(filepath.Join(root, "app/components/teesheet.head.html"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app/components/teesheet.html"), head, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, base, commit("book")
}

// CCE-V0-015: each subcommand is reachable as `corvint-test e2e <sub>`; anything else is a usage
// error, and --out inside the repository refuses.
func TestCCEV0015SubcommandsReachable(t *testing.T) {
	root, base, head := fixture(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"e2e", "catalog", "--root", root, "--rev", head},
		{"e2e", "obligations", "--root", root, "--base", base, "--head", head},
		{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
	} {
		var out bytes.Buffer
		if err := run(ctx, args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var doc struct {
			Schema string `json:"schema"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil || !strings.HasPrefix(doc.Schema, "e2e-") {
			t.Fatalf("%v: %v %s", args, err, out.Bytes())
		}
	}
	out := t.TempDir()
	if err := run(ctx, []string{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", out}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "e2e/corvint-compiled/book-tee-time-booked.spec.ts")); err != nil {
		t.Fatal(err)
	}
	// prove is registered: a bad flag fails at parse, not as an unknown subcommand.
	for _, args := range [][]string{{}, {"e2e"}, {"e2e", "unknown"}, {"test"}, {"e2e", "prove", "--bogus"}} {
		if err := run(ctx, args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), usage) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	var r *changee2e.Refusal
	err := run(ctx, []string{"e2e", "catalog", "--root", root, "--rev", head, "--out", filepath.Join(root, ".corvint", "c.json")}, &bytes.Buffer{})
	if !errors.As(err, &r) || r.Code != "e2e-out-not-allowed" {
		t.Fatalf("%v", err)
	}
}

// CCE-V0-015: core `corvint` never imports the companion or its package; the companion never
// names `corvint` as an executable, and its diagnostics stay disjoint from core `corvint test`'s
// `corvint-test:` prefix.
func TestCCEV0015CoreBoundary(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "../corvint").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasSuffix(dep, "/internal/changee2e") || strings.HasSuffix(dep, "/cmd/corvint-test") {
			t.Fatalf("core corvint imports %s", dep)
		}
	}
	srcs, _ := filepath.Glob("../../internal/changee2e/*.go")
	srcs = append(srcs, "main.go")
	corvintArgv := regexp.MustCompile(`"corvint"|"corvint\s|/corvint"`)
	bareDiag := regexp.MustCompile(`"corvint-test:`)
	for _, p := range srcs {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if corvintArgv.Match(data) || bareDiag.Match(data) {
			t.Fatalf("%s names the core executable or its diagnostic prefix", p)
		}
	}
}

// CCE-V0-015 subprocess check: run the built companion with a `corvint` that records any call
// first on PATH; no subcommand may reach it.
func TestCCEV0015NeverExecutesCorvint(t *testing.T) {
	root, base, head := fixture(t)
	bin := filepath.Join(t.TempDir(), "corvint-test")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	fake := t.TempDir()
	marker := filepath.Join(t.TempDir(), "called")
	if err := os.WriteFile(filepath.Join(fake, "corvint"), []byte("#!/bin/sh\necho \"$@\" >> '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, args := range [][]string{
		{"e2e", "catalog", "--root", root, "--rev", head},
		{"e2e", "obligations", "--root", root, "--base", base, "--head", head},
		{"e2e", "compile", "--root", root, "--base", base, "--head", head, "--out", t.TempDir()},
	} {
		if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
		data, _ := os.ReadFile(marker)
		t.Fatalf("companion executed corvint: %s", data)
	}
}
