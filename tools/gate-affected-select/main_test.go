package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
)

const fixtureModule = "example.com/fixture"

func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for file, body := range files {
		name := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func verdict(plan receipt, root string) string {
	output := strings.Join(selectPackages(plan, fixtureModule, root), "\n")
	return output[strings.LastIndex(output, "\n")+1:]
}

// TestSelectPackagesAttributesEveryDirtyPath replays AFP-V0-011 and AFP-V0-012:
// a deleted source reaches its importers, a data path reaches the packages that
// enclose or name it, an unresolved package is selected on any dirty path, and
// no path can forge the verdict line.
func TestSelectPackagesAttributesEveryDirtyPath(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"core/core.go":            "package core\n",
		"core/testdata/x.txt":     "x\n",
		"mid/mid.go":              "package mid\n\nimport _ \"example.com/fixture/core\"\n",
		"leaf/leaf.go":            "package leaf\n\nimport _ \"example.com/fixture/mid\"\n",
		"solo/solo_test.go":       "package solo\n\nvar source = \"../core/core.go\"\n",
		"spec/spec_test.go":       "package spec\n\nvar specs = \"../docs/specs\"\n",
		"walker/walker_test.go":   "package walker\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\n",
		"nested/go.mod":           "module example.com/nested\n",
		"nested/reader/reader.go": "package reader\n\nvar all = \"../../../..\"\n",
	})
	cases := []struct {
		name     string
		packages []string
		dirty    string
		verdict  string
	}{
		{"a deleted package source widens to its importers", nil, "core/gone.go", "run example.com/fixture/core example.com/fixture/leaf example.com/fixture/mid example.com/fixture/walker"},
		{"a Go file read as data selects its reader", []string{"example.com/fixture/core", "example.com/fixture/leaf", "example.com/fixture/mid"}, "core/core.go", "run example.com/fixture/core example.com/fixture/leaf example.com/fixture/mid example.com/fixture/solo example.com/fixture/walker"},
		{"a document selects the packages that name it", nil, "docs/specs/x.md", "run example.com/fixture/spec example.com/fixture/walker"},
		{"a testdata fixture selects its enclosing package", nil, "core/testdata/x.txt", "run example.com/fixture/core example.com/fixture/walker"},
		{"a control character in a dirty path falls back", nil, "evil\nrun touch PWNED/b.go", "FALLBACK a dirty path contains a control character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var plan receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Provider.Go.Packages = tc.packages
			plan.Plan.Dirty = []string{tc.dirty}
			if got := verdict(plan, root); got != tc.verdict {
				t.Fatalf("verdict = %q, want %q", got, tc.verdict)
			}
		})
	}
}

// TestSelectPackagesNarrowsChangeEvidenceReaders covers AFP-V0-012 (c) for the
// CEM sidecar: a fixture-root token does not select its holder, a literal that
// resolves to the sidecar or its directory from the holder's directory does,
// partial literals composed with a root-climbing literal remain conservative,
// and any other path keeps the component-run rule.
func TestSelectPackagesNarrowsChangeEvidenceReaders(t *testing.T) {
	if changeEvidence != frontier.ExcludedPath {
		t.Fatalf("changeEvidence = %q, want the CEM excluded path %q", changeEvidence, frontier.ExcludedPath)
	}
	root := writeFixture(t, map[string]string{
		"fixture/fixture_test.go": "package fixture\n\nvar a, b, c = \".corvint\", \"change.cem.json\", \".corvint/change.cem.json\"\n",
		"exact/exact_test.go":     "package exact\n\nvar sidecar = \"../.corvint/change.cem.json\"\n",
		"anchored/anchored.go":    "package anchored\n\nfunc Sidecar(root string) string { return root + \"/.corvint/change.cem.json\" }\n",
		"deep/dir/dir_test.go":    "package dir\n\nvar evidence = \"../../.corvint\"\n",
		"partial/partial.go":      "package partial\n\nimport \"path/filepath\"\n\nvar evidence = filepath.Join(\"..\", \".corvint/change.cem\") + \".json\"\n",
		"split/split.go":          "package split\n\nfunc Sidecar(root string) string { return root + \"/.cor\" + \"vint/change.cem.json\" }\n",
	})
	cases := []struct{ name, dirty, verdict string }{
		{name: "AFP-V0-012 sidecar selects only resolving readers", dirty: ".corvint/change.cem.json", verdict: "run example.com/fixture/anchored example.com/fixture/deep/dir example.com/fixture/exact example.com/fixture/partial example.com/fixture/split"},
		{name: "AFP-V0-012 other hidden path keeps component-run readers", dirty: ".corvint/other.json", verdict: "run example.com/fixture/deep/dir example.com/fixture/fixture"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var plan receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			if got := verdict(plan, root); got != tc.verdict {
				t.Fatalf("verdict = %q, want %q", got, tc.verdict)
			}
		})
	}
}

