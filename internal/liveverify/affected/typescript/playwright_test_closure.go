package typescript

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// playwrightPrepared is the graph phase shared by a Playwright plan and the per-test closures:
// the config parsed, the TypeScript units observed with test membership and global hooks bound,
// and the graph built over the frontier reasons the Playwright profile does not model itself.
type playwrightPrepared struct {
	sourceDigest     string
	configSum        [sha256.Size]byte
	projects         []PlaywrightProject
	globalTestDir    string
	configUnknown    []PlaywrightUnknown
	environment      []PlaywrightEnvironmentInput
	environmentFiles []PlaywrightEnvironmentFile
	result           affected.Result
	sites            frontierSet
	observed         affected.Result
	units            []PlaywrightSelection
	filtered         affected.Result
	graph            *affected.Graph
}

func preparePlaywrightGraph(root, configPath string, environment []PlaywrightEnvironmentInput) (playwrightPrepared, error) {
	if !affected.ValidRelativePath(configPath) || !hasSourceExtension(configPath) {
		return playwrightPrepared{}, errors.New("playwright config path is not canonical TypeScript/JavaScript source")
	}
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return playwrightPrepared{}, err
	}
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil || !utf8.Valid(configBytes) {
		return playwrightPrepared{}, errors.New("playwright config is unreadable")
	}
	p := playwrightPrepared{sourceDigest: sourceDigest, configSum: sha256.Sum256(configBytes)}
	p.projects, p.globalTestDir, p.configUnknown = parsePlaywrightConfig(configPath, string(configBytes))
	p.environment, p.environmentFiles = resolvePlaywrightExecutables(root, p.projects, environment, configPath, string(configBytes))
	p.result, p.sites, err = New().units(root, true)
	if err != nil {
		return playwrightPrepared{}, err
	}
	p.configUnknown = append(p.configUnknown, bindPlaywrightGlobalHooks(configPath, string(configBytes), &p.result, affected.HeldIgnoredMatcher(root))...)
	p.observed = snapshotPlaywrightResult(p.result)
	tests := playwrightSourcePaths(p.result)
	if playwrightPathOutsideMatcherModel(root) || slices.ContainsFunc(tests, playwrightPathOutsideMatcherModel) {
		p.configUnknown = append(p.configUnknown, selectionUnknown(PlaywrightUnknownProjectMembership, "a test path contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently"))
	}
	p.units = playwrightUnits(root, configPath, p.projects, p.globalTestDir, tests)
	bindPlaywrightTestMembership(&p.result, p.units, configPath)
	p.filtered = p.result
	p.filtered.Frontier = nil
	for _, reason := range p.result.Frontier {
		if reason != FrontierConfig && reason != FrontierE2ERuntimeDependency {
			p.filtered.Frontier = append(p.filtered.Frontier, reason)
		}
	}
	p.graph, err = affected.Build(context.Background(), root, observedTypeScript{result: p.filtered})
	if err != nil {
		return playwrightPrepared{}, err
	}
	return p, nil
}

// PlaywrightTestClosureRule names the computed per-test closure rule (TJAA-V0-140).
const PlaywrightTestClosureRule = "corvint-test-closure/0"

// PlaywrightTestClosure is the closure evidence of one bound Playwright test file (TJAA-V0-140).
// When Unknown is empty, Files are the repository files of the graph units a run of the test may
// load and Excludable are the graph files it cannot load: each owned by a unit that the whole
// suite's closure holds, that this test's closure does not, and that no unit outside the whole
// suite's closure reaches, so neither the runner nor code outside the suite (such as an application
// the run serves) loads it on this test's behalf. When Unknown is non-empty both are nil and the
// caller must not narrow the test's identity (TJAA-V0-141). Widened is non-empty, and Excludable
// empty, when the test's closure is known but a frontier outside the whole suite's closure may load
// a suite file, so no file can be proved excludable (TJAA-V0-141).
type PlaywrightTestClosure struct {
	Test       string
	Files      []string
	Excludable []string
	Widened    string
	Unknown    string
}

// playwrightTestClosureBound caps the units of the whole suite's closure whose reach is walked to
// prove a file excludable; past it every test's closure is unknown (TJAA-V0-141).
const playwrightTestClosureBound = playwrightClosureBound

