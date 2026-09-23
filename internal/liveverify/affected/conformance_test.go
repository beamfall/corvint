package affected_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/dotnet"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/kotlin"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
	rubyadapter "github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/liveverify/affected/rust"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// The fixture encodes one shape in eight languages: core <- mid <- leaf, plus an
// unrelated solo unit. Editing core must reach the whole chain and must not
// reach solo, in every language, with the same code path.
type seamCase struct {
	language affected.Language
	// dirty is the source path edited.
	dirty string
	// wantTests is every test file the plan must select.
	wantTests []string
	// wantExcluded is every test file the plan must exclude with a certificate.
	wantExcluded []string
	// solo is the unrelated unit's source path.
	solo string
	// ownedUnindexed is a path this plugin claims but no unit declares.
	ownedUnindexed string
	// permanentFrontier is a seam limitation that makes every plan unknown.
	permanentFrontier string
}

func seamCases() []seamCase {
	return []seamCase{
		{
			language:       golang.New(),
			solo:           "solo/solo.go",
			dirty:          "core/core.go",
			wantTests:      []string{"core/core_test.go", "leaf/leaf_test.go", "mid/mid_test.go"},
			wantExcluded:   []string{"solo/solo_test.go"},
			ownedUnindexed: "core/brand_new.go",
		},
		{
			language:       python.New(),
			solo:           "src/solo.py",
			dirty:          "src/core.py",
			wantTests:      []string{"tests/test_core.py", "tests/test_leaf.py", "tests/test_mid.py"},
			wantExcluded:   []string{"tests/test_solo.py"},
			ownedUnindexed: "src/brand_new.py",
		},
		{
			language:       dotnet.New(),
			solo:           "dotnet/solo/Solo.cs",
			dirty:          "dotnet/core/Core.cs",
			wantTests:      []string{"dotnet/core/CoreTests.cs", "dotnet/leaf/LeafTests.cs", "dotnet/mid/MidTests.cs"},
			wantExcluded:   []string{"dotnet/solo/SoloTests.cs"},
			ownedUnindexed: "dotnet/core/BrandNew.cs",
		},
		{
			language: kotlin.New(),
			solo:     "kotlin/solo/src/main/kotlin/solo/Solo.kt",
			dirty:    "kotlin/core/src/main/kotlin/core/Core.kt",
			wantTests: []string{
				"kotlin/core/src/test/kotlin/core/CoreTest.kt",
				"kotlin/leaf/src/test/kotlin/leaf/LeafTest.kt",
				"kotlin/mid/src/test/kotlin/mid/MidTest.kt",
			},
			wantExcluded:   []string{"kotlin/solo/src/test/kotlin/solo/SoloTest.kt"},
			ownedUnindexed: "kotlin/core/src/main/kotlin/core/BrandNew.kt",
		},
		{
			language:       rubyadapter.New(),
			solo:           "lib/solo.rb",
			dirty:          "lib/core.rb",
			wantTests:      []string{"spec/core_spec.rb", "spec/leaf_spec.rb", "spec/mid_spec.rb"},
			wantExcluded:   []string{"spec/solo_spec.rb"},
			ownedUnindexed: "lib/brand_new.rb",
		},
		{
			language:       rust.New(),
			solo:           "rust/solo/src/lib.rs",
			dirty:          "rust/core/src/lib.rs",
			wantTests:      []string{"rust/core/src/lib.rs", "rust/leaf/tests/leaf.rs", "rust/mid/src/lib.rs"},
			wantExcluded:   []string{"rust/solo/src/lib.rs"},
			ownedUnindexed: "rust/core/src/brand_new.rs",
		},
		{
			language:          swift.New(),
			solo:              "swift/solo/Solo.swift",
			dirty:             "swift/core/Core.swift",
			wantTests:         []string{"swift/tests/CoreTests.swift", "swift/tests/LeafLegacyTests.swift", "swift/tests/LeafModernTests.swift", "swift/tests/MidTests.swift"},
			wantExcluded:      []string{"swift/tests/SoloTests.swift"},
			ownedUnindexed:    "swift/core/BrandNew.swift",
			permanentFrontier: swift.FrontierExclusionContract,
		},
		{
			language:       typescript.New(),
			solo:           "web/solo.mjs",
			dirty:          "web/core.mjs",
			wantTests:      []string{"web/tests/core.test.mjs", "web/tests/leaf.test.mjs", "web/tests/mid.test.mjs"},
			wantExcluded:   []string{"web/tests/solo.test.mjs"},
			ownedUnindexed: "web/brand_new.mjs",
		},
	}
}