// TestSelectPackagesFallsBackWhenAttributionFails covers AFP-V0-012's global
// fallback: a Go file whose imports do not parse leaves its edges unknown.
func TestSelectPackagesFallsBackWhenAttributionFails(t *testing.T) {
	root := writeFixture(t, map[string]string{"core/core.go": "package core\n", "broken/broken.go": "package broken\n\nimport (\n"})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"docs/x.md"}
	want := `FALLBACK the repository could not be indexed for attribution: "broken/broken.go: imports do not parse"`
	if got := verdict(plan, root); got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// AFP-V0-012: a path literal equal to the root module names the root package,
// so a deleted root source must reach the package that carries that literal.
func TestSelectPackagesLinksRootModuleLiteral(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"root.go":               "package fixture\n",
		"runner/runner_test.go": "package runner\n\nvar target = \"example.com/fixture\"\n",
	})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"gone.go"}
	const want = "run example.com/fixture example.com/fixture/runner"
	if got := verdict(plan, root); got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// V1-0059: a path token held only by a package's `_test.go` file still makes
// that package a direct reader of a dirty path naming it, but must not widen
// the closure to that package's own importers: a test file is never
// imported, so nothing reaches `importer` through `holder`'s test literal.
func TestSelectPackagesStopsClosureAtTestOnlyTokenHolder(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"target/target.go":      "package target\n",
		"holder/holder_test.go": "package holder\n\nvar ref = \"target\"\n",
		"importer/importer.go":  "package importer\n\nimport _ \"example.com/fixture/holder\"\n",
	})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"target/gone.go"}
	const want = "run example.com/fixture/holder example.com/fixture/target"
	if got := verdict(plan, root); got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// AFP-V0-012 (b): `//go:embed` may reach into a subdirectory that is its own
// package, so a data path there must reach the embedding ancestor's dependents.
func TestSelectPackagesReachesEmbeddingAncestorDependents(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"x/x.go":         "package x\n\nimport _ \"embed\"\n\n//go:embed sub/data.txt\nvar Data string\n",
		"x/sub/sub.go":   "package sub\n",
		"x/sub/data.txt": "hi\n",
		"y/y.go":         "package y\n\nimport \"example.com/fixture/x\"\n\nvar D = x.Data\n",
	})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"x/sub/data.txt"}
	const want = "run example.com/fixture/x example.com/fixture/x/sub example.com/fixture/y"
	if got := verdict(plan, root); got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// AFP-V0-012: a root-locator call must be recognized under whatever local
// name the file's own import gave it — an alias or a dot import — not only
// its literal package name, or the package is wrongly treated as bounded.
func TestSelectPackagesResolvesAliasedAndDotRootLocatorImports(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"docs/x.md":                   "x\n",
		"aliased/aliased_test.go":     "package aliased\n\nimport rt \"runtime\"\n\nvar _, file, _, _ = rt.Caller(0)\n",
		"dotimport/dotimport_test.go": "package dotimport\n\nimport . \"runtime\"\n\nvar _, file, _, _ = Caller(0)\n",
	})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"docs/x.md"}
	const want = "run example.com/fixture/aliased example.com/fixture/dotimport"
	if got := verdict(plan, root); got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// AFP-V0-012: go build/test compiles a symlinked .go file like any other, so
// an index that silently skipped it would under-select its dependents. The
// walk must fail closed to the full run instead.
func TestIndexRepositoryFailsClosedOnSymlinkedGoFile(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"leaf/leaf.go": "package leaf\n\nimport _ \"example.com/fixture/core\"\n",
		"outside.go":   "package core\n",
	})
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.go"), filepath.Join(root, "core", "core.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"docs/x.md"}
	got := verdict(plan, root)
	if !strings.HasPrefix(got, "FALLBACK ") || !strings.Contains(got, "core/core.go") {
		t.Fatalf("verdict = %q, want a FALLBACK naming the symlinked file", got)
	}
}