// PlaywrightTestClosures computes, read-only, one PlaywrightTestClosure per distinct test file a
// project of the config at configPath admits, sorted by test path (TJAA-V0-140, TJAA-V0-141). It
// reuses the plan's graph phase and closure walk, so the whole-suite selection is unchanged.
func PlaywrightTestClosures(root, configPath string) ([]PlaywrightTestClosure, error) {
	p, err := preparePlaywrightGraph(root, configPath, nil)
	if err != nil {
		return nil, err
	}
	graph := p.graph
	tests := map[string][]string{}
	for _, unit := range p.units {
		tests[unit.Test] = append(tests[unit.Test], unit.Project)
	}
	names := sortedKeys(tests)
	whole := newPlaywrightClosure(graph, configPath, nil, p.sites)
	informational := playwrightInformationalFrontier(whole, p.filtered.Frontier, p.sites, nil, p.projects, root, p.globalTestDir)
	relevant := relevantPlaywrightFrontier(p.filtered.Frontier, informational)
	whole.members()
	unknown := ""
	switch {
	case len(closureConfigUnknown(p.configUnknown)) != 0:
		unknown = "Playwright config is not statically resolved: " + canonicalPlaywrightUnknowns(closureConfigUnknown(p.configUnknown))[0].Detail
	case whole.bound != "":
		unknown = "the whole suite's closure is unbounded: " + whole.bound
	case len(whole.units) > playwrightTestClosureBound:
		unknown = strconv.Itoa(len(whole.units)) + " units in the whole suite's closure exceed the " + strconv.Itoa(playwrightTestClosureBound) + "-unit excludability bound"
	}
	closures := make([]PlaywrightTestClosure, 0, len(names))
	if unknown != "" {
		for _, test := range names {
			closures = append(closures, PlaywrightTestClosure{Test: test, Unknown: unknown})
		}
		return closures, nil
	}
	// A unit of the whole suite's closure that a unit outside it reaches may be loaded by code the
	// runner does not start, so no test may exclude it.
	shared := map[string]bool{}
	for _, id := range sortedKeys(whole.units) {
		unit, _ := graph.Unit(id)
		for _, reached := range graph.ReachedUnitIDs(append(slices.Clone(unit.Sources), unit.Tests...)) {
			if whole.units[reached] == "" {
				shared[id] = true
				break
			}
		}
	}
	widened := playwrightExcludabilityUnknown(graph, whole, p.filtered.Frontier, p.sites)
	owners := playwrightTestOwners(p.units)
	for _, test := range names {
		closure := playwrightTestClosure(graph, configPath, p, relevant, whole, shared, owners, test, tests[test])
		if closure.Unknown == "" && closure.Widened == "" && widened != "" {
			closure.Excludable, closure.Widened = []string{}, widened
		}
		closures = append(closures, closure)
	}
	after, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return nil, err
	}
	if after != p.sourceDigest {
		return nil, errors.New("Playwright source changed while the closures were computed")
	}
	return closures, nil
}

// closureConfigUnknown is the config unknowns that can change which files a test loads. A
// browser identity unknown (a dynamic launchOptions, browserName or device) decides which browser
// runs, not which source files load, so it does not leave every closure unknown (PWR-V0-177); a
// widening unknown degrades only the tests of the projects that hold it (PWR-V0-184, PWR-V0-185).
func closureConfigUnknown(unknown []PlaywrightUnknown) []PlaywrightUnknown {
	kept := make([]PlaywrightUnknown, 0, len(unknown))
	for _, u := range unknown {
		if u.Reason != PlaywrightUnknownBrowserIdentity && !closureWideningUnknown(u) {
			kept = append(kept, u)
		}
	}
	return kept
}

// closureWideningUnknown reports a project-scoped unknown that runs config code the static
// selection does not model but cannot change which test files a project admits or depends on: a
// use value that is not proved side-effect free (PWR-V0-184) or dynamic project metadata
// (PWR-V0-185). The tests of a project holding one, and of every project that depends on it, keep
// a known closure with no excludable file (Widened) instead of an unknown.
func closureWideningUnknown(u PlaywrightUnknown) bool {
	return u.Reason == PlaywrightUnknownUseValueUnverified ||
		u.Reason == PlaywrightUnknownConfigSyntax && strings.HasSuffix(u.Detail, playwrightMetadataDynamic)
}