// allLanguages is every plugin the seam admits, in Name() order.
func allLanguages() []affected.Language {
	return []affected.Language{
		dotnet.New(), golang.New(), kotlin.New(), python.New(),
		rubyadapter.New(), rust.New(), swift.New(), typescript.New(),
	}
}

// allLanguageNames mirrors allLanguages and is the sorted namespace set a
// composed graph must report.
var allLanguageNames = []string{"dotnet", "go", "kotlin", "python", "ruby", "rust", "swift", "typescript"}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestSeamSelectsTheDependencyClosureInEveryLanguage is the seam's conformance
// suite. It runs one assertion set against every plugin, so a new language is
// admitted by satisfying this test rather than by adding selection code.
func TestSeamSelectsTheDependencyClosureInEveryLanguage(t *testing.T) {
	root := fixtureRoot(t)
	for _, testCase := range seamCases() {
		t.Run(testCase.language.Name(), func(t *testing.T) {
			graph, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			plan := affected.Select(graph, []string{testCase.dirty})
			if got := plan.SelectedTests(); !equal(got, testCase.wantTests) {
				t.Fatalf("selected tests=%v want=%v", got, testCase.wantTests)
			}
			excludedTests := make([]string, 0, len(plan.Excluded))
			for _, exclusion := range plan.Excluded {
				unit, ok := graph.Unit(exclusion.UnitID)
				if !ok {
					t.Fatalf("exclusion names unknown unit %s", exclusion.UnitID)
				}
				if exclusion.Universe != graph.Digest() {
					t.Fatalf("exclusion universe=%s want graph digest", exclusion.Universe)
				}
				if exclusion.Reason == "" || exclusion.Invalidation == "" {
					t.Fatalf("exclusion %s is not a bounded certificate: %+v", exclusion.UnitID, exclusion)
				}
				excludedTests = append(excludedTests, unit.Tests...)
			}
			sort.Strings(excludedTests)
			if !equal(excludedTests, testCase.wantExcluded) {
				t.Fatalf("excluded tests=%v want=%v", excludedTests, testCase.wantExcluded)
			}
		})
	}
}

// TestSeamNamesAWitnessChainThatResolvesInTheGraph proves LPCV-V0-013 for every
// plugin: a selection is only admissible if its witness is machine-checkable.
func TestSeamNamesAWitnessChainThatResolvesInTheGraph(t *testing.T) {
	root := fixtureRoot(t)
	for _, testCase := range seamCases() {
		t.Run(testCase.language.Name(), func(t *testing.T) {
			graph, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{testCase.dirty})
			for _, selection := range plan.Selected {
				witness := selection.Witness
				if witness.DirtyPath != testCase.dirty {
					t.Fatalf("%s witness dirtyPath=%s want=%s", selection.UnitID, witness.DirtyPath, testCase.dirty)
				}
				if len(witness.Via) == 0 || witness.Via[len(witness.Via)-1] != selection.UnitID {
					t.Fatalf("%s witness chain does not end at the unit: %v", selection.UnitID, witness.Via)
				}
				owner, owned := graph.OwnerOf(witness.DirtyPath)
				if !owned || owner != witness.Via[0] {
					t.Fatalf("%s witness chain does not start at the changed unit: %v", selection.UnitID, witness.Via)
				}
				assertChainIsRealDependencyPath(t, graph, witness.Via)
				assertWitnessKind(t, graph, witness, selection.UnitID)
			}
		})
	}
}

