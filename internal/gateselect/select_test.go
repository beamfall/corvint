package gateselect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/frontier"
)

const fixtureModule = "example.com/fixture"

func writeFixture(t testing.TB, files map[string]string) string {
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

func verdict(plan Receipt, root string) string {
	output := strings.Join(selectPackages(plan, fixtureModule, root, true), "\n")
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
		"walker/walker_test.go":   "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"emb/emb.go":              "package emb\n\nimport _ \"embed\"\n\n//go:embed emb_test.go\nvar Source string\n",
		"user/user.go":            "package user\n\nimport _ \"example.com/fixture/emb\"\n",
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
		{"a changed test file selects only its own package (AFP-V0-154)", nil, "core/core_test.go", "run example.com/fixture/core example.com/fixture/walker"},
		{"a deleted test file selects only its own package (AFP-V0-154)", nil, "core/gone_test.go", "run example.com/fixture/core example.com/fixture/walker"},
		{"a test file in an embedding package still widens to its importers (AFP-V0-154)", nil, "emb/emb_test.go", "run example.com/fixture/emb example.com/fixture/user example.com/fixture/walker"},
		{"a test file outside any indexed package still widens", nil, "fresh/fresh_test.go", "run example.com/fixture/walker"},
		{"a Go file read as data selects its reader", []string{"example.com/fixture/core", "example.com/fixture/leaf", "example.com/fixture/mid"}, "core/core.go", "run example.com/fixture/core example.com/fixture/leaf example.com/fixture/mid example.com/fixture/solo example.com/fixture/walker"},
		{"a document selects the packages that name it", nil, "docs/specs/x.md", "run example.com/fixture/spec example.com/fixture/walker"},
		{"a testdata fixture selects its enclosing package", nil, "core/testdata/x.txt", "run example.com/fixture/core example.com/fixture/walker"},
		{"a control character in a dirty path falls back", nil, "evil\nrun touch PWNED/b.go", "FALLBACK a dirty path contains a control character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var plan Receipt
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
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			if got := verdict(plan, root); got != tc.verdict {
				t.Fatalf("verdict = %q, want %q", got, tc.verdict)
			}
		})
	}
}

// TestSelectPackagesNarrowsSealedChangeEvidenceReaders covers AFP-V0-152: a
// sealed CEM directly under .corvint/changes/ selects only the readers whose
// literal resolves to it or an ancestor, as the sidecar does, and the
// -bounds attribution agrees; another path there keeps the component-run rule.
func TestSelectPackagesNarrowsSealedChangeEvidenceReaders(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"fixture/fixture_test.go": "package fixture\n\nvar a, b = \".corvint/changes\", \".corvint/changes/\"\n",
		"exact/exact_test.go":     "package exact\n\nvar sealed = \"../.corvint/changes\"\n",
		"anchored/anchored.go":    "package anchored\n\nfunc Sealed(root string) string { return root + \"/.corvint/changes/\" }\n",
		"deep/dir/dir_test.go":    "package dir\n\nvar evidence = \"../../.corvint\"\n",
		// AFP-V0-931 (V1-3146): the module path is a bare `/` anchor, and a
		// lone `.corvint` beside it names a directory component only, so it
		// is no reader of a sealed CEM or the sidecar, as in the planner.
		"modroot/modroot_test.go": "package modroot\n\nvar module, dir = \"example.com/fixture\", \".corvint\"\n",
	})
	sealed := ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	cases := []struct{ name, dirty, verdict string }{
		{name: "sealed CEM selects only resolving readers", dirty: sealed, verdict: "run example.com/fixture/anchored example.com/fixture/deep/dir example.com/fixture/exact"},
		{name: "AFP-V0-931 sidecar ignores a lone component beside a module anchor", dirty: ".corvint/change.cem.json", verdict: "run example.com/fixture/deep/dir"},
		{name: "other file keeps component-run readers", dirty: ".corvint/changes/x/y.cem.json", verdict: "run example.com/fixture/anchored example.com/fixture/deep/dir example.com/fixture/exact example.com/fixture/fixture example.com/fixture/modroot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			if got := verdict(plan, root); got != tc.verdict {
				t.Fatalf("verdict = %q, want %q", got, tc.verdict)
			}
		})
	}
	lines, err := packageBounds("example.com/fixture", root, []string{sealed})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(lines, ""); strings.Contains(got, "example.com/fixture/fixture ") || !strings.Contains(got, "bound example.com/fixture/exact reader "+sealed) {
		t.Fatalf("bounds disagree with selection:\n%s", got)
	}
}