// playwrightExcludabilityUnknown returns why no suite file can be proved excludable, or "": a
// unit outside the whole suite's closure with unit frontier or path tokens over the bound, or a
// graph frontier reason, relevant or informational, with no located site or a site outside the
// whole suite's closure. Such a site may load a suite file from code the runner does not start.
func playwrightExcludabilityUnknown(graph *affected.Graph, whole *playwrightClosure, frontier []string, sites frontierSet) string {
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		switch {
		case whole.units[id] != "":
		case len(unit.Frontier) != 0:
			return "unit " + id + " outside the whole suite's closure has frontier " + unit.Frontier[0]
		case unit.PathTokensBounded:
			return "unit " + id + " outside the whole suite's closure keeps no path tokens over the bound"
		}
	}
	for _, reason := range frontier {
		located := orderedSites(sites[reason])
		if len(located) == 0 {
			return "graph frontier " + reason + " has no located site"
		}
		for _, site := range located {
			if id, owned := graph.OwnerOf(playwrightSiteFile(site)); !owned || whole.units[id] == "" {
				return "graph frontier " + reason + " at " + site + " is outside the whole suite's closure"
			}
		}
	}
	return ""
}

// playwrightTestOwners maps each project name to the test files it admits.
func playwrightTestOwners(units []PlaywrightSelection) map[string][]string {
	owners := map[string][]string{}
	for _, unit := range units {
		owners[unit.Project] = append(owners[unit.Project], unit.Test)
	}
	return owners
}

// playwrightTestClosure walks one test's closure from its unit and the units of every test its
// projects' dependency and teardown projects admit, since Playwright runs those first or after.
func playwrightTestClosure(graph *affected.Graph, configPath string, p playwrightPrepared, relevant []string, whole *playwrightClosure, shared map[string]bool, owners map[string][]string, test string, projects []string) PlaywrightTestClosure {
	byName := map[string]PlaywrightProject{}
	for _, project := range p.projects {
		byName[project.Name] = project
	}
	// Only a project reached through a dependency or teardown edge runs all of its tests beside this
	// one; the test's own projects run just this test.
	files := map[string]bool{test: true}
	seen, admitted := map[string]bool{}, map[string]bool{}
	widening := []string{}
	type step struct {
		name   string
		linked bool
	}
	queue := []step{}
	for _, name := range projects {
		queue = append(queue, step{name: name})
	}
	for ; len(queue) != 0; queue = queue[1:] {
		current := queue[0]
		if current.linked && !admitted[current.name] {
			admitted[current.name] = true
			for _, file := range owners[current.name] {
				files[file] = true
			}
		}
		if seen[current.name] {
			continue
		}
		seen[current.name] = true
		project := byName[current.name]
		for _, u := range project.unresolved {
			if closureWideningUnknown(u) {
				widening = append(widening, u.Detail)
			}
		}
		for _, next := range append(slices.Clone(project.Dependencies), project.Teardown) {
			if next != "" {
				queue = append(queue, step{name: next, linked: true})
			}
		}
	}
	seeds := []string{}
	for _, file := range sortedKeys(files) {
		id, owned := graph.OwnerOf(file)
		if !owned {
			return PlaywrightTestClosure{Test: test, Unknown: file + " is not a TypeScript graph unit"}
		}
		seeds = append(seeds, id)
	}
	closure := newPlaywrightClosure(graph, configPath, relevant, p.sites)
	closure.seeds = slices.Compact(slices.Sorted(slices.Values(seeds)))
	closure.build()
	if closure.unknown != "" {
		return PlaywrightTestClosure{Test: test, Unknown: closure.unknown}
	}
	result := PlaywrightTestClosure{Test: test, Files: []string{}, Excludable: []string{}}
	for _, id := range sortedKeys(whole.units) {
		unit, _ := graph.Unit(id)
		owned := append(slices.Clone(unit.Sources), unit.Tests...)
		switch {
		case closure.units[id] != "":
			result.Files = append(result.Files, owned...)
		case !shared[id]:
			result.Excludable = append(result.Excludable, owned...)
		}
	}
	for _, id := range sortedKeys(closure.units) {
		if whole.units[id] == "" {
			// A unit outside the whole suite's closure cannot be in one test's; refuse rather
			// than trust a walk that disagrees with itself.
			return PlaywrightTestClosure{Test: test, Unknown: "closure unit " + id + " is outside the whole suite's closure"}
		}
	}
	if !slices.Contains(result.Files, configPath) {
		result.Files = append(result.Files, configPath)
	}
	sort.Strings(result.Files)
	sort.Strings(result.Excludable)
	result.Files = slices.Compact(result.Files)
	result.Excludable = slices.Compact(result.Excludable)
	if len(widening) != 0 {
		sort.Strings(widening)
		result.Excludable, result.Widened = []string{}, widening[0]
	}
	return result
}