func assertChainIsRealDependencyPath(t *testing.T, graph *affected.Graph, via []string) {
	t.Helper()
	for index := 0; index+1 < len(via); index++ {
		importer, ok := graph.Unit(via[index+1])
		if !ok {
			t.Fatalf("witness chain names unknown unit %s", via[index+1])
		}
		if !contains(importer.Imports, via[index]) {
			t.Fatalf("witness chain edge %s -> %s does not exist", via[index], via[index+1])
		}
	}
}

func assertWitnessKind(t *testing.T, graph *affected.Graph, witness affected.Witness, unitID string) {
	t.Helper()
	if len(witness.Via) > 1 {
		if witness.Kind != affected.WitnessDependency {
			t.Fatalf("%s multi-hop witness kind=%s", unitID, witness.Kind)
		}
		return
	}
	unit, _ := graph.Unit(unitID)
	want := affected.WitnessDirectSource
	if contains(unit.Tests, witness.DirtyPath) {
		want = affected.WitnessDirectTest
	}
	if witness.Kind != want {
		t.Fatalf("%s direct witness kind=%s want=%s", unitID, witness.Kind, want)
	}
}

// TestSeamWidensRatherThanNarrowingOnUnknownInput proves LPCV-V0-016 for every
// plugin: an input the graph cannot account for makes scope UNKNOWN instead of
// quietly shrinking the plan.
func TestSeamWidensRatherThanNarrowingOnUnknownInput(t *testing.T) {
	root := fixtureRoot(t)
	for _, testCase := range seamCases() {
		t.Run(testCase.language.Name(), func(t *testing.T) {
			graph, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatal(err)
			}
			baseline := affected.Select(graph, []string{testCase.dirty})
			if testCase.permanentFrontier == "" && baseline.Scope != affected.ScopeBounded {
				t.Fatalf("clean fixture scope=%s unknown=%+v", baseline.Scope, baseline.Unknown)
			}
			if testCase.permanentFrontier != "" {
				assertWidened(t, baseline, affected.UnknownLanguageFrontier, testCase.permanentFrontier)
			}
			unowned := affected.Select(graph, []string{testCase.dirty, "README.md"})
			assertWidened(t, unowned, affected.UnknownUnownedDirtyPath, "README.md")
			unindexed := affected.Select(graph, []string{testCase.dirty, testCase.ownedUnindexed})
			assertWidened(t, unindexed, affected.UnknownUnindexedSourcePath, testCase.ownedUnindexed)
			// Widening never removes a check that the narrower plan required.
			if !isSuperset(unowned.SelectedTests(), baseline.SelectedTests()) {
				t.Fatalf("widening dropped a required check: %v vs %v", unowned.SelectedTests(), baseline.SelectedTests())
			}
		})
	}
}

func assertWidened(t *testing.T, plan affected.Plan, reason, detail string) {
	t.Helper()
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s want UNKNOWN for %s", plan.Scope, detail)
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == reason && unknown.Detail == detail {
			return
		}
	}
	t.Fatalf("plan does not name %s for %s: %+v", reason, detail, plan.Unknown)
}

// TestSeamIsDeterministicAndCanonical proves LPCV-V0-011/019 for every plugin:
// a rebuild from repository authority reproduces the graph and the plan byte
// for byte.
func TestSeamIsDeterministicAndCanonical(t *testing.T) {
	root := fixtureRoot(t)
	for _, testCase := range seamCases() {
		t.Run(testCase.language.Name(), func(t *testing.T) {
			first, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatal(err)
			}
			second, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatal(err)
			}
			if first.Digest() != second.Digest() {
				t.Fatalf("graph digest drift %s vs %s", first.Digest(), second.Digest())
			}
			firstBody, err := affected.Select(first, []string{testCase.dirty}).Canonical()
			if err != nil {
				t.Fatal(err)
			}
			// Dirty order must not matter.
			secondBody, err := affected.Select(second, []string{testCase.dirty, testCase.dirty}).Canonical()
			if err != nil {
				t.Fatal(err)
			}
			if string(firstBody) != string(secondBody) {
				t.Fatalf("plan bytes drift:\n%s\n%s", firstBody, secondBody)
			}
		})
	}
}

