// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// directed is a reaching plugin that classifies its frontier reasons
// (FrontierDirection) and reports which units it can run (RunnableUnits).
type directed struct {
	reaching
	directions map[string]string
	runnable   func(id string) bool
	sites      map[string][]string
}

func (language directed) FrontierDirection(reason string) string { return language.directions[reason] }
func (language directed) RunnableUnit(id string) bool {
	return language.runnable != nil && language.runnable(id)
}
func (language directed) Units(_ context.Context, _ string) (Result, error) {
	result, err := language.fake.Units(context.Background(), "")
	result.FrontierSites = language.sites
	return result, err
}

func closureSelectedIDs(plan Plan) []string {
	ids := make([]string, 0, len(plan.Selected))
	for _, selection := range plan.Selected {
		ids = append(ids, selection.UnitID)
	}
	return ids
}

// hostedFixtureClosed reports an entry the plan closed through the fixture's
// Go host (AZU-V0-211).
func hostedFixtureClosed(plan Plan, entry string) bool {
	for _, exclusion := range plan.FrontierExcluded {
		if exclusion.Reason == HostedFixtureExclusionReason && slices.Contains(exclusion.Frontier, entry) {
			return true
		}
	}
	return false
}

func witnessOf(plan Plan, id string) (Witness, bool) {
	for _, selection := range plan.Selected {
		if selection.UnitID == id {
			return selection.Witness, true
		}
	}
	return Witness{}, false
}

// AZU-V0-003, AZU-V0-212 (V1-4686): an OUTBOUND entry that bears through
// participation selects every site unit with witness FRONTIER_SITE, its
// dependents, the readers of its files and the units that embed them, and is
// not named; the hidden edge starts at the site file, so the fixture proves
// that only those units can observe whatever path it binds. A bare entry and
// an entry whose site file no unit owns stay unknown (fail closed), and a
// `reason:+N` entry closes only through the plugin's complete site record.
func TestOutboundSiteClosure_AZU_V0_003(t *testing.T) {
	const site = "y:dynamic-load:lib/loader.y:7 `import`"
	const bare = "y:scan-unresolved"
	const unlocated = "y:dynamic-load:vendor/blob.y:1 `require`"
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{bare, site, unlocated}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:loader", Sources: []string{"lib/loader.y"}},
		{ID: "y:consumer", Tests: []string{"lib/consumer_test.y"}, Imports: []string{"y:loader"}},
		{ID: "y:other", Tests: []string{"other/other_test.y"}},
	}}}, directions: map[string]string{"y:dynamic-load": FrontierOutbound}}
	goPlugin := fake{name: "go", units: []Unit{
		{ID: "go:runner", Sources: []string{"runner/runner.go"}, Tests: []string{"runner/runner_test.go"}, PathTokens: []string{"lib/loader.y"}},
		{ID: "go:embedder", Sources: []string{"embed/embed.go"}, Tests: []string{"embed/embed_test.go"}, Embeds: true, EmbedFiles: []string{"lib/loader.y"}},
		{ID: "go:clitest", Tests: []string{"cli/cli_test.go"}, Execs: []string{"go:embedder"}, TestOnlyExecs: []string{"go:embedder"}},
		{ID: "go:bystander", Sources: []string{"by/by.go"}, Tests: []string{"by/by_test.go"}},
	}}
	graph, err := Build(context.Background(), t.TempDir(), goPlugin, language)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"app/app.y"})
	frontier := languageFrontier(plan)
	if slices.Contains(frontier, site) {
		t.Errorf("located OUTBOUND site named instead of selected: %v", frontier)
	}
	for _, want := range []string{bare, unlocated} {
		if !slices.Contains(frontier, want) {
			t.Errorf("%s must stay unknown: %v", want, frontier)
		}
	}
	for id, kind := range map[string]string{"y:consumer": WitnessDependency, "go:runner": WitnessPathLiteralReader, "go:embedder": WitnessDependency,
		"go:clitest": WitnessBinaryExec} {
		witness, hit := witnessOf(plan, id)
		if !hit || witness.Kind != kind {
			t.Errorf("%s: witness %+v, want %s through the site", id, witness, kind)
		} else if len(witness.Via) < 2 || witness.Via[0] != "y:loader" {
			t.Errorf("%s: chain %v (length %d, dirty %s) does not start at the site unit", id, witness.Via, witness.ViaLength(), witness.DirtyPath)
		}
	}
	if slices.Contains(closureSelectedIDs(plan), "y:other") || slices.Contains(closureSelectedIDs(plan), "go:bystander") {
		t.Errorf("closure selected units no site reaches: %v", closureSelectedIDs(plan))
	}
	for _, bearer := range plan.FrontierBearers {
		if bearer.Detail == site {
			t.Errorf("a discharged entry keeps a bearer: %+v", bearer)
		}
	}
	// A plan in which the plugin does not take part selects nothing for the
	// site and names no entry (AFP-V0-560).
	plan = Select(graph, []string{"by/by.go"})
	if len(languageFrontier(plan)) != 0 || slices.Contains(closureSelectedIDs(plan), "y:consumer") {
		t.Errorf("a plan outside the plugin ran the closure: %+v", plan)
	}
}

