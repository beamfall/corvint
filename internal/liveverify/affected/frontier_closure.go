// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"slices"
	"sort"
	"strings"
)

// HostedFixtureExclusionReason explains a plugin-wide frontier entry left out
// of Unknown because its site lies in a `testdata` directory a Go unit hosts
// that the change neither touches nor reaches (AZU-V0-211).
const HostedFixtureExclusionReason = "every site lies below a testdata directory a Go unit hosts; no dirty path lies below it, no unit the change propagates through owns a file below it, no dirty path is a resolution input of the plugin, and the reason binds only the plugin's own paths or its runner configuration, so no edge the site hides can carry this change"

// NoRunnerHint is appended to a PACKAGE_RUNNABLE entry whose site unit the
// plugin does not report runnable (AZU-V0-008, AZU-V0-213).
const NoRunnerHint = "NO_RUNNER: no identified runner runs the site's unit, so selecting it cannot run the package's tests"

// terminalReach reports whether a reached unit is a terminal selection for
// the participation rules (AFP-V0-560): a terminal witness, or an unbounded
// reader whose only cause is a runner its tests run (AFP-V0-2991), that no
// traversal carried the change through and whose dependents the plan does
// not follow.
func (graph *Graph) terminalReach(id string, witness Witness, carried, followed map[string]bool) bool {
	_, unbounded := graph.causes[id]
	runner := graph.runnerOnly[id]
	terminalSelection := terminalWitness(witness) || runner && witness.Kind == WitnessUnboundedReader
	return (!unbounded || runner) && !carried[id] && !followed[id] && terminalSelection
}

// frontierDirection is the plugin's classification of an entry's reason
// (AZU-V0-003): FrontierInbound for a plugin without the declaration or a
// reason it does not classify.
func (graph *Graph) frontierDirection(name, entry string) string {
	direction, declares := graph.claimants[name].(FrontierDirection)
	if !declares {
		return FrontierInbound
	}
	fields := strings.SplitN(entry, ":", 3)
	if len(fields) < 2 {
		return FrontierInbound
	}
	switch value := direction.FrontierDirection(fields[0] + ":" + fields[1]); value {
	case FrontierOutbound, FrontierDiscovery, FrontierPackageRunnable:
		return value
	}
	return FrontierInbound
}

// frontierSiteFile is the file of one located site, `file`, `file:line`,
// `file `+"`token`"+` or `file:line `+"`token`"+` (inboundRuntimeSiteFile).
func frontierSiteFile(site string) string {
	return inboundRuntimeSiteFile(site)
}

// frontierEntrySites lists the located sites one plugin-wide entry stands for: the
// entry's own site, or for a `reason:+N` entry every site the plugin recorded
// for the reason (Result.FrontierSites, AZU-V0-212). A bare entry, and a `+N`
// entry of a plugin that recorded no sites, stands for none.
func (graph *Graph) frontierEntrySites(name, entry string) []string {
	fields := strings.SplitN(entry, ":", 3)
	if len(fields) < 3 || fields[2] == "" {
		return nil
	}
	if strings.HasPrefix(fields[2], "+") {
		return graph.frontierSitesBy[name][fields[0]+":"+fields[1]]
	}
	return []string{fields[2]}
}

// frontierSiteUnits maps one plugin-wide entry to the units that own its
// sites' files, sorted and unique. located is false when the entry stands for
// no site or a site's file is owned by no unit of the graph: the entry then
// stays unknown when it bears (AZU-V0-003).
func (graph *Graph) frontierSiteUnits(name, entry string) (units []string, located bool) {
	sites := graph.frontierEntrySites(name, entry)
	if len(sites) == 0 {
		return nil, false
	}
	seen := make(map[string]bool, len(sites))
	for _, site := range sites {
		id, owned := graph.owner[frontierSiteFile(site)]
		if !owned {
			id, owned = graph.owner[site]
		}
		if !owned || !strings.HasPrefix(id, name+":") {
			return nil, false
		}
		if !seen[id] {
			seen[id] = true
			units = append(units, id)
		}
	}
	sort.Strings(units)
	return units, true
}

// siteVerdict is how one site of a plugin-wide entry stands in a plan
// (AZU-V0-212).
type siteVerdict int

const (
	// siteOpen: the site keeps the entry bearing.
	siteOpen siteVerdict = iota
	// siteSelectionClosed: the plan selects the site unit as the reason's
	// direction requires (AZU-V0-003, AZU-V0-008).
	siteSelectionClosed
	// siteFixtureClosed: the site lies in a hosted `testdata` directory the
	// change neither touches nor reaches, and its reason binds nothing
	// outside the fixture or the plan selects the hosting unit (AZU-V0-211).
	siteFixtureClosed
)