// TestSelectPackagesTaskStateOnlyDiffSelectsNothing covers AFP-V0-153
// (V1-2070): a BOUNDED plan whose every dirty path is a classified Tasks store
// projection record selects nothing in the fast tier; an unclassified, unknown
// or out-of-layout case keeps the fallback, and a package whose literal names
// a projection record is still selected.
func TestSelectPackagesTaskStateOnlyDiffSelectsNothing(t *testing.T) {
	plain := writeFixture(t, map[string]string{"core/core.go": "package core\n"})
	reading := writeFixture(t, map[string]string{
		"core/core.go":        "package core\n",
		"store/store_test.go": "package store\n\nvar queue = \"../.taskman/queue.json\"\n",
	})
	queue, ticket := ".taskman/queue.json", ".taskman/tickets/V1-0001.json"
	plan := func(scope string, classified []string, dirty ...string) Receipt {
		var receipt Receipt
		receipt.Provider.Go.State = "EMPTY_SELECTION"
		receipt.Plan.Scope = scope
		receipt.Plan.Dirty = dirty
		if classified != nil {
			receipt.Plan.NonSource = append(receipt.Plan.NonSource, struct {
				Class string   `json:"class"`
				Paths []string `json:"paths"`
			}{Class: "TASK_STORE_PROJECTION", Paths: classified})
		}
		return receipt
	}
	cases := []struct {
		name, root string
		plan       Receipt
		verdict    string
	}{
		{"classified task state selects nothing", plain, plan("BOUNDED", []string{queue, ticket}, queue, ticket), "NOTHING 2 dirty path(s) are Tasks store projection task state; no Go package names or reads them"},
		{"unclassified path keeps the fallback", plain, plan("BOUNDED", []string{queue}, queue, ticket), "FALLBACK the selection is empty while 2 path(s) are dirty"},
		{"no nonSource member keeps the fallback", plain, plan("BOUNDED", nil, queue), "FALLBACK the selection is empty while 1 path(s) are dirty"},
		{"unknown plan keeps the fallback", plain, plan("UNKNOWN", []string{queue}, queue), "FALLBACK the plan is UNKNOWN but names no unknown"},
		{"out-of-layout path keeps the fallback", plain, plan("BOUNDED", []string{".taskman/notes.txt"}, ".taskman/notes.txt"), "FALLBACK the selection is empty while 1 path(s) are dirty"},
		{"a reader of the record is still selected", reading, plan("BOUNDED", []string{queue}, queue), "run example.com/fixture/store"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := verdict(tc.plan, tc.root); got != tc.verdict {
				t.Fatalf("verdict = %q, want %q", got, tc.verdict)
			}
		})
	}
}