// TestSeamNamespacesEveryIdentity proves LPCV-V0-022's namespacing rule: no two
// plugins can collide, so a composed graph is unambiguous.
func TestSeamNamespacesEveryIdentity(t *testing.T) {
	root := fixtureRoot(t)
	graph, err := affected.Build(root, allLanguages()...)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if got := graph.Languages(); !equal(got, allLanguageNames) {
		t.Fatalf("languages=%v", got)
	}
	counts := map[string]int{}
	for _, id := range graph.UnitIDs() {
		namespace, _, ok := strings.Cut(id, ":")
		if !ok {
			t.Fatalf("unit %s has no namespace", id)
		}
		counts[namespace]++
	}
	for _, namespace := range allLanguageNames {
		if counts[namespace] == 0 {
			t.Fatalf("composed graph missing %s: %v", namespace, counts)
		}
	}
}

// TestComposedGraphKeepsLanguagesIndependent proves that adding a language
// cannot change another language's selection. This is the property that makes
// the seam safe to extend.
func TestComposedGraphKeepsLanguagesIndependent(t *testing.T) {
	root := fixtureRoot(t)
	composed, err := affected.Build(root, allLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range seamCases() {
		single, err := affected.Build(root, testCase.language)
		if err != nil {
			t.Fatal(err)
		}
		alone := affected.Select(single, []string{testCase.dirty}).SelectedTests()
		together := affected.Select(composed, []string{testCase.dirty}).SelectedTests()
		if !equal(alone, together) {
			t.Fatalf("%s selection changed when composed: %v vs %v", testCase.language.Name(), alone, together)
		}
	}
}

func equal(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isSuperset(superset, subset []string) bool {
	for _, value := range subset {
		if !contains(superset, value) {
			return false
		}
	}
	return true
}

// TestSeamWidensWhenNoTestReachesAChangedUnit proves AFP-V0-020 for every
// plugin. With solo's tests removed, an edit to solo reaches no test and widens
// the plan; an edit to core leaves the untested solo unchanged and stays bounded.
func TestSeamWidensWhenNoTestReachesAChangedUnit_AFPV0020(t *testing.T) {
	for _, testCase := range seamCases() {
		t.Run(testCase.language.Name(), func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(fixtureRoot(t))); err != nil {
				t.Fatal(err)
			}
			for _, test := range testCase.wantExcluded {
				untestSolo(t, filepath.Join(root, filepath.FromSlash(test)), test == testCase.solo)
			}
			if project, ok := plainSoloProjects[testCase.language.Name()]; ok {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(project)), []byte(`<Project Sdk="Microsoft.NET.Sdk" />`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			graph, err := affected.Build(root, testCase.language)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			owner, _ := graph.OwnerOf(testCase.solo)
			changed := affected.Select(graph, []string{testCase.solo})
			if named := noSelectableTest(changed); changed.Scope != affected.ScopeUnknown || len(named) != 1 || named[0] != owner {
				t.Fatalf("changed untested unit %s scope=%s unknown=%v", owner, changed.Scope, changed.Unknown)
			}
			unchanged := affected.Select(graph, []string{testCase.dirty})
			if len(noSelectableTest(unchanged)) != 0 || (testCase.permanentFrontier == "" && unchanged.Scope != affected.ScopeBounded) {
				t.Fatalf("unchanged untested unit scope=%s unknown=%v", unchanged.Scope, unchanged.Unknown)
			}
		})
	}
}

// plainSoloProjects names a solo test project that would otherwise declare
// test discovery with no test source left.
var plainSoloProjects = map[string]string{"dotnet": "dotnet/solo/Solo.csproj"}

// untestSolo removes a test file, or strips the test attribute from a source
// that carries its own test, as Rust's solo crate does.
func untestSolo(t *testing.T, path string, inSource bool) {
	t.Helper()
	if !inSource {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(content), "#[test]\n", "")), 0o600); err != nil {
		t.Fatal(err)
	}
}

// noSelectableTest returns the units a plan names as having no selectable test.
func noSelectableTest(plan affected.Plan) []string {
	var named []string
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownNoSelectableTest {
			named = append(named, unknown.Detail)
		}
	}
	return named
}
