package typescript

import (
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Advisory narrowing states and reasons (TJAA-V0-030..032).
const (
	PlaywrightNarrowingBasis = "STATIC_IMPORT_GRAPH"
	PlaywrightNarrowed       = "NARROWED"
	PlaywrightNotNarrowed    = "NOT_NARROWED"

	PlaywrightNarrowingNoDirtyPath       = "NO_DIRTY_PATH"
	PlaywrightNarrowingNotTestSide       = "NOT_TEST_SIDE"
	PlaywrightNarrowingConfigUnresolved  = "CONFIG_UNRESOLVED"
	PlaywrightNarrowingGraphFrontier     = "GRAPH_FRONTIER"
	PlaywrightNarrowingDiscoveryProjects = "DISCOVERY_PROJECT_SET"

	PlaywrightMembershipStatic     = "STATIC_CONFIG"
	PlaywrightMembershipDiscovered = "DISCOVERY_MATCHED"
	PlaywrightMembershipUnresolved = "UNRESOLVED"

	playwrightWitnessDynamicImport = "DYNAMIC_IMPORT_FRONTIER"
)

// PlaywrightNarrowing is advisory evidence beside FULL_RELEVANT_SUITE: the statically importing
// Playwright units of a test-side change, plus every project whose membership is unresolved and so
// must still run in full. It never replaces the fallback (TJAA-V0-030).
type PlaywrightNarrowing struct {
	Basis      string                       `json:"basis"`
	Candidates []PlaywrightSelection        `json:"candidates"`
	Detail     string                       `json:"detail,omitzero"`
	Projects   []PlaywrightNarrowingProject `json:"projects"`
	Reason     string                       `json:"reason,omitzero"`
	State      string                       `json:"state"`
	Symbols    []PlaywrightSymbolChange     `json:"symbols"`
}

// PlaywrightNarrowingProject names how one project's file membership is known. An UNRESOLVED
// project contributes no candidates; its Argv runs it in full (TJAA-V0-032).
type PlaywrightNarrowingProject struct {
	Argv       []string `json:"argv"`
	Detail     string   `json:"detail,omitzero"`
	Membership string   `json:"membership"`
	Name       string   `json:"name"`
}

// playwrightNarrowingDraft is the static narrowing before discovery reconciliation.
type playwrightNarrowingDraft struct {
	narrowing PlaywrightNarrowing
	units     []PlaywrightSelection
}

// snapshotPlaywrightResult copies the observer's unit roles and import edges before Playwright
// membership binding rewrites them, so test-side classification sees every runner's tests.
func snapshotPlaywrightResult(result affected.Result) affected.Result {
	snapshot := affected.Result{Frontier: append([]string(nil), result.Frontier...), Units: make([]affected.Unit, len(result.Units))}
	for index, unit := range result.Units {
		snapshot.Units[index] = affected.Unit{
			ID: unit.ID, Sources: append([]string(nil), unit.Sources...),
			Tests: append([]string(nil), unit.Tests...), Imports: append([]string(nil), unit.Imports...),
		}
	}
	return snapshot
}

// playwrightTestSide answers whether every root of a unit's reverse-import closure, the unit
// itself included, is a test unit of any runner: then no application entry point reaches it
// through a static import (TJAA-V0-031).
type playwrightTestSide struct {
	owner     map[string]string
	test      map[string]bool
	importers map[string][]string
}

func newPlaywrightTestSide(observed affected.Result) playwrightTestSide {
	side := playwrightTestSide{owner: map[string]string{}, test: map[string]bool{}, importers: map[string][]string{}}
	for _, unit := range observed.Units {
		for _, name := range append(append([]string{}, unit.Sources...), unit.Tests...) {
			side.owner[name] = unit.ID
		}
		side.test[unit.ID] = len(unit.Tests) != 0
		for _, imported := range unit.Imports {
			side.importers[imported] = append(side.importers[imported], unit.ID)
		}
	}
	return side
}

func (side playwrightTestSide) path(relative string) bool {
	id, owned := side.owner[relative]
	if !owned {
		return false
	}
	seen := map[string]bool{id: true}
	queue := []string{id}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		importers := side.importers[current]
		if len(importers) == 0 && !side.test[current] {
			return false
		}
		for _, importer := range importers {
			if !seen[importer] {
				seen[importer] = true
				queue = append(queue, importer)
			}
		}
	}
	return true
}

