//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestAFPV02370DryRunNarrowsAnAllowListedDependent drives `corvint test
// --dry-run` over a module whose package p tests lib through two test
// functions: an edit of lib.A narrows p's DEPENDENCY_PATH selection to TestA,
// the negative control (an edit of lib.B) to TestB, an added init keeps p
// whole with the typed-reach reason, and a package off the allow-list stays
// whole without one (AFP-V0-2370, AFP-V0-2372, AFP-V0-2375).
func TestAFPV02370DryRunNarrowsAnAllowListedDependent(t *testing.T) {
	saved := typedReachPackages
	typedReachPackages = map[string]bool{"p": true}
	t.Cleanup(func() { typedReachPackages = saved })
	root := t.TempDir()
	lib := "package lib\n\nfunc A() int { return 1 }\n\nfunc B() int { return 2 }\n"
	files := map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"lib/lib.go":                lib,
		"p/p.go":                    "package p\n",
		"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/ed\"\n\t\"example.test/m/lib\"\n)\n\nfunc TestA(t *testing.T) { _ = lib.A() }\n\nfunc TestB(t *testing.T) { _ = lib.B() }\n\nfunc TestEmbed(t *testing.T) { _ = ed.Read() }\n",
		"ed/ed.go":                  "package ed\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n\nfunc Read() string { return data }\n",
		"ed/data.txt":               "one\n",
		"lib/notes.txt":             "read at run time\n",
		"o/o.go":                    "package o\n",
		"o/o_test.go":               "package o\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestO(t *testing.T) { _ = lib.A() }\n",
		sessiontest.DeclarationPath: sessionDeclaration(""),
	}
	fixture := newTypedSliceFixture(t, root, files)
	planFiles := fixture.plan
	plan := func(t *testing.T, libSource string) map[string]sessiontest.Unit {
		t.Helper()
		return planFiles(t, map[string]string{"lib/lib.go": libSource})
	}
	if _, ok := plan(t, strings.Replace(lib, "return 1", "return 3", 1))["go:example.test/m/o"]; !ok {
		t.Fatal("o, which calls A, is not selected for an edit of A")
	}
	for _, tc := range []struct{ name, edit, pattern string }{
		{"edit of A", strings.Replace(lib, "return 1", "return 3", 1), "^(TestA)$"},
		{"negative control: edit of B", strings.Replace(lib, "return 2", "return 4", 1), "^(TestB)$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			units := plan(t, tc.edit)
			got := units["go:example.test/m/p"]
			if got.Slice != sessiontest.SliceSelected || got.Pattern != tc.pattern || !strings.Contains(got.Reason, "(typed reach: 2 of 3 test functions cannot reach it; build configuration sha256:") || got.ReasonCode != sessiontest.ReasonSelectedReachingChange {
				t.Fatalf("p %+v, want pattern %s", got, tc.pattern)
			}
			// Symbol-level narrowing (ADN-V0-660) leaves o out when only B
			// changes; when o is selected it stays whole.
			if other, ok := units["go:example.test/m/o"]; ok && (other.Slice != sessiontest.SliceWhole || strings.Contains(other.Reason, "typed reach")) {
				t.Fatalf("o off the allow-list %+v, want whole without typed reach", other)
			}
		})
	}
	// AFP-V0-2377: a data file is not Go source, so no declaration shows its
	// change; an embedded one and one beside a package's source keep every
	// selection of a dependent whole, and never slice it.
	for _, tc := range []struct{ name, file, reason string }{
		{"an embedded data file", "ed/data.txt", "may be embedded by the package in ed"},
		{"a data file beside a package", "lib/notes.txt", "is a data file under the Go package directory lib"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edits := map[string]string{tc.file: "two\n"}
			if tc.file == "lib/notes.txt" {
				// Ensure the dependent is selected, even when the data file
				// has no literal reader in this fixture. Mixed code/data edits
				// must preserve the whole-package dependency selection.
				edits["lib/lib.go"] = strings.Replace(lib, "return 1", "return 3", 1)
			}
			units := planFiles(t, edits)
			got, ok := units["go:example.test/m/p"]
			if !ok {
				t.Fatal("p is not selected for the dependency change")
			}
			if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("p %+v, want whole naming %q", got, tc.reason)
			}
		})
	}
	t.Run("an added init keeps p whole", func(t *testing.T) {
		got := plan(t, lib+"\nvar hits int\n\nfunc init() { hits++ }\n")["go:example.test/m/p"]
		if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "(typed reach: ALWAYS_RUN_REACHES_CHANGE: ") {
			t.Fatalf("p %+v, want whole with the always-run reason", got)
		}
	})
	t.Run("a removed shared write keeps p whole", func(t *testing.T) {
		affectedGit(t, root, "checkout", "-q", "--", ".")
		before := "package lib\n\nvar Shared int\n\nfunc A() int { Shared = 1; return 1 }\n\nfunc B() int { return 2 }\n"
		if err := os.WriteFile(filepath.Join(root, "lib/lib.go"), []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := files["p/p_test.go"] + "\nfunc TestShared(t *testing.T) { if lib.Shared != 1 { t.Fatal(lib.Shared) } }\n"
		if err := os.WriteFile(filepath.Join(root, "p/p_test.go"), []byte(reader), 0o644); err != nil {
			t.Fatal(err)
		}
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "shared-write baseline")
		fixture.base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		got := plan(t, strings.Replace(before, "Shared = 1; ", "", 1))["go:example.test/m/p"]
		if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "BASE_MAY_WRITE_SHARED_STATE") {
			t.Fatalf("removed write: %+v, want whole with the base-effect reason", got)
		}
	})
}

