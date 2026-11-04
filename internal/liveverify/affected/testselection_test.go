// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// contractsOwner is the V1-2662 example: a change to a contract document
// selects the TypeScript spec that checks the contracts.
const contractsOwner = `{"profile":"corvint-test-selection/0","owners":[{"paths":"docs/contracts/**/*.json","tests":["tests/contracts.spec.ts"]},{"paths":"gen/**","tests":["tests/contracts.spec.ts"]}]}`

// smokeMustRun pins the smoke package, which no change reaches through code.
const smokeMustRun = `{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/","reason":"release smoke suite"}]}`

// testSelectionLanguages is a g plugin (an api package, a command importing
// it, and a smoke package nothing imports) beside a ts plugin whose spec reads
// the contract documents at run time, an edge no plugin models.
func testSelectionLanguages() []Language {
	return []Language{
		fake{name: "g", units: []Unit{
			{ID: "g:api", Sources: []string{"api.g"}, Tests: []string{"api_test.g"}},
			{ID: "g:cmd", Sources: []string{"cmd.g"}, Tests: []string{"cmd_test.g"}, Imports: []string{"g:api"}},
			{ID: "g:smoke", Sources: []string{"smoke/smoke.g"}, Tests: []string{"smoke/smoke_test.g"}},
		}},
		fake{name: "ts", units: []Unit{
			{ID: "ts:contracts", Tests: []string{"tests/contracts.spec.ts"}},
		}},
	}
}

// testSelectionRoot returns a root holding each declaration path with its
// body, every unit file, and nothing else.
func testSelectionRoot(t *testing.T, declarations map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{}
	for _, language := range testSelectionLanguages() {
		for _, unit := range language.(fake).units {
			for _, path := range append(slices.Clone(unit.Sources), unit.Tests...) {
				files[path] = "x"
			}
		}
	}
	for path, body := range declarations {
		files[path] = body
	}
	for path, body := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func testSelectionPlan(t *testing.T, declaration string, dirty ...string) Plan {
	t.Helper()
	graph, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: declaration}), testSelectionLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	return Select(graph, dirty)
}

func unknownOf(plan Plan, reason string) []string {
	var details []string
	for _, unknown := range plan.Unknown {
		if unknown.Reason == reason {
			details = append(details, unknown.Detail)
		}
	}
	return details
}

func nonSourceOf(plan Plan, class string) []string {
	for _, group := range plan.NonSource {
		if group.Class == class {
			return group.Paths
		}
	}
	return nil
}