// AZU-V0-212: a `reason:+N` entry closes through Result.FrontierSites, the
// plugin's complete site record; without the record it stays unknown.
func TestOverflowSitesClose_AZU_V0_212(t *testing.T) {
	const overflow = "y:dynamic-load:+2"
	units := []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:one", Sources: []string{"lib/one.y"}, Tests: []string{"lib/one_test.y"}},
		{ID: "y:two", Sources: []string{"lib/two.y"}, Tests: []string{"lib/two_test.y"}},
	}
	for _, recorded := range []bool{false, true} {
		language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{overflow}, units: units}}, directions: map[string]string{"y:dynamic-load": FrontierOutbound}}
		if recorded {
			language.sites = map[string][]string{"y:dynamic-load": {"lib/one.y:3 `import`", "lib/two.y:9 `require`"}}
		}
		graph, err := Build(context.Background(), t.TempDir(), language)
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{"app/app.y"})
		named := slices.Contains(languageFrontier(plan), overflow)
		selected := slices.Contains(closureSelectedIDs(plan), "y:one") && slices.Contains(closureSelectedIDs(plan), "y:two")
		if recorded && (named || !selected || plan.Scope != ScopeBounded) {
			t.Errorf("recorded sites: named=%v selected=%v scope=%s", named, selected, plan.Scope)
		}
		if !recorded && (!named || selected) {
			t.Errorf("unrecorded sites: named=%v selected=%v; a +N entry without a record must stay unknown", named, selected)
		}
	}
}

// AZU-V0-008, AZU-V0-213 (V1-4691): a PACKAGE_RUNNABLE entry selects its
// runnable site unit for its tests alone and closes; one whose unit the
// plugin cannot run stays unknown with the NO_RUNNER hint.
func TestPackageRunnableSites_AZU_V0_008(t *testing.T) {
	const flags = "y:runtime-flags-unresolved:tests/flags_test.y:2 `node:test`"
	const runner = "y:test-runner-unknown:tests/unknown_test.y"
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{flags, runner}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:node:flags", Tests: []string{"tests/flags_test.y"}},
		{ID: "y:unknown:runner", Tests: []string{"tests/unknown_test.y"}},
	}}}, directions: map[string]string{"y:runtime-flags-unresolved": FrontierPackageRunnable, "y:test-runner-unknown": FrontierPackageRunnable},
		runnable: func(id string) bool { return strings.HasPrefix(id, "y:node:") }}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"app/app.y"})
	if witness, hit := witnessOf(plan, "y:node:flags"); !hit || witness.Kind != WitnessFrontierSite {
		t.Errorf("runnable site not selected as a frontier site: %+v", plan.Selected)
	}
	if slices.Contains(languageFrontier(plan), flags) {
		t.Errorf("closed PACKAGE_RUNNABLE entry named: %v", plan.Unknown)
	}
	index := slices.IndexFunc(plan.Unknown, func(u Unknown) bool { return u.Detail == runner })
	if index < 0 || !strings.Contains(plan.Unknown[index].Hint, NoRunnerHint) || plan.Scope != ScopeUnknown {
		t.Errorf("NO_RUNNER site must stay unknown with its hint: %+v", plan.Unknown)
	}
	if slices.Contains(closureSelectedIDs(plan), "y:unknown:runner") {
		t.Errorf("a unit no runner runs was selected: %v", closureSelectedIDs(plan))
	}
}