// entryVerdict is a plugin-wide entry's standing: closed when every site is
// closed, with the hosting units of its fixture-closed sites and the hint of a
// site the plugin cannot run.
type entryVerdict struct {
	closed  bool
	fixture bool
	hosts   []string
	hint    string
}

// frontierEntryVerdict evaluates one plugin-wide entry site by site
// (AZU-V0-212). A site in a hosted, untouched, unreached `testdata` directory
// is closed by its Go host (siteFixtureClosed) when the reason is source-bound
// or fixture-closed, or when the plan selects the host with its tests. Any
// other site is closed only by selection: an OUTBOUND site when its unit is
// reached and the plan follows its dependents, a DISCOVERY site the same way
// unless the site file is dirty or a dirty path of the plugin lies below the
// site's directory, a PACKAGE_RUNNABLE site when its runnable unit is
// reached. An INBOUND site, an unlocated site and a site the plugin cannot
// run stay open, the last with NoRunnerHint. An entry with no site is open.
// A steered plugin (steeredBy) has no fixture-closed site: a dirty resolution
// input may rebind the fixture's sites, which are then live sites.
func (graph *Graph) frontierEntryVerdict(name, entry string, hosted *hostedFixtureSites, reached map[string]Witness, followed map[string]bool, dirty []string, steered bool) entryVerdict {
	sites := graph.frontierEntrySites(name, entry)
	verdict := entryVerdict{closed: len(sites) != 0}
	if len(sites) == 0 {
		return verdict
	}
	direction := graph.frontierDirection(name, entry)
	fields := strings.SplitN(entry, ":", 3)
	reason := fields[0] + ":" + fields[1]
	local := graph.sourceBound[name][reason] || graph.fixtureClosed[name][reason]
	hosts := make(map[string]bool)
	for _, site := range sites {
		file := graph.siteFile(site)
		if host := hosted.host(file); host != "" && !steered {
			verdict.fixture = true
			if local || graph.selectsTests(host, reached) {
				hosts[host] = true
				continue
			}
			verdict.closed = false
			continue
		}
		id, owned := graph.owner[file]
		if direction == FrontierInbound || !owned || !strings.HasPrefix(id, name+":") {
			verdict.closed = false
			continue
		}
		_, hit := reached[id]
		switch direction {
		case FrontierPackageRunnable:
			if !graph.runnableUnit(name, id) {
				verdict.hint = NoRunnerHint
				verdict.closed = false
			} else if !hit {
				verdict.closed = false
			}
		case FrontierDiscovery:
			if graph.dirtyBelowSite(name, file, dirty) || !hit || !followed[id] {
				verdict.closed = false
			}
		default:
			if !hit || !followed[id] {
				verdict.closed = false
			}
		}
	}
	verdict.hosts = sortedKeys(hosts)
	return verdict
}

// selectsTests reports a reached unit with tests: a selection of the plan.
func (graph *Graph) selectsTests(id string, reached map[string]Witness) bool {
	_, hit := reached[id]
	return hit && len(graph.units[id].Tests) != 0
}

// dirtyBelowSite reports a dirty path that is the site file or a path the
// site's plugin owns below the site file's directory: what a DISCOVERY site
// decides about may have changed with it.
func (graph *Graph) dirtyBelowSite(name, file string, dirty []string) bool {
	directory := relativeDirectory(file)
	for _, path := range dirty {
		if path == file {
			return true
		}
		if directory != "." && !strings.HasPrefix(path, directory+"/") {
			continue
		}
		if id, owned := graph.owner[path]; owned && strings.HasPrefix(id, name+":") {
			return true
		}
	}
	return false
}

// frontierSiteClosed reports whether a bearing plugin-wide entry is closed by
// the plan's selections and hosts (frontierEntryVerdict), with the hint for
// one that stays. The caller passes closable false on a plan with a dirty
// path no plugin owns: that path keeps the scope UNKNOWN whatever the
// frontier says, so a selection could not bound the plan and would only
// widen it (AZU-V0-212).
func (graph *Graph) frontierSiteClosed(name, entry string, hosted *hostedFixtureSites, reached map[string]Witness, followed map[string]bool, dirty []string, closable bool) (closed bool, hint string) {
	if !closable || !slices.Contains(graph.frontierBy[name], entry) {
		return false, ""
	}
	verdict := graph.frontierEntryVerdict(name, entry, hosted, reached, followed, dirty, graph.steeredBy(name, dirty))
	return verdict.closed, verdict.hint
}