func notNarrowed(reason, detail string) playwrightNarrowingDraft {
	return playwrightNarrowingDraft{narrowing: PlaywrightNarrowing{
		Basis: PlaywrightNarrowingBasis, Candidates: []PlaywrightSelection{}, Detail: detail,
		Projects: []PlaywrightNarrowingProject{}, Reason: reason, State: PlaywrightNotNarrowed,
		Symbols: []PlaywrightSymbolChange{},
	}}
}

// draftPlaywrightNarrowing computes the advisory candidate set from the static graph. Any input it
// cannot attribute to one project or one test-side module makes it NOT_NARROWED (TJAA-V0-030..032).
func draftPlaywrightNarrowing(root, configPath string, observed affected.Result, graph *affected.Graph, base affected.Plan, configUnknown []PlaywrightUnknown, projects []PlaywrightProject, units []PlaywrightSelection, dirty []string, informational map[string]bool) (playwrightNarrowingDraft, error) {
	if len(dirty) == 0 {
		return notNarrowed(PlaywrightNarrowingNoDirtyPath, "no dirty path to narrow from"), nil
	}
	scoped := map[PlaywrightUnknown]bool{}
	for _, project := range projects {
		for _, unknown := range project.unresolved {
			scoped[unknown] = true
		}
	}
	for _, unknown := range canonicalPlaywrightUnknowns(append([]PlaywrightUnknown(nil), configUnknown...)) {
		if !scoped[unknown] {
			return notNarrowed(PlaywrightNarrowingConfigUnresolved, unknown.Reason+": "+unknown.Detail), nil
		}
	}
	dynamicFrontier := false
	// An informational frontier reason cannot change the selection, so it never blocks narrowing
	// (TJAA-V0-110).
	for _, reason := range observed.Frontier {
		switch {
		case informational[reason]:
		case reason == FrontierConfig || reason == FrontierE2ERuntimeDependency:
		case reason == FrontierDynamicImport:
			dynamicFrontier = true
		default:
			return notNarrowed(PlaywrightNarrowingGraphFrontier, reason), nil
		}
	}
	for _, unknown := range base.Unknown {
		switch {
		case unknown.Reason == affected.UnknownNoSelectableTest:
		case unknown.Reason == affected.UnknownLanguageFrontier && informational[unknown.Detail]:
		case unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == FrontierDynamicImport:
		case unknown.Reason == affected.UnknownLanguageFrontier:
			return notNarrowed(PlaywrightNarrowingGraphFrontier, unknown.Detail), nil
		default:
			return notNarrowed(PlaywrightNarrowingNotTestSide, unknown.Reason+": "+unknown.Detail), nil
		}
	}
	side := newPlaywrightTestSide(observed)
	for _, relative := range dirty {
		if relative == configPath {
			return notNarrowed(PlaywrightNarrowingNotTestSide, "the Playwright config is dirty: "+relative), nil
		}
		if !side.path(relative) {
			return notNarrowed(PlaywrightNarrowingNotTestSide, "not a test-side TypeScript/JavaScript module: "+relative), nil
		}
	}
	candidates := map[string]PlaywrightSelection{}
	if dynamicFrontier {
		dynamic, reason, err := playwrightDynamicImportFiles(root, observed)
		if err != nil {
			return playwrightNarrowingDraft{}, err
		}
		if reason != "" {
			return notNarrowed(PlaywrightNarrowingGraphFrontier, reason), nil
		}
		for _, relative := range dynamic {
			if !side.path(relative) {
				return notNarrowed(PlaywrightNarrowingGraphFrontier, "dynamic import outside test-side code: "+relative), nil
			}
		}
		// A test that statically reaches a dynamic import may load the dirty module at runtime.
		for _, unit := range selectPlaywrightUnits(configPath, affected.Select(graph, dynamic), projects, units, dynamic) {
			unit.Witness.Kind = playwrightWitnessDynamicImport
			candidates[unit.ID] = unit
		}
	}
	for _, unit := range selectPlaywrightUnits(configPath, base, projects, units, dirty) {
		candidates[unit.ID] = unit
	}
	draft := playwrightNarrowingDraft{units: units, narrowing: PlaywrightNarrowing{
		Basis: PlaywrightNarrowingBasis, State: PlaywrightNarrowed, Symbols: []PlaywrightSymbolChange{},
		Candidates: expandPlaywrightProjectEdges(sortedPlaywrightSelections(candidates), projects, units),
	}}
	for _, project := range projects {
		entry := PlaywrightNarrowingProject{Argv: append([]string(nil), project.Argv...), Membership: PlaywrightMembershipStatic, Name: project.Name}
		if len(project.unresolved) != 0 {
			entry.Membership = PlaywrightMembershipUnresolved
			details := make([]string, 0, len(project.unresolved))
			for _, unknown := range canonicalPlaywrightUnknowns(append([]PlaywrightUnknown(nil), project.unresolved...)) {
				details = append(details, unknown.Reason+": "+unknown.Detail)
			}
			entry.Detail = strings.Join(details, "; ")
		}
		draft.narrowing.Projects = append(draft.narrowing.Projects, entry)
	}
	draft.narrowing.Candidates = withoutUnresolvedProjects(draft.narrowing.Candidates, draft.narrowing.Projects)
	return draft, nil
}