// AFP-V0-2480: the declaration is closed, bounded and strictly ordered; an
// invalid one refuses the build naming the declaration and the member; an
// absent or empty one leaves the graph digest unchanged, a present one joins
// the digest and the cache entry, and the declaration path alone is a
// CORVINT_DECLARATION change.
func TestTestSelectionDeclarationGrammar_AFPV02480(t *testing.T) {
	long := strings.Repeat("r", maxMustRunReasonBytes+1)
	tooMany := `"a.ts","b.ts","c.ts","d.ts","e.ts","f.ts","g.ts","h.ts","i.ts","j.ts","k.ts","l.ts","m.ts","n.ts","o.ts","p.ts","q.ts"`
	for name, c := range map[string]struct{ body, want string }{
		"wrong profile":        {`{"profile":"corvint-test-selection/1"}`, "profile must be"},
		"unknown member":       {`{"profile":"corvint-test-selection/0","x":1}`, DeclaredSelectionPath},
		"owners descending":    {`{"profile":"corvint-test-selection/0","owners":[{"paths":"z/**","tests":["a.ts"]},{"paths":"a/**","tests":["a.ts"]}]}`, `owners paths "a/**"`},
		"owners duplicate":     {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/**","tests":["a.ts"]},{"paths":"a/**","tests":["b.ts"]}]}`, `owners paths "a/**"`},
		"tests descending":     {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/**","tests":["b.ts","a.ts"]}]}`, `tests "a.ts"`},
		"no tests":             {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/**","tests":[]}]}`, "must name 1 to 16 tests"},
		"seventeen tests":      {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/**","tests":[` + tooMany + `]}]}`, "must name 1 to 16 tests"},
		"test under .corvint":  {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":".corvint/x","reason":"r"}]}`, `mustRun tests ".corvint/x"`},
		"test escaping":        {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/**","tests":["../a.ts"]}]}`, `tests "../a.ts"`},
		"invalid pattern":      {`{"profile":"corvint-test-selection/0","owners":[{"paths":"a/[bc]","tests":["a.ts"]}]}`, `owners paths "a/[bc]"`},
		"mustRun descending":   {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":"z/","reason":"r"},{"tests":"a/","reason":"r"}]}`, `mustRun tests "a/"`},
		"blank reason":         {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/","reason":""}]}`, "reason must be"},
		"control in reason":    {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/","reason":"a\u0007b"}]}`, "reason must be"},
		"oversized reason":     {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/","reason":"` + long + `"}]}`, "reason must be"},
		"missing reason field": {`{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/"}]}`, "reason must be"},
	} {
		_, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: c.body}), testSelectionLanguages()...)
		if err == nil || !strings.Contains(err.Error(), DeclaredSelectionPath) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err=%v, want it to name %s and %q", name, err, DeclaredSelectionPath, c.want)
		}
	}
	oversized := `{"profile":"corvint-test-selection/0","mustRun":[{"tests":"smoke/","reason":"` + strings.Repeat(" ", maxDeclaredSelectionBytes) + `r"}]}`
	if _, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: oversized}), testSelectionLanguages()...); err == nil {
		t.Error("a declaration over 64 KiB was accepted")
	}

	digest := func(declarations map[string]string) string {
		t.Helper()
		graph, err := Build(context.Background(), testSelectionRoot(t, declarations), testSelectionLanguages()...)
		if err != nil {
			t.Fatal(err)
		}
		return graph.Digest()
	}
	absent := digest(nil)
	if empty := digest(map[string]string{DeclaredSelectionPath: `{"profile":"corvint-test-selection/0"}`}); empty != absent {
		t.Errorf("an empty declaration changed the digest: %s != %s", empty, absent)
	}
	if present := digest(map[string]string{DeclaredSelectionPath: smokeMustRun}); present == absent {
		t.Error("a present declaration left the digest unchanged")
	}

	// The cache entry carries the declaration: an entry that survives its
	// own encoding restores the same digest and must-run units, and one
	// without the declaration does not.
	built, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: smokeMustRun}), testSelectionLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	entry := cacheEntry{TestSelection: built.testSelection}
	for _, language := range testSelectionLanguages() {
		cached := cachedLanguage{Name: language.Name()}
		for _, unit := range language.(fake).units {
			cached.Units = append(cached.Units, cachedUnit(unit))
		}
		entry.Languages = append(entry.Languages, cached)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var decoded cacheEntry
	if err := json.Unmarshal(encoded, &decoded, json.RejectUnknownMembers(true)); err != nil {
		t.Fatal(err)
	}
	restored, err := restoreGraph(context.Background(), decoded, testSelectionLanguages())
	if err != nil {
		t.Fatal(err)
	}
	if restored.Digest() != built.Digest() || !reflect.DeepEqual(restored.MustRunUnits(), built.MustRunUnits()) || len(built.MustRunUnits()) != 1 {
		t.Fatalf("restored must-run %+v digest %s, built %+v digest %s", restored.MustRunUnits(), restored.Digest(), built.MustRunUnits(), built.Digest())
	}
	decoded.TestSelection = nil
	if bare, err := restoreGraph(context.Background(), decoded, testSelectionLanguages()); err != nil || bare.Digest() == built.Digest() || bare.MustRunUnits() != nil {
		t.Fatalf("an entry without the declaration restored it: err=%v", err)
	}

	// The declaration alone is a Corvint declaration change (AFP-V0-443).
	plan := testSelectionPlan(t, smokeMustRun, DeclaredSelectionPath)
	if got := nonSourceOf(plan, ClassCorvintDeclaration); !reflect.DeepEqual(got, []string{DeclaredSelectionPath}) {
		t.Errorf("declaration change: nonSource=%+v unknown=%+v", plan.NonSource, plan.Unknown)
	}
}

// AFP-V0-2481: the owner pattern grammar and its whole-path, case-sensitive
// match; no pattern matches `.git` or `.corvint`.
func TestOwnerPatternGrammar_AFPV02481(t *testing.T) {
	for _, pattern := range []string{"a", "docs/contracts/**/*.json", "**", "**/x", "a/*/c", "?.md", "a/**/b/**/c/**/d/**"} {
		if !validOwnerPattern(pattern) {
			t.Errorf("valid pattern %q refused", pattern)
		}
	}
	for _, pattern := range []string{"", "/a", "a/", "a//b", "./a", "a/./b", "a/../b", "..", "a/**b", "a/***", "a/[ab]", "a/{x,y}", `a\b`, "**/**/**/**/**", strings.Repeat("a", maxOwnerPatternBytes+1), "a/\x01"} {
		if validOwnerPattern(pattern) {
			t.Errorf("invalid pattern %q accepted", pattern)
		}
	}
	for _, c := range []struct {
		pattern, path string
		want          bool
	}{
		{"docs/contracts/**/*.json", "docs/contracts/a.json", true},
		{"docs/contracts/**/*.json", "docs/contracts/v1/a.json", true},
		{"docs/contracts/**/*.json", "docs/contracts/v1/x/a.json", true},
		{"docs/contracts/**/*.json", "docs/contracts/a.yaml", false},
		{"docs/contracts/**/*.json", "docs/contractsX/a.json", false},
		{"docs/contracts/**/*.json", "docs/contracts", false},
		{"docs/contracts/**/*.json", "x/docs/contracts/a.json", false},
		{"**/x", "x", true},
		{"**/x", "a/b/x", true},
		{"**/x", "a/b/xy", false},
		{"a/*/c", "a/b/c", true},
		{"a/*/c", "a/b/d/c", false},
		{"?.md", "a.md", true},
		{"?.md", "ab.md", false},
		{"README.md", "readme.md", false},
		{"**", "a/b", true},
		{"**", ".corvint/test-selection.json", false},
		{"**", ".git", false},
		{".corvint/*", ".corvint/x", false},
		{"**/*", ".git/config", false},
	} {
		if got := ownerPatternMatches(c.pattern, c.path); got != c.want {
			t.Errorf("ownerPatternMatches(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

// AFP-V0-2482 (V1-2662): a dirty path a declared pattern matches selects the
// owner's tests with a terminal DECLARED_TEST_OWNER witness and moves the path
// from unknown to one DECLARED_TEST_OWNER group. A path no pattern matches
// stays an unowned unknown, and an unindexed source path is never removed.
func TestDeclaredOwnerSelectsItsTests_AFPV02482(t *testing.T) {
	const contract = "docs/contracts/v1/order.json"
	plan := testSelectionPlan(t, contractsOwner, contract)
	want := Witness{Kind: WitnessDeclaredTestOwner, DirtyPath: contract, Via: []string{"ts:contracts"}, Glob: "docs/contracts/**/*.json"}
	if got := witnesses(plan); len(got) != 1 || !reflect.DeepEqual(got["ts:contracts"], want) {
		t.Fatalf("selected=%+v, want only ts:contracts by %+v", plan.Selected, want)
	}
	if got := nonSourceOf(plan, ClassDeclaredTestOwner); !reflect.DeepEqual(got, []string{contract}) || plan.Scope != ScopeBounded {
		t.Fatalf("scope=%s nonSource=%+v unknown=%+v", plan.Scope, plan.NonSource, plan.Unknown)
	}

	// Negative control: the same change without the declaration is unowned.
	plan = testSelectionPlan(t, `{"profile":"corvint-test-selection/0"}`, contract)
	if len(plan.Selected) != 0 || !reflect.DeepEqual(unknownOf(plan, UnknownUnownedDirtyPath), []string{contract}) {
		t.Fatalf("undeclared: selected=%+v unknown=%+v", plan.Selected, plan.Unknown)
	}

	// Negative control: a path the pattern does not match stays unowned.
	plan = testSelectionPlan(t, contractsOwner, "docs/other/order.json")
	if len(plan.Selected) != 0 || plan.Scope != ScopeUnknown || !reflect.DeepEqual(unknownOf(plan, UnknownUnownedDirtyPath), []string{"docs/other/order.json"}) {
		t.Fatalf("unmatched: scope=%s selected=%+v unknown=%+v", plan.Scope, plan.Selected, plan.Unknown)
	}

	// Negative control: a path a plugin claims but indexes in no unit keeps
	// its UNINDEXED_SOURCE_PATH unknown although an owner matches it.
	plan = testSelectionPlan(t, contractsOwner, "gen/new.g")
	if _, ok := witnesses(plan)["ts:contracts"]; !ok || plan.Scope != ScopeUnknown || !reflect.DeepEqual(unknownOf(plan, UnknownUnindexedSourcePath), []string{"gen/new.g"}) || nonSourceOf(plan, ClassDeclaredTestOwner) != nil {
		t.Fatalf("unindexed: scope=%s selected=%+v unknown=%+v nonSource=%+v", plan.Scope, plan.Selected, plan.Unknown, plan.NonSource)
	}

	// An owner never replaces a unit's own witness.
	plan = testSelectionPlan(t, `{"profile":"corvint-test-selection/0","owners":[{"paths":"*.g","tests":["api_test.g","tests/contracts.spec.ts"]}]}`, "api.g")
	if got := witnesses(plan); got["g:api"].Kind != WitnessDirectSource || got["ts:contracts"].Kind != WitnessDeclaredTestOwner {
		t.Fatalf("selected=%+v", plan.Selected)
	}
	// AFP-V0-2489: plan.declaredOwners names every owner-selected unit,
	// the one reached first by its own source included.
	wantOwners := []DeclaredOwner{{UnitID: "g:api", Paths: "*.g", DirtyPath: "api.g"}, {UnitID: "ts:contracts", Paths: "*.g", DirtyPath: "api.g"}}
	if !reflect.DeepEqual(plan.DeclaredOwners, wantOwners) {
		t.Fatalf("declaredOwners=%+v, want %+v", plan.DeclaredOwners, wantOwners)
	}
	// Negative control: no matching owner, no member.
	if plan = testSelectionPlan(t, contractsOwner, "api.g"); plan.DeclaredOwners != nil {
		t.Fatalf("unmatched declaredOwners=%+v", plan.DeclaredOwners)
	}
}

// AFP-V0-2488: an owner declaration only adds. An owner-declared path still
// counts as owned by no plugin for the frontier rules, so every plugin's
// LANGUAGE_FRONTIER entry the unowned path raised without the declaration is
// still raised with it, no frontier moves to frontierExcluded, and the plan
// stays UNKNOWN; only its UNOWNED_DIRTY_PATH entry turns into the
// DECLARED_TEST_OWNER group (AFP-V0-2482).
func TestOwnerKeepsFrontierUnknowns_AFPV02488(t *testing.T) {
	const contract = "docs/contracts/v1/order.json"
	languages := testSelectionLanguages()
	g, ts := languages[0].(fake), languages[1].(fake)
	g.frontier = []string{"g:dynamic-load:api.g:3"}
	ts.frontier = []string{"ts:dynamic-import:tests/contracts.spec.ts:9"}
	languages = []Language{g, ts}
	plan := func(declaration string) Plan {
		t.Helper()
		graph, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: declaration}), languages...)
		if err != nil {
			t.Fatal(err)
		}
		return Select(graph, []string{contract})
	}
	frontier := []string{"g:dynamic-load:api.g:3", "ts:dynamic-import:tests/contracts.spec.ts:9"}

	// Negative control: without an owner the unowned path raises every
	// plugin's frontier.
	bare := plan(`{"profile":"corvint-test-selection/0"}`)
	if bare.Scope != ScopeUnknown || !reflect.DeepEqual(unknownOf(bare, UnknownLanguageFrontier), frontier) ||
		!reflect.DeepEqual(unknownOf(bare, UnknownUnownedDirtyPath), []string{contract}) {
		t.Fatalf("undeclared: scope=%s unknown=%+v", bare.Scope, bare.Unknown)
	}

	owned := plan(contractsOwner)
	if owned.Scope != ScopeUnknown || !reflect.DeepEqual(unknownOf(owned, UnknownLanguageFrontier), frontier) || len(owned.FrontierExcluded) != 0 {
		t.Fatalf("owned: scope=%s unknown=%+v frontierExcluded=%+v", owned.Scope, owned.Unknown, owned.FrontierExcluded)
	}
	if got := witnesses(owned); len(got) != 1 || got["ts:contracts"].Kind != WitnessDeclaredTestOwner {
		t.Fatalf("owned: selected=%+v, want ts:contracts by its owner", owned.Selected)
	}
	if unknownOf(owned, UnknownUnownedDirtyPath) != nil || !reflect.DeepEqual(nonSourceOf(owned, ClassDeclaredTestOwner), []string{contract}) {
		t.Fatalf("owned: unknown=%+v nonSource=%+v", owned.Unknown, owned.NonSource)
	}
}

// AFP-V0-2483: a declared test that holds no test file contradicts the
// observed graph and refuses a full build, naming the entry; a partial build
// skips it, and an owner left with no unit keeps its unknowns.
func TestContradictedTestSelectionRefuses_AFPV02483(t *testing.T) {
	staleOwner := `{"profile":"corvint-test-selection/0","owners":[{"paths":"docs/contracts/**/*.json","tests":["tests/gone.spec.ts"]}]}`
	staleMustRun := `{"profile":"corvint-test-selection/0","mustRun":[{"tests":"gone/","reason":"r"}]}`
	for body, want := range map[string]string{
		staleOwner:   `owners "docs/contracts/**/*.json" tests "tests/gone.spec.ts" holds no test file`,
		staleMustRun: `mustRun tests "gone/" holds no test file`,
	} {
		root := testSelectionRoot(t, map[string]string{DeclaredSelectionPath: body})
		if _, err := Build(context.Background(), root, testSelectionLanguages()...); err == nil || !strings.Contains(err.Error(), DeclaredSelectionPath) || !strings.Contains(err.Error(), want) {
			t.Errorf("full build: err=%v, want %q", err, want)
		}
		graph, err := BuildPartial(context.Background(), root, testSelectionLanguages()...)
		if err != nil {
			t.Fatalf("partial build refused %s: %v", body, err)
		}
		plan := Select(graph, []string{"docs/contracts/v1/order.json"})
		if len(plan.Selected) != 0 || plan.MustRun != nil || !reflect.DeepEqual(unknownOf(plan, UnknownUnownedDirtyPath), []string{"docs/contracts/v1/order.json"}) {
			t.Errorf("partial plan: selected=%+v mustRun=%+v unknown=%+v", plan.Selected, plan.MustRun, plan.Unknown)
		}
	}
}

// AFP-V0-2484 (V1-1184): a must-run unit is selected whatever the change, by
// a MUST_RUN witness when nothing else reaches it, and named in plan.mustRun;
// no exclusion basis removes it, and it never enters the reached universe.
func TestMustRunSurvivesEveryExclusion_AFPV02484(t *testing.T) {
	documentation := `{"profile":"corvint-documentation-paths/0","paths":["NOTES.md"]}`
	graph, err := Build(context.Background(), testSelectionRoot(t, map[string]string{DeclaredSelectionPath: smokeMustRun, DocumentationPathsPath: documentation}), testSelectionLanguages()...)
	if err != nil {
		t.Fatal(err)
	}
	pinned := []MustRun{{UnitID: "g:smoke", Tests: "smoke/", Reason: "release smoke suite"}}

	// No dependency path: api.g reaches g:api and g:cmd, never g:smoke.
	plan := Select(graph, []string{"api.g"})
	want := Witness{Kind: WitnessMustRun, DirtyPath: "api.g", Via: []string{"g:smoke"}}
	if got := witnesses(plan); !reflect.DeepEqual(got["g:smoke"], want) || len(got) != 3 || !reflect.DeepEqual(plan.MustRun, pinned) {
		t.Fatalf("no dependency path: selected=%+v mustRun=%+v", plan.Selected, plan.MustRun)
	}
	if slices.ContainsFunc(plan.Excluded, func(exclusion Exclusion) bool { return exclusion.UnitID == "g:smoke" }) {
		t.Fatalf("g:smoke excluded: %+v", plan.Excluded)
	}
	if slices.Contains(graph.ReachedUnitIDs([]string{"api.g"}), "g:smoke") {
		t.Fatal("a must-run unit entered the reached universe")
	}

	// A documentation-only change selects nothing but the must-run unit.
	plan = Select(graph, []string{"NOTES.md"})
	if got := witnesses(plan); len(got) != 1 || got["g:smoke"].Kind != WitnessMustRun || plan.Scope != ScopeBounded {
		t.Fatalf("documentation change: scope=%s selected=%+v", plan.Scope, plan.Selected)
	}

	// A unit something else reaches keeps its own witness.
	plan = Select(graph, []string{"smoke/smoke.g"})
	if got := witnesses(plan); got["g:smoke"].Kind != WitnessDirectSource || !reflect.DeepEqual(plan.MustRun, pinned) {
		t.Fatalf("direct change: selected=%+v mustRun=%+v", plan.Selected, plan.MustRun)
	}

	// Negative control: with nothing dirty nothing is selected.
	plan = Select(graph, nil)
	if len(plan.Selected) != 0 || plan.MustRun != nil {
		t.Fatalf("empty dirty set: selected=%+v mustRun=%+v", plan.Selected, plan.MustRun)
	}

	// Negative control: without the declaration the smoke package is excluded.
	plan = testSelectionPlan(t, `{"profile":"corvint-test-selection/0"}`, "api.g")
	if _, ok := witnesses(plan)["g:smoke"]; ok || plan.MustRun != nil {
		t.Fatalf("undeclared: selected=%+v", plan.Selected)
	}
}

// AFP-V0-2482: a graph whose project declares no test selection names no
// owner for any path rather than panicking on the absent declaration.
func TestDeclaredTestOwnersWithoutDeclaration_AFPV02482(t *testing.T) {
	if got := (&Graph{}).DeclaredTestOwners("docs/contracts/v1/order.json"); got != nil {
		t.Fatalf("DeclaredTestOwners without a declaration = %v, want nil", got)
	}
}