// TestSelectPackagesFallsBackWhenAttributionFails covers AFP-V0-012's global
// fallback: a Go file whose imports do not parse leaves its edges unknown.
func TestSelectPackagesFallsBackWhenAttributionFails(t *testing.T) {
	root := writeFixture(t, map[string]string{"core/core.go": "package core\n", "broken/broken.go": "package broken\n\nimport (\n"})
	var plan Receipt
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
	var plan Receipt
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
	var plan Receipt
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
	var plan Receipt
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
// runtime.Caller, under any name, reads frame metadata and is not a root
// locator (AFP-V0-810), so `framer` is not selected.
func TestSelectPackagesResolvesAliasedAndDotRootLocatorImports(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"docs/x.md":                   "x\n",
		"aliased/aliased_test.go":     "package aliased\n\nimport wd \"os\"\n\nvar _, _ = wd.Getwd()\n",
		"dotimport/dotimport_test.go": "package dotimport\n\nimport . \"os\"\n\nvar _, _ = Getwd()\n",
		"framer/framer_test.go":       "package framer\n\nimport rt \"runtime\"\n\nvar _, file, _, _ = rt.Caller(0)\n",
		"framedot/framedot_test.go":   "package framedot\n\nimport . \"runtime\"\n\nvar _, file, _, _ = Caller(0)\n",
	})
	var plan Receipt
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
	var plan Receipt
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
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Provider.Go.Packages = []string{fixtureModule + "2/evil"}
	got := verdict(plan, root)
	want := `FALLBACK package path is not a plain import path under the module: "example.com/fixture2/evil"`
	if got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
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
		"walker/walker.go":  "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
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
		"walker/walker.go":       "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
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
		"unresolved example.com/fixture/walker: \"walker/walker.go calls os.Getwd\"\n"
	if got != want {
		t.Errorf("bounds:\n%s\nwant:\n%s", got, want)
	}
	if _, err := packageBounds(fixtureModule, root, []string{"docs/\x01.md"}); err == nil {
		t.Error("a path with a control character was bounded")
	}
}