// playwrightDynamicImportFiles names every owned module whose import scan reports a dynamic
// import or computed require; an unreadable or unparsable module is a reason, not a file.
func playwrightDynamicImportFiles(root string, observed affected.Result) ([]string, string, error) {
	language := New()
	dynamic := []string{}
	for _, unit := range observed.Units {
		for _, relative := range append(append([]string{}, unit.Sources...), unit.Tests...) {
			if !language.Owns(relative) {
				continue
			}
			body, err := affected.ReadSource(root, relative)
			if err != nil {
				return nil, "", err
			}
			_, isDynamic, parseErr := scanImports(relative, string(body))
			if parseErr != nil {
				return nil, FrontierUnparsedSource + ": " + relative, nil
			}
			if isDynamic {
				dynamic = append(dynamic, relative)
			}
		}
	}
	sort.Strings(dynamic)
	return dynamic, "", nil
}

func withoutUnresolvedProjects(candidates []PlaywrightSelection, projects []PlaywrightNarrowingProject) []PlaywrightSelection {
	unresolved := map[string]bool{}
	for _, project := range projects {
		if project.Membership == PlaywrightMembershipUnresolved {
			unresolved[project.Name] = true
		}
	}
	kept := []PlaywrightSelection{}
	for _, unit := range candidates {
		if !unresolved[unit.Project] {
			kept = append(kept, unit)
		}
	}
	return kept
}

// finishPlaywrightNarrowing cross-checks each statically resolved project against a discovery
// receipt bound to this revision, config and sources, when one exists (TJAA-V0-032).
func finishPlaywrightNarrowing(draft playwrightNarrowingDraft, discovery PlaywrightDiscoverySummary, listed []PlaywrightDiscoveryUnit) PlaywrightNarrowing {
	narrowing := draft.narrowing
	bound := discovery.State == "STATIC_UNRESOLVED" || discovery.State == "UNIVERSE_MISMATCH" || discovery.State == "MATCHED"
	if narrowing.State != PlaywrightNarrowed || !bound {
		return narrowing
	}
	staticTests := map[string][]string{}
	for _, unit := range draft.units {
		staticTests[unit.Project] = append(staticTests[unit.Project], unit.Test)
	}
	listedTests := map[string][]string{}
	for _, unit := range listed {
		listedTests[unit.Project] = append(listedTests[unit.Project], unit.Test)
	}
	known := map[string]bool{}
	for _, project := range narrowing.Projects {
		known[project.Name] = true
	}
	for _, name := range sortedKeys(listedTests) {
		if !known[name] {
			return notNarrowed(PlaywrightNarrowingDiscoveryProjects, "discovery lists project "+name+" that the static config does not declare").narrowing
		}
	}
	projects := make([]PlaywrightNarrowingProject, len(narrowing.Projects))
	copy(projects, narrowing.Projects)
	for index, project := range projects {
		if project.Membership != PlaywrightMembershipStatic {
			continue
		}
		static, discovered := staticTests[project.Name], listedTests[project.Name]
		sort.Strings(static)
		sort.Strings(discovered)
		if slices.Equal(static, discovered) {
			projects[index].Membership = PlaywrightMembershipDiscovered
		} else {
			projects[index].Membership = PlaywrightMembershipUnresolved
			projects[index].Detail = "discovery listing differs from static file membership"
		}
	}
	narrowing.Projects = projects
	narrowing.Candidates = withoutUnresolvedProjects(narrowing.Candidates, projects)
	return narrowing
}