// AFP-V0-011: a package path that merely shares the module's characters as a
// string prefix, without a "/" boundary, names a sibling module and MUST
// fall back rather than being selected to run.
func TestSelectPackagesRejectsSiblingModulePrefix(t *testing.T) {
	root := writeFixture(t, map[string]string{"root.go": "package fixture\n"})
	var plan receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Provider.Go.Packages = []string{fixtureModule + "2/evil"}
	got := verdict(plan, root)
	want := `FALLBACK package path is not a plain import path under the module: "example.com/fixture2/evil"`
	if got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// AFP-V0-011: the selector refuses an oversized external plan before JSON
// decoding instead of allowing the plan file to consume unbounded memory.
func TestSelectorRejectsOversizedPlan(t *testing.T) {
	if os.Getenv("CORVINT_TEST_OVERSIZED_PLAN") == "1" {
		os.Args = []string{"gate-affected-select", os.Getenv("CORVINT_TEST_PLAN"), fixtureModule, os.Getenv("CORVINT_TEST_ROOT")}
		main()
		return
	}
	root := writeFixture(t, map[string]string{"root.go": "package fixture\n"})
	plan := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(plan, []byte(strings.Repeat(" ", (8<<20)+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestSelectorRejectsOversizedPlan$")
	command.Env = append(os.Environ(), "CORVINT_TEST_OVERSIZED_PLAN=1", "CORVINT_TEST_PLAN="+plan, "CORVINT_TEST_ROOT="+root)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("oversized plan succeeded: %s", output)
	}
	if !strings.Contains(string(output), "plan exceeds 8388608 bytes") {
		t.Fatalf("oversized plan did not report its bound: %s", output)
	}
}

// AFP-V0-012: readBounded must refuse when a path classified as a regular Go
// source has become a symlink before open, rather than reading outside root.
func TestReadBoundedRefusesSymlinkSwap(t *testing.T) {
	target := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(target, []byte("package outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "inside.go")
	if err := os.WriteFile(link, []byte("package inside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := readBounded(link, expected); err == nil {
		t.Fatal("readBounded followed a swapped symlink")
	}
}

// TestUnresolvedPackagesListsRootLocators pins the `-unresolved` listing GL-V0-004
// reads: a root-locating package and its dependents are listed with their reasons,
// a package whose reads its literals bound is not (solo names ../core/core.go, a
// path attribution can see), and an unindexable repository is an error, not an
// empty list.
func TestUnresolvedPackagesListsRootLocators(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"core/core.go":      "package core\n",
		"walker/walker.go":  "package walker\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\n",
		"leaf/leaf.go":      "package leaf\n\nimport _ \"example.com/fixture/walker\"\n",
		"solo/solo_test.go": "package solo\n\nvar source = \"../core/core.go\"\n",
	})
	lines, err := unresolvedPackages(fixtureModule, root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "")
	for _, want := range []string{"unresolved example.com/fixture/walker: \"", "unresolved example.com/fixture/leaf: \"depends on example.com/fixture/walker"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, bounded := range []string{"fixture/core", "fixture/solo"} {
		if strings.Contains(got, bounded) {
			t.Errorf("bounded package %s listed:\n%s", bounded, got)
		}
	}
	broken := writeFixture(t, map[string]string{"bad/bad.go": "package bad\n\nimport (\n"})
	if _, err := unresolvedPackages(fixtureModule, broken); err == nil {
		t.Error("an unindexable repository returned a list")
	}
}

// TestPackageBoundsAttributesResolvedPackages pins the `-bounds` mode the gate
// ledger keys resolved packages on (GL-V0-009): every path rules (a) to (c)
// attribute to a package is listed under its first rule, and an unresolved
// package carries its rule (d) reason instead of a bound.
func TestPackageBoundsAttributesResolvedPackages(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"core/core.go":           "package core\n",
		"dep/dep.go":             "package dep\n\nimport _ \"example.com/fixture/core\"\n",
		"reader/reader_test.go":  "package reader\n\nvar guide = \"docs/guide.md\"\n",
		"walker/walker.go":       "package walker\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\n",
		"docs/guide.md":          "guide\n",
		"core/testdata/seed.txt": "seed\n",
	})
	paths := []string{"core/core.go", "core/testdata/seed.txt", "dep/dep.go", "docs/guide.md", "reader/reader_test.go", "walker/walker.go"}
	lines, err := packageBounds(fixtureModule, root, paths)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "")
	want := "bound example.com/fixture/core frontier core/core.go\n" +
		"bound example.com/fixture/core reader core/testdata/seed.txt\n" +
		"bound example.com/fixture/dep frontier core/core.go\n" +
		"bound example.com/fixture/dep frontier dep/dep.go\n" +
		"bound example.com/fixture/reader reader docs/guide.md\n" +
		"bound example.com/fixture/reader frontier reader/reader_test.go\n" +
		"unresolved example.com/fixture/walker: \"walker/walker.go calls runtime.Caller\"\n"
	if got != want {
		t.Errorf("bounds:\n%s\nwant:\n%s", got, want)
	}
	if _, err := packageBounds(fixtureModule, root, []string{"docs/\x01.md"}); err == nil {
		t.Error("a path with a control character was bounded")
	}
}

// TestPathMatcherAgreesWithNamesPath keeps the batched rule (c) matcher the
// `-bounds` mode uses equal to the per-path namesPath relation of readers().
func TestPathMatcherAgreesWithNamesPath(t *testing.T) {
	paths := []string{"docs/guide.md", "docs/specs/gate-ledger-v0.md", "internal/x/testdata/seed.txt", "testdata/seed.txt", "a/b/c/d.go", "docs/decisions/0001.md", ".corvint/change.cem.json"}
	values := []string{"docs/guide.md", "guide.md", "docs/specs/", "specs/gate-ledger-v0.md", "gate-ledger-v0", "testdata/seed.txt", "x/testdata", "seed", "b/c", "/c/d.go", "docs", "0001.md", "decisions/000", ".corvint/change.cem.json", "nothing/here", "", "/"}
	matcher := newPathMatcher(paths)
	for _, value := range values {
		var want []int
		for i, p := range paths {
			if namesPath(value, p) {
				want = append(want, i)
			}
		}
		if got := matcher.named(value); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: matcher %v, namesPath %v", value, got, want)
		}
	}
}