// runnableUnit asks the plugin whether selecting the unit runs its tests
// through an identified runner (RunnableUnits); a plugin without the method
// runs nothing this way, so its PACKAGE_RUNNABLE entries stay unknown.
func (graph *Graph) runnableUnit(name, id string) bool {
	runnable, declares := graph.claimants[name].(RunnableUnits)
	return declares && len(graph.units[id].Tests) != 0 && runnable.RunnableUnit(id)
}

// frontierSiteClosure selects, for each bearing plugin-wide LANGUAGE_FRONTIER
// entry, what closes each of its open sites (AZU-V0-003, AZU-V0-008,
// AZU-V0-212): for an OUTBOUND or DISCOVERY site outside every hosted fixture,
// its unit with witness FRONTIER_SITE, traversed from as from a changed unit
// (dependents, test users, users of the commands it builds, units that embed
// its files and readers of its files), because whatever path the hidden edge
// binds, only the site unit and what observes it can see the change; for a
// PACKAGE_RUNNABLE site, its runnable unit for its tests alone; for a site in
// a hosted `testdata` directory the change neither touches nor reaches, the
// hosting Go unit for its tests alone, instead of walking into the fixture
// (AZU-V0-211). The caller passes only the entries a selection may close: on
// a plan with a dirty path no plugin owns there are none. It returns whether it
// reached or followed a unit the plan had not, so the caller recomputes the
// bearing frontier to a fixed point.
func (graph *Graph) frontierSiteClosure(closable []Unknown, reached map[string]Witness, carried, followed map[string]bool, dirty []string) bool {
	if len(dirty) == 0 {
		return false
	}
	hosted := graph.hostedFixtures(reached, carried, followed, dirty)
	changed := false
	seeds := make(map[string]Witness)
	var hosts []string
	for _, entry := range closable {
		if entry.Reason != UnknownLanguageFrontier {
			continue
		}
		name, _, _ := strings.Cut(entry.Detail, ":")
		if !slices.Contains(graph.frontierBy[name], entry.Detail) {
			continue
		}
		direction := graph.frontierDirection(name, entry.Detail)
		fields := strings.SplitN(entry.Detail, ":", 3)
		if len(fields) < 3 {
			continue
		}
		local := graph.sourceBound[name][fields[0]+":"+fields[1]] || graph.fixtureClosed[name][fields[0]+":"+fields[1]]
		steered := graph.steeredBy(name, dirty)
		for _, site := range graph.frontierEntrySites(name, entry.Detail) {
			file := graph.siteFile(site)
			if host := hosted.host(file); host != "" && !steered {
				if !local && !graph.selectsTests(host, reached) && len(graph.units[host].Tests) != 0 {
					hosts = append(hosts, host)
				}
				continue
			}
			id, owned := graph.owner[file]
			if direction == FrontierInbound || !owned || !strings.HasPrefix(id, name+":") {
				continue
			}
			switch direction {
			case FrontierPackageRunnable:
				if _, hit := reached[id]; !hit && graph.runnableUnit(name, id) {
					reached[id] = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{id}}
					changed = true
				}
			case FrontierDiscovery:
				if graph.dirtyBelowSite(name, file, dirty) {
					continue
				}
				fallthrough
			default:
				if !followed[id] {
					witness, hit := reached[id]
					if !hit {
						witness = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{id}}
					}
					seeds[id] = witness
				}
			}
		}
	}
	// The hosting Go unit's tests are the only run of a hosted fixture
	// (AFP-V0-446), so they observe whatever a fixture site's hidden edge
	// binds; the host is selected for its tests alone, as a reader is.
	sort.Strings(hosts)
	for _, host := range slices.Compact(hosts) {
		if _, hit := reached[host]; !hit {
			reached[host] = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{host}}
			changed = true
		}
	}
	if len(seeds) == 0 {
		return changed
	}
	closure := graph.traverse(seeds, nil)
	graph.testUsersOf(closure, nil)
	graph.execUsersOf(closure, graph.builtCommands(closure, nil))
	for id, witness := range closure {
		if _, hit := reached[id]; !hit {
			reached[id] = witness
			changed = true
		}
		if !followed[id] {
			followed[id] = true
			changed = true
		}
	}
	// What reads or embeds a site unit's files observes the hidden edge's
	// effect as it would a change to those files: a Go test that runs a
	// helper script, a package that embeds it.
	seeded := make([]string, 0, len(seeds))
	for id := range seeds {
		seeded = append(seeded, id)
	}
	sort.Strings(seeded)
	for _, id := range seeded {
		files := append(append([]string(nil), graph.units[id].Sources...), graph.units[id].Tests...)
		site := reached[id]
		if site.link == nil && len(site.Via) == 0 {
			site.Via = []string{id}
		}
		if graph.observersOf(files, site, reached, followed) {
			changed = true
		}
	}
	return changed
}