// AZU-V0-212: a DISCOVERY entry closes like OUTBOUND unless the site file is
// dirty or a dirty path of the plugin lies below the site's directory, when
// what the site decides about may have changed.
func TestDiscoverySiteKeepsEntryOnDirtyConfiguration_AZU_V0_212(t *testing.T) {
	const site = "y:executable-config-unresolved:suite/y.config.y:5 `testMatch`"
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{site}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:config", Sources: []string{"suite/y.config.y"}},
		{ID: "y:suite", Tests: []string{"suite/a_test.y"}, Imports: []string{"y:config"}},
		{ID: "y:member", Sources: []string{"suite/member.y"}},
	}}}, directions: map[string]string{"y:executable-config-unresolved": FrontierDiscovery}}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dirty string
		named bool
	}{
		{"app/app.y", false},
		{"suite/y.config.y", true},
		{"suite/member.y", true},
	} {
		plan := Select(graph, []string{tc.dirty})
		if got := slices.Contains(languageFrontier(plan), site); got != tc.named {
			t.Errorf("dirty %s: named=%v, want %v (%v)", tc.dirty, got, tc.named, plan.Unknown)
		}
		if tc.dirty == "app/app.y" && !slices.Contains(closureSelectedIDs(plan), "y:suite") {
			t.Errorf("dirty %s: the closure must select the suite under the configuration: %v", tc.dirty, closureSelectedIDs(plan))
		}
	}
}

// AZU-V0-211 (V1-4999, V1-4679): a participating plugin's entry whose site
// lies in a `testdata` directory a Go unit hosts does not bear when the change
// neither touches nor reaches the fixture: a source-bound reason closes
// outright, a path-open reason closes by selecting the hosting unit, and the
// closed entries are reported in frontierExcluded. A dirty path inside the
// fixture, a reached fixture unit, a dirty resolution input and an unhosted
// fixture keep the entry bearing (fault injections).
func TestHostedFixtureSitesCloseThroughTheHost_AZU_V0_211(t *testing.T) {
	const bare = "y:bare-import:pkg/testdata/suite/specs/a_test.y:1 `react`"
	const dynamic = "y:dynamic-load:pkg/testdata/suite/specs/a_test.y:4 `import`"
	const sibling = "y:dynamic-load:pkg/testdata/other/b_test.y:4 `import`"
	build := func(t *testing.T, hosted bool, inputs func(string) bool) *Graph {
		language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{bare, dynamic, sibling}, units: []Unit{
			{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
			{ID: "y:fixture", Tests: []string{"pkg/testdata/suite/specs/a_test.y"}},
			{ID: "y:sibling", Tests: []string{"pkg/testdata/other/b_test.y"}},
			{ID: "y:inside", Sources: []string{"pkg/testdata/suite/specs/helper.y"}},
		}}, sourceBound: []string{"y:bare-import"}, inputs: inputs}, directions: map[string]string{"y:bare-import": FrontierOutbound, "y:dynamic-load": FrontierOutbound}}
		plugins := []Language{language}
		if hosted {
			plugins = append(plugins, fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}})
		}
		graph, err := Build(context.Background(), t.TempDir(), plugins...)
		if err != nil {
			t.Fatal(err)
		}
		return graph
	}
	graph := build(t, true, nil)
	plan := Select(graph, []string{"app/app.y"})
	if frontier := languageFrontier(plan); len(frontier) != 0 || plan.Scope != ScopeBounded {
		t.Fatalf("hosted fixture sites bore: %v scope %s", frontier, plan.Scope)
	}
	if witness, hit := witnessOf(plan, "go:pkg"); !hit || witness.Kind != WitnessFrontierSite {
		t.Errorf("the host must be selected for the path-open fixture sites: %+v", plan.Selected)
	}
	if slices.Contains(closureSelectedIDs(plan), "y:fixture") || slices.Contains(closureSelectedIDs(plan), "y:sibling") {
		t.Errorf("the closure walked into the fixture: %v", closureSelectedIDs(plan))
	}
	excluded := slices.IndexFunc(plan.FrontierExcluded, func(e FrontierExclusion) bool { return e.Reason == HostedFixtureExclusionReason })
	if excluded < 0 || !slices.Equal(plan.FrontierExcluded[excluded].Frontier, []string{bare, sibling, dynamic}) || !slices.Equal(plan.FrontierExcluded[excluded].Units, []string{"go:pkg"}) {
		t.Errorf("hosted fixture closure not reported: %+v", plan.FrontierExcluded)
	}
	// Fault injections. A dirty path inside the fixture makes it live: its
	// own OUTBOUND site is closed by selecting the fixture unit as the site
	// (AZU-V0-003), while the untouched sibling fixture's site still closes
	// through the host.
	plan = Select(graph, []string{"pkg/testdata/suite/specs/helper.y"})
	if witness, hit := witnessOf(plan, "y:fixture"); !hit || witness.Kind != WitnessFrontierSite || !hostedFixtureClosed(plan, sibling) || hostedFixtureClosed(plan, dynamic) {
		t.Errorf("dirty path inside the fixture: selected %v, excluded %+v; want the fixture unit selected as the site and only the sibling closed by the host", closureSelectedIDs(plan), plan.FrontierExcluded)
	}
	plan = Select(graph, []string{"pkg/testdata/suite/specs/a_test.y"})
	if !slices.Contains(closureSelectedIDs(plan), "y:fixture") {
		t.Errorf("a dirty fixture test is selected: %v", closureSelectedIDs(plan))
	}
	// A dirty resolution input the plugin does not own may rebind every
	// site, so the fixture sites are live: the fixture units are selected as
	// sites and nothing closes through the host.
	plan = Select(build(t, true, func(path string) bool { return path == "app/package.y" }), []string{"app/package.y", "app/app.y"})
	if !slices.Contains(closureSelectedIDs(plan), "y:fixture") || !slices.Contains(closureSelectedIDs(plan), "y:sibling") || len(plan.FrontierExcluded) != 0 {
		t.Errorf("a dirty resolution input must make the fixture sites live: selected %v excluded %+v", closureSelectedIDs(plan), plan.FrontierExcluded)
	}
	plan = Select(build(t, false, nil), []string{"app/app.y"})
	if !slices.Contains(closureSelectedIDs(plan), "y:fixture") && !slices.Contains(languageFrontier(plan), dynamic) {
		t.Errorf("an unhosted fixture site is a live site: %v %v", languageFrontier(plan), closureSelectedIDs(plan))
	}
}