// typedSliceFixture is a committed module under root whose `corvint test
// --dry-run` plans are read per unit after a set of edits.
type typedSliceFixture struct {
	root, base string
}

func newTypedSliceFixture(t *testing.T, root string, files map[string]string) *typedSliceFixture {
	t.Helper()
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	// The fake go command answers the runner; the build configuration probe
	// (AFP-V0-2372) reaches the real one.
	real, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\ncase \"$*\" in \"env GOFLAGS\"|*\"{{with context}}\"*) exec '" + real + "' \"$@\";; esac\n" + strings.TrimPrefix(sessionFakeGo, "#!/bin/sh\n")
	bin := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SESSION_FAKE_LOG", filepath.Join(t.TempDir(), "runners.log"))
	t.Setenv("SESSION_FAKE_MODE", "pass")
	return &typedSliceFixture{root: root, base: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))}
}

// plan restores the committed tree, applies edits and returns the dry-run
// units by ID.
func (f *typedSliceFixture) plan(t *testing.T, edits map[string]string) map[string]sessiontest.Unit {
	t.Helper()
	affectedGit(t, f.root, "checkout", "-q", "--", ".")
	for name, body := range edits {
		if err := os.WriteFile(filepath.Join(f.root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runCLI(t, "--root", f.root, "test", "--base", f.base, "--dry-run", "--format", "json", "--manifest", filepath.Join(t.TempDir(), "manifest.json"))
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	var printed struct {
		Invocations []struct {
			Units []sessiontest.Unit `json:"units"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		t.Fatal(err)
	}
	units := map[string]sessiontest.Unit{}
	for _, invocation := range printed.Invocations {
		for _, unit := range invocation.Units {
			units[unit.Unit] = unit
		}
	}
	return units
}

// TestAFPV02376DryRunReadsRootPackageChanges edits the module's root
// package, whose files sit beside go.mod: the changed declarations belong
// to the module path itself, so an edit of B narrows the allow-listed
// dependent p to TestB rather than to no test, and an edit of A, which
// writes shared state at base, keeps p whole (AFP-V0-2376).
func TestAFPV02376DryRunReadsRootPackageChanges(t *testing.T) {
	saved := typedReachPackages
	typedReachPackages = map[string]bool{"p": true}
	t.Cleanup(func() { typedReachPackages = saved })
	lib := "package m\n\nvar Shared int\n\nfunc A() int { Shared = 1; return 1 }\n\nfunc B() int { return 2 }\n"
	fixture := newTypedSliceFixture(t, t.TempDir(), map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"lib.go":                    lib,
		"p/p.go":                    "package p\n",
		"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m\"\n)\n\nfunc TestA(t *testing.T) { _ = m.A() }\n\nfunc TestB(t *testing.T) { _ = m.B() }\n\nfunc TestControl(t *testing.T) {}\n",
		sessiontest.DeclarationPath: sessionDeclaration(""),
	})
	t.Run("edit of B narrows to TestB", func(t *testing.T) {
		got, ok := fixture.plan(t, map[string]string{"lib.go": strings.Replace(lib, "return 2", "return 4", 1)})["go:example.test/m/p"]
		if !ok || got.Slice != sessiontest.SliceSelected || got.Pattern != "^(TestB)$" {
			t.Fatalf("p %+v (selected %v), want pattern ^(TestB)$", got, ok)
		}
	})
	t.Run("edit of a base shared writer keeps p whole", func(t *testing.T) {
		got, ok := fixture.plan(t, map[string]string{"lib.go": strings.Replace(lib, "return 1", "return 3", 1)})["go:example.test/m/p"]
		if !ok || got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "BASE_MAY_WRITE_SHARED_STATE") {
			t.Fatalf("p %+v (selected %v), want whole with the base-effect reason", got, ok)
		}
	})
}

// typedSliceDeclaration is the fixture declaration with extra go flags and
// environment entries.
func typedSliceDeclaration(env, flags string) string {
	return `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"` + env + `}, "flags": ["-p", "1", "-count=1"` + flags + `]}, "languages": {}}` + "\n"
}

// TestAFPV02372DryRunBindsExecutorBuildConfiguration plans a package with a
// test file only a declared build tag compiles: with the tag declared, an
// edit of A keeps that file's test and the slice names the configuration
// identity; without it the test is not compiled, the identity differs, and an
// executor flag the model does not know keeps the package whole
// (AFP-V0-2372).
func TestAFPV02372DryRunBindsExecutorBuildConfiguration(t *testing.T) {
	saved := typedReachPackages
	typedReachPackages = map[string]bool{"p": true}
	t.Cleanup(func() { typedReachPackages = saved })
	lib := "package lib\n\nfunc A() int { return 1 }\n\nfunc B() int { return 2 }\n"
	edit := strings.Replace(lib, "return 1", "return 3", 1)
	identity := func(t *testing.T, reason string) string {
		t.Helper()
		_, after, ok := strings.Cut(reason, "; build configuration ")
		if !ok {
			t.Fatalf("reason %q names no build configuration", reason)
		}
		return strings.TrimSuffix(after, ")")
	}
	plan := func(t *testing.T, declaration string) sessiontest.Unit {
		t.Helper()
		fixture := newTypedSliceFixture(t, t.TempDir(), map[string]string{
			"go.mod":                    "module example.test/m\n\ngo 1.27\n",
			"lib/lib.go":                lib,
			"p/p.go":                    "package p\n",
			"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestA(t *testing.T) { _ = lib.A() }\n\nfunc TestB(t *testing.T) { _ = lib.B() }\n",
			"p/tagged_test.go":          "//go:build floortag\n\npackage p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestTagged(t *testing.T) { _ = lib.A() }\n",
			sessiontest.DeclarationPath: declaration,
		})
		return fixture.plan(t, map[string]string{"lib/lib.go": edit})["go:example.test/m/p"]
	}
	tagged := plan(t, typedSliceDeclaration("", `, "-tags=floortag"`))
	if tagged.Slice != sessiontest.SliceSelected || tagged.Pattern != "^(TestA|TestTagged)$" || !strings.Contains(tagged.Reason, "(typed reach: 1 of 3 test functions") {
		t.Fatalf("declared tag: %+v", tagged)
	}
	plain := plan(t, typedSliceDeclaration("", ""))
	if plain.Slice != sessiontest.SliceSelected || plain.Pattern != "^(TestA)$" || !strings.Contains(plain.Reason, "(typed reach: 1 of 2 test functions") {
		t.Fatalf("no tag: %+v", plain)
	}
	if identity(t, tagged.Reason) == identity(t, plain.Reason) {
		t.Fatalf("the declared tag does not change the identity: %s", tagged.Reason)
	}
	for name, declaration := range map[string]string{
		"an unmodelled flag":    typedSliceDeclaration("", `, "-gcflags=all=-N"`),
		"an unmodelled GOFLAGS": typedSliceDeclaration(`, "GOFLAGS": "-toolexec=/bin/true"`, ""),
		// The fix-only review repro: GOFLAGS enables race, the declared flag
		// disables it; explicit values and doubled switches are disabled for 1.0.
		"GOFLAGS -race with -race=false": typedSliceDeclaration(`, "GOFLAGS": "-race"`, `, "-race=false"`),
		"-race in both":                  typedSliceDeclaration(`, "GOFLAGS": "-race"`, `, "-race"`),
	} {
		t.Run(name, func(t *testing.T) {
			got := plan(t, declaration)
			if got.Slice != sessiontest.SliceWhole || !strings.Contains(got.Reason, "the executor build configuration cannot be read") {
				t.Fatalf("%s: %+v", name, got)
			}
		})
	}
}

// TestAFPV02372RunRefusesAnotherConfiguration: the executing shard recomputes
// the plan under its own environment, so a run whose build configuration
// differs from the planner's refuses the plan.
func TestAFPV02372RunRefusesAnotherConfiguration(t *testing.T) {
	saved := typedReachPackages
	typedReachPackages = map[string]bool{"p": true}
	t.Cleanup(func() { typedReachPackages = saved })
	lib := "package lib\n\nfunc A() int { return 1 }\n\nfunc B() int { return 2 }\n"
	edit := strings.Replace(lib, "return 1", "return 3", 1)
	{
		fixture := newTypedSliceFixture(t, t.TempDir(), map[string]string{
			"go.mod":                    "module example.test/m\n\ngo 1.27\n",
			"lib/lib.go":                lib,
			"p/p.go":                    "package p\n",
			"p/p_test.go":               "package p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestA(t *testing.T) { _ = lib.A() }\n\nfunc TestB(t *testing.T) { _ = lib.B() }\n",
			"p/tagged_test.go":          "//go:build floortag\n\npackage p\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/lib\"\n)\n\nfunc TestTagged(t *testing.T) { _ = lib.A() }\n",
			sessiontest.DeclarationPath: typedSliceDeclaration("", ""),
		})
		if err := os.WriteFile(filepath.Join(fixture.root, "lib/lib.go"), []byte(edit), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest := filepath.Join(t.TempDir(), "manifest.json")
		if code, _, stderr := runCLI(t, "--root", fixture.root, "test", "--base", fixture.base, "--dry-run", "--manifest", manifest); code != 0 {
			t.Fatalf("dry run %d %s", code, stderr)
		}
		// A tag no file names selects the same tests: only the bound
		// configuration identity tells the runs apart.
		t.Setenv("GOFLAGS", "-tags=unusedtag")
		code, _, stderr := runCLI(t, "--root", fixture.root, "test", "--base", fixture.base, "--manifest", manifest, "--receipt", filepath.Join(t.TempDir(), "run.json"))
		if code != 3 || !strings.Contains(stderr, "it differs from the manifest this shard recomputed") {
			t.Fatalf("a run under -tags=unusedtag accepted the untagged plan: %d %s", code, stderr)
		}
	}
}