// TestDeclaredPackagesAreBoundByTheirScope covers GL-V0-050 (V1-0634): the
// -unresolved and -bounds listings treat a declared root locator as the
// selector does, bound by its declaration, subtree and declared entries; an
// undeclared locator stays unresolved and an invalid declaration is an error.
func TestDeclaredPackagesAreBoundByTheirScope(t *testing.T) {
	files := map[string]string{
		"walker/walker.go":               "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"roamer/roamer.go":               "package roamer\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"docs/guide.md":                  "guide\n",
		"notes/other.md":                 "other\n",
		".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"walker":["docs/"]}}` + "\n",
	}
	root := writeFixture(t, files)
	unresolved, err := unresolvedPackages(fixtureModule, root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(unresolved, ""), "unresolved example.com/fixture/roamer: \"roamer/roamer.go calls os.Getwd\"\n"; got != want {
		t.Errorf("unresolved:\n%s\nwant:\n%s", got, want)
	}
	paths := []string{".corvint/test-read-scopes.json", "docs/guide.md", "notes/other.md", "roamer/roamer.go", "walker/walker.go"}
	lines, err := packageBounds(fixtureModule, root, paths)
	if err != nil {
		t.Fatal(err)
	}
	want := "unresolved example.com/fixture/roamer: \"roamer/roamer.go calls os.Getwd\"\n" +
		"bound example.com/fixture/walker declared .corvint/test-read-scopes.json\n" +
		"bound example.com/fixture/walker declared docs/guide.md\n" +
		"bound example.com/fixture/walker frontier walker/walker.go\n"
	if got := strings.Join(lines, ""); got != want {
		t.Errorf("bounds:\n%s\nwant:\n%s", got, want)
	}
	files[".corvint/test-read-scopes.json"] = "{}\n"
	invalid := writeFixture(t, files)
	if _, err := unresolvedPackages(fixtureModule, invalid); err == nil {
		t.Error("an invalid declaration listed the unresolved packages")
	}
	if _, err := packageBounds(fixtureModule, invalid, paths); err == nil {
		t.Error("an invalid declaration bounded the packages")
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

// TestPathReadersAgreesWithReaders keeps Select's batched rule (c) equal to
// readers() per dirty path (V1-1879).
func TestPathReadersAgreesWithReaders(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"core/core.go":          "package core\n\nvar seed = \"testdata/seed.txt\"\n",
		"reader/reader_test.go": "package reader\n\nvar guide, specs = \"docs/guide.md\", \"../docs/specs/\"\n",
		"spec/spec.go":          "package spec\n\nvar ledger, partial = \"gate-ledger-v0.md\", \"decisions/000\"\n",
		"cem/cem.go":            "package cem\n\nvar sidecar = \".corvint/change.cem.json\"\n",
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{".corvint/change.cem.json", "core/testdata/seed.txt", "docs/decisions/0001.md", "docs/guide.md", "docs/specs/gate-ledger-v0.md", "testdata/seed.txt", "unrelated/x.go"}
	got := index.pathReaders(paths)
	matched := 0
	for i, p := range paths {
		want := index.readers(p)
		if fmt.Sprint(got[i]) != fmt.Sprint(want) {
			t.Errorf("%s: pathReaders %v, readers %v", p, got[i], want)
		}
		matched += len(want)
	}
	if matched == 0 {
		t.Fatal("fixture names no reader; the comparison proves nothing")
	}
}

// BenchmarkPathReaders measures rule (c) attribution over many dirty paths,
// batched against per path (V1-1879).
func BenchmarkPathReaders(b *testing.B) {
	files := map[string]string{}
	var paths []string
	for i := range 200 {
		files[fmt.Sprintf("p%03d/p.go", i)] = fmt.Sprintf("package p\n\nvar a, b, c = \"docs/specs/s%03d.md\", \"testdata/%03d.txt\", \"x/y%03d\"\n", i, i, i)
	}
	for i := range 600 {
		paths = append(paths, fmt.Sprintf("docs/specs/s%03d.md", i%300), fmt.Sprintf("p%03d/testdata/%03d.txt", i%200, i))
	}
	root := writeFixture(b, files)
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("batched", func(b *testing.B) {
		for b.Loop() {
			index.pathReaders(paths)
		}
	})
	b.Run("per-path", func(b *testing.B) {
		for b.Loop() {
			for _, p := range paths {
				index.readers(p)
			}
		}
	})
}

// TestSelectStructuralLeavesRuleDToThePlan_STRV0061 pins STR-V0-061: Select
// still adds this index's own rule (d) closure, which reaches `tester` through
// its test-only import of `leaf` and stops there, as the planner does, because
// `far`'s test binary does not link the root locator (V1-2727), while
// SelectStructural attributes only rules (a) to (c) and the declarations,
// leaving rule (d) to the plan's own `UNBOUNDED_READER` selections, which it
// keeps as `selected` lines.
func TestSelectStructuralLeavesRuleDToThePlan_STRV0061(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"core/core.go":          "package core\n",
		"mid/mid.go":            "package mid\n\nimport _ \"example.com/fixture/core\"\n",
		"walker/walker.go":      "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"leaf/leaf.go":          "package leaf\n\nimport _ \"example.com/fixture/walker\"\n",
		"tester/tester.go":      "package tester\n",
		"tester/tester_test.go": "package tester\n\nimport _ \"example.com/fixture/leaf\"\n",
		"far/far.go":            "package far\n\nimport _ \"example.com/fixture/tester\"\n",
	})
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Provider.Go.Packages = []string{"example.com/fixture/walker"}
	plan.Plan.Dirty = []string{"core/core.go"}
	full := strings.Join(Select(plan, fixtureModule, root), "\n")
	for _, want := range []string{"unresolved example.com/fixture/leaf: ", "unresolved example.com/fixture/tester: "} {
		if !strings.Contains(full, want) {
			t.Errorf("Select lacks %q in:\n%s", want, full)
		}
	}
	if strings.Contains(full, "example.com/fixture/far") {
		t.Errorf("Select follows tester's test-only import past tester:\n%s", full)
	}
	lines := SelectStructural(plan, fixtureModule, root)
	got := strings.Join(lines, "\n")
	if strings.Contains(got, "unresolved ") {
		t.Errorf("SelectStructural added a rule (d) line:\n%s", got)
	}
	if want := "run example.com/fixture/core example.com/fixture/mid example.com/fixture/walker"; lines[len(lines)-1] != want {
		t.Errorf("verdict = %q, want %q in:\n%s", lines[len(lines)-1], want, got)
	}
	if !strings.Contains(got, "selected example.com/fixture/walker") {
		t.Errorf("the plan's own rule (d) selection is not kept:\n%s", got)
	}
}