// AZU-V0-212: a plan with a dirty path no plugin owns stays UNKNOWN whatever
// the frontier says, so no selection closes an entry on it and the closure
// selects nothing.
func TestUnownedPathDisablesSiteClosure_AZU_V0_212(t *testing.T) {
	const site = "y:dynamic-load:lib/loader.y:7 `import`"
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{site}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:loader", Sources: []string{"lib/loader.y"}},
		{ID: "y:consumer", Tests: []string{"lib/consumer_test.y"}, Imports: []string{"y:loader"}},
	}}}, directions: map[string]string{"y:dynamic-load": FrontierOutbound}}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"data/payload.json", "app/app.y"})
	if !slices.Contains(languageFrontier(plan), site) || slices.Contains(closureSelectedIDs(plan), "y:consumer") {
		t.Errorf("unowned path: frontier %v selected %v, want the entry named and no closure", languageFrontier(plan), closureSelectedIDs(plan))
	}
}

// AZU-V0-212: the graph cache carries the plugin's site record, so a cached
// graph closes `+N` entries as a fresh one does.
func TestFrontierSitesSurviveTheGraphCache_AZU_V0_212(t *testing.T) {
	const overflow = "y:dynamic-load:+1"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.y"), []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}
	language := directed{reaching: reaching{fake: fake{name: "y", frontier: []string{overflow}, units: []Unit{
		{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		{ID: "y:one", Sources: []string{"lib/one.y"}, Tests: []string{"lib/one_test.y"}},
	}}}, directions: map[string]string{"y:dynamic-load": FrontierOutbound}, sites: map[string][]string{"y:dynamic-load": {"lib/one.y:3 `import`"}}}
	dir := t.TempDir()
	fresh, outcome, err := BuildCached(context.Background(), root, dir, "test", language)
	if err != nil {
		t.Fatal(err)
	}
	cached, second, err := BuildCached(context.Background(), root, dir, "test", language)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cache outcomes: %+v then %+v", outcome, second)
	for name, graph := range map[string]*Graph{"fresh": fresh, "cached": cached} {
		plan := Select(graph, []string{"app/app.y"})
		if slices.Contains(languageFrontier(plan), overflow) || !slices.Contains(closureSelectedIDs(plan), "y:one") {
			t.Errorf("%s graph: frontier %v selected %v", name, languageFrontier(plan), closureSelectedIDs(plan))
		}
	}
}
