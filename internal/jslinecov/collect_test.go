package jslinecov

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// fixture copies testdata/repo to a fresh directory and, when
// JSLINECOV_NODE_MODULES names an installed node_modules, links it there so
// the runners and their coverage providers resolve.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := filepath.WalkDir("testdata/repo", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/repo", p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if nm := os.Getenv("JSLINECOV_NODE_MODULES"); nm != "" {
		if err := os.Symlink(nm, filepath.Join(dir, "node_modules")); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// executable finds a runner: an installed node_modules/.bin entry, then PATH.
func executable(t *testing.T, root, name string) string {
	t.Helper()
	if p := filepath.Join(root, "node_modules", ".bin", name); fileExists(p) {
		return p
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not installed; set JSLINECOV_NODE_MODULES to a node_modules holding it", name)
	}
	abs, _ := filepath.Abs(p)
	return abs
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// TestCollect covers LPCV-V0-967 and LPCV-V0-955 for each node runner: every
// test runs alone under the runner's own filter, its record holds the lines
// it executed, mapped through the build's source map to the TypeScript
// original, and the file's in-order run is recorded. Runners not installed
// skip.
func TestCollect(t *testing.T) {
	for _, c := range []struct {
		runner, bin, file, config string
		names                     []string // the record names, sorted
	}{
		{runner: "node-test", bin: "node", file: "test/node.test.js", names: []string{"adds", "squares"}},
		{runner: "mocha", bin: "mocha", file: "test/mocha.spec.js", names: []string{"math adds", "math squares"}},
		{runner: "jasmine", bin: "jasmine", file: "test/jasmine.spec.js", names: []string{"math adds", "math squares"}},
		{runner: "jest", bin: "jest", file: "test/jest.test.js", names: []string{"math adds", "math squares"}},
		{runner: "vitest", bin: "vitest", file: "test/vitest.test.js", config: "vitest.config.mjs", names: []string{"math adds", "math squares"}},
		{runner: "ava", bin: "ava", file: "test/ava.test.mjs", names: []string{"adds", "squares"}},
		{runner: "bun-test", bin: "bun", file: "test/bun.test.js", names: []string{"adds", "squares"}},
		{runner: "deno-test", bin: "deno", file: "test/deno.test.ts", names: []string{"halves", "squares"}},
	} {
		t.Run(c.runner, func(t *testing.T) {
			root := fixture(t)
			exe := executable(t, root, c.bin)
			out, err := Collect(context.Background(), root, Options{
				Module: "m", Runner: c.runner, Executable: exe, Files: []string{c.file}, Config: c.config,
				Work: t.TempDir(), Parallel: 2, Timeout: 2 * time.Minute, SuiteTimeout: 2 * time.Minute,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			var names []string
			for _, o := range out {
				if o.Record == nil {
					t.Fatalf("%s divergent: %s", o.Name, o.Divergent)
				}
				if o.Package != c.file {
					t.Fatalf("package %q", o.Package)
				}
				got[o.Name] = string(o.Record.Profile)
				if o.Name != linecov.InOrder {
					names = append(names, o.Name)
				}
			}
			if strings.Join(names, ",") != strings.Join(c.names, ",") {
				t.Fatalf("records %v, want %v", names, c.names)
			}
			if _, ok := got[linecov.InOrder]; !ok {
				t.Fatal("no in-order record")
			}
			ran := func(profile, file string, line int) bool {
				return strings.Contains(profile, "m/"+file+":"+itoa(line)+".1,"+itoa(line+1)+".1 1 1\n")
			}
			// Bun marks a never-called function's return line executed: an
			// over-approximation, which widens selection but stays sound.
			exact := c.runner != "bun-test"
			squares := got[c.names[len(c.names)-1]]
			if !ran(squares, "src/calc.ts", 2) || exact && ran(squares, "src/calc.ts", 7) {
				t.Errorf("squares record not mapped to src/calc.ts:\n%s", squares)
			}
			if strings.Contains(squares, "out/calc.js") {
				t.Errorf("generated file recorded:\n%s", squares)
			}
			if c.runner == "deno-test" {
				if halves := got["halves"]; ran(halves, "src/calc.ts", 2) {
					t.Errorf("halves ran square:\n%s", halves)
				}
				return
			}
			adds := got[c.names[0]]
			if !ran(adds, "lib.js", 2) || exact && ran(adds, "lib.js", 7) || ran(adds, "src/calc.ts", 2) {
				t.Errorf("adds record:\n%s", adds)
			}
			if ran(squares, "lib.js", 2) {
				t.Errorf("squares ran add:\n%s", squares)
			}
		})
	}
}

// TestCollectIsolation covers LPCV-V0-955: a test whose title another test
// of the file shares has no record, and a failing in-order run leaves the
// file without records.
func TestCollectIsolation(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	root := fixture(t)
	write := func(name, src string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("test/dup.test.js", `const test = require("node:test");
test("same", () => {});
test("same", () => {});
test("alone", () => {});
`)
	write("test/fail.test.js", `const test = require("node:test");
test("ok", () => {});
test("bad", () => { throw new Error("x"); });
`)
	out, err := Collect(context.Background(), root, Options{
		Module: "m", Runner: "node-test", Executable: node, Files: []string{"test/dup.test.js", "test/fail.test.js"},
		Work: t.TempDir(), Timeout: time.Minute, SuiteTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	var summary []string
	for _, o := range out {
		state := "record"
		if o.Record == nil {
			state = "divergent"
		}
		summary = append(summary, o.Package+" "+o.Name+" "+state)
	}
	want := []string{
		"test/dup.test.js (in-order) record",
		"test/dup.test.js alone record",
		"test/dup.test.js same divergent",
		"test/dup.test.js same divergent",
		"test/fail.test.js (in-order) divergent",
	}
	if strings.Join(summary, "\n") != strings.Join(want, "\n") {
		t.Fatalf("outcomes:\n%s\nwant:\n%s", strings.Join(summary, "\n"), strings.Join(want, "\n"))
	}
}

// TestFilter covers LPCV-V0-955: each runner's filter anchors the title, and
// AVA titles its glob cannot match exactly are refused.
func TestFilter(t *testing.T) {
	for runner, want := range map[string]string{
		"mocha":     "--grep (^| )a\\.b$",
		"jasmine":   "--filter=(^| )a\\.b$",
		"node-test": "--test-name-pattern=^a\\.b$",
		"jest":      "-t (^| )a\\.b$",
		"deno-test": "--filter /^a\\.b$/",
		"ava":       "--match=a.b",
	} {
		got, ok := Options{Runner: runner}.filter("a.b")
		if !ok || strings.Join(got, " ") != want {
			t.Errorf("%s: %q", runner, got)
		}
	}
	for _, title := range []string{"a*", "!a"} {
		if _, ok := (Options{Runner: "ava"}).filter(title); ok {
			t.Errorf("ava accepted %q", title)
		}
	}
	if _, err := Collect(context.Background(), ".", Options{Runner: "karma", Executable: "/x", Work: "/w"}); err == nil {
		t.Fatal("unknown runner accepted")
	}
}