// observersOf selects the units that read or embed the given files of a
// frontier site, as a change to those files would (AZU-V0-212): readers are
// selected for their tests, as readers of a dirty path are (AFP-V0-581 (2)),
// with the site's dirty path and a chain through the site; an embedding unit
// compiles the file, so it and its dependents are followed. It reports
// whether it reached or followed a unit the plan had not.
func (graph *Graph) observersOf(files []string, site Witness, reached map[string]Witness, followed map[string]bool) bool {
	if len(files) == 0 {
		return false
	}
	changed := false
	embedded := make(map[string]Witness)
	for _, file := range files {
		for _, embedder := range graph.embedded[file] {
			if _, hit := reached[embedder]; !hit {
				embedded[embedder] = Witness{Kind: WitnessDependency, DirtyPath: site.DirtyPath, link: site.extended(embedder)}
			}
		}
	}
	if len(embedded) != 0 {
		walked := graph.traverse(embedded, nil)
		graph.testUsersOf(walked, nil)
		// A command that embeds the file is rebuilt, so the tests that run
		// its binary observe the change as they do in the site closure.
		graph.execUsersOf(walked, graph.builtCommands(walked, nil))
		for dependent, witness := range walked {
			if _, hit := reached[dependent]; !hit {
				reached[dependent] = witness
				changed = true
			}
			if !followed[dependent] {
				followed[dependent] = true
				changed = true
			}
		}
	}
	readers := make(map[string]Witness, len(reached))
	for existing, witness := range reached {
		readers[existing] = witness
	}
	graph.readers(readers, files)
	for reader, witness := range readers {
		if _, hit := reached[reader]; hit {
			continue
		}
		witness.DirtyPath = site.DirtyPath
		witness.Via, witness.link = nil, site.extended(reader)
		reached[reader] = witness
		changed = true
	}
	return changed
}

// hostedFixtureSites is the per-plan state of the hosted-fixture rule
// (AZU-V0-211): the fixture roots a dirty path lies below or a unit the
// change propagates through owns a file below, and the Go packages by
// directory.
type hostedFixtureSites struct {
	graph    *Graph
	packages map[string]string
	touched  map[string]bool
}

// fixtureRoot is the fixture a path belongs to: the directory directly below
// its outermost `testdata` component (`internal/x/testdata/suite` for
// `internal/x/testdata/suite/e2e/a.spec.ts`), or the `testdata` directory
// itself for a file directly in it; "" when the path has no `testdata`
// component (AFP-V0-446). Sibling fixtures of one package are distinct roots,
// so a change inside one does not make another's sites bear.
func fixtureRoot(relative string) string {
	directory := fixtureDirectory(relative)
	if directory == "" {
		return ""
	}
	rest := strings.TrimPrefix(relative, directory+"/")
	if next, _, nested := strings.Cut(rest, "/"); nested {
		return directory + "/" + next
	}
	return directory
}

func (graph *Graph) hostedFixtures(reached map[string]Witness, carried, followed map[string]bool, dirty []string) *hostedFixtureSites {
	state := &hostedFixtureSites{graph: graph, packages: graph.goPackageDirectories(), touched: make(map[string]bool)}
	for _, path := range dirty {
		if root := fixtureRoot(path); root != "" {
			state.touched[root] = true
		}
	}
	for id, witness := range reached {
		if graph.terminalReach(id, witness, carried, followed) {
			continue
		}
		unit := graph.units[id]
		for _, path := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			if root := fixtureRoot(path); root != "" {
				state.touched[root] = true
			}
		}
	}
	return state
}

// host returns the Go unit hosting the fixture root of file when the change
// neither touches nor reaches that root, or "" when the file is in no
// fixture, the root is touched or reached, or no Go unit lies above it.
func (state *hostedFixtureSites) host(file string) string {
	root := fixtureRoot(file)
	if state == nil || root == "" || state.touched[root] {
		return ""
	}
	for parent := relativeDirectory(fixtureDirectory(file)); ; parent = relativeDirectory(parent) {
		if id, present := state.packages[parent]; present {
			return id
		}
		if parent == "." {
			return ""
		}
	}
}

// siteFile is the file of a site, taking the whole site text when a unit
// owns it as such (a line-less site whose name ends in a backquoted word).
func (graph *Graph) siteFile(site string) string {
	file := frontierSiteFile(site)
	if _, owned := graph.owner[file]; !owned {
		if _, whole := graph.owner[site]; whole {
			return site
		}
	}
	return file
}
