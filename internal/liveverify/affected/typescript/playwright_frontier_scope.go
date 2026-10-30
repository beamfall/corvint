package typescript

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightAxisInformational marks a shared TypeScript graph frontier reason that cannot change
// the Playwright selection, reported beside the plan without widening it (TJAA-V0-109).
const PlaywrightAxisInformational = "INFORMATIONAL"

// playwrightInformationalDetail explains why an informational frontier reason does not widen
// the plan (TJAA-V0-109).
const playwrightInformationalDetail = "; no site is dirty, in the Playwright closure, in a unit a dirty path reaches, or named by one of them, so it cannot change this selection"

// playwrightInformationalFrontier returns the selection frontier reasons that cannot change the
// Playwright answer (TJAA-V0-108). A reason is relevant, and so absent from the result, when it
// has no located site, when closure membership is itself bounded, or when any of its sites:
//   - is a dirty file;
//   - is owned by a unit in the Playwright closure or in a unit a dirty path reaches;
//   - is a file outside every unit that a closure unit or a dirty-reached unit may name: some
//     closure unit keeps no path tokens over the bound, or the reach of the reason's unowned
//     files meets either set;
//   - for a reason other than a cross-language test asset, is a source file a Playwright project
//     would load as a test, or a config-like file in the root or in an ancestor directory of the
//     Playwright config or of a closure file, which may configure resolution for them.
//
// Every other reason sits only in units outside the closure that no dirty path reaches: such a
// site can add an edge only out of a unit nothing the plan runs depends on, so it cannot add a
// unit to the closure or a test to the selection.
func playwrightInformationalFrontier(closure *playwrightClosure, frontier []string, sites frontierSet, dirty []string, projects []PlaywrightProject, root, globalTestDir string) map[string]bool {
	closure.members()
	if closure.bound != "" || len(frontier) == 0 {
		return nil
	}
	graph := closure.graph
	relevant := map[string]bool{}
	for _, id := range sortedKeys(closure.units) {
		relevant[id] = true
	}
	for _, id := range graph.ReachedUnitIDs(dirty) {
		relevant[id] = true
	}
	dirtySet := map[string]bool{}
	for _, relative := range dirty {
		dirtySet[relative] = true
	}
	tokensBounded := false
	directories := map[string]bool{".": true}
	addAncestors := func(relative string) {
		for directory := path.Dir(relative); !directories[directory]; directory = path.Dir(directory) {
			directories[directory] = true
		}
	}
	addAncestors(closure.configPath)
	for id := range closure.units {
		unit, _ := graph.Unit(id)
		tokensBounded = tokensBounded || unit.PathTokensBounded
		for _, relative := range append(slices.Clone(unit.Sources), unit.Tests...) {
			addAncestors(relative)
		}
	}
	informational := map[string]bool{}
	for _, reason := range frontier {
		located := orderedSites(sites[reason])
		if len(located) == 0 {
			continue
		}
		matters := false
		var unowned []string
		for _, site := range located {
			file := playwrightSiteFile(site)
			if dirtySet[file] {
				matters = true
				break
			}
			if id, owned := graph.OwnerOf(file); owned {
				if relevant[id] {
					matters = true
					break
				}
				continue
			}
			if reason != FrontierCrossLanguageTestAsset && playwrightFrontierFileMayConfigure(root, file, projects, globalTestDir, directories) {
				matters = true
				break
			}
			unowned = append(unowned, file)
		}
		if !matters && len(unowned) != 0 {
			matters = tokensBounded || slices.ContainsFunc(graph.ReachedUnitIDs(unowned), func(id string) bool { return relevant[id] })
		}
		if !matters {
			informational[reason] = true
		}
	}
	return informational
}

// playwrightFrontierFileMayConfigure reports whether a frontier site outside every unit may still
// decide what the Playwright run loads: a source file some project would load as a test (every
// file when no project resolved), or a non-source file, such as a tsconfig.json or package.json,
// in a directory that holds or is an ancestor of the config or a closure file (TJAA-V0-108).
func playwrightFrontierFileMayConfigure(root, file string, projects []PlaywrightProject, globalTestDir string, directories map[string]bool) bool {
	if hasSourceExtension(file) {
		if len(projects) == 0 {
			return true
		}
		return slices.ContainsFunc(projects, func(project PlaywrightProject) bool {
			return playwrightProjectOwns(root, project, globalTestDir, file)
		})
	}
	return directories[path.Dir(file)]
}

// playwrightInformationalUnknowns lists the informational reasons with their sites, in frontier
// order (TJAA-V0-109).
func playwrightInformationalUnknowns(frontier []string, sites frontierSet, informational map[string]bool) []PlaywrightUnknown {
	var listed []PlaywrightUnknown
	for _, reason := range frontier {
		if !informational[reason] {
			continue
		}
		detail := frontierDetail(sites[reason]) + playwrightInformationalDetail
		if reason == FrontierOverBoundSource {
			// The remedy stays visible though the site does not widen (AFP-V0-444).
			detail += "; " + affected.SourceBoundRemedy
		}
		listed = append(listed, PlaywrightUnknown{Axis: PlaywrightAxisInformational, Reason: reason, Detail: detail})
	}
	return listed
}

// relevantPlaywrightFrontier is frontier without its informational reasons.
func relevantPlaywrightFrontier(frontier []string, informational map[string]bool) []string {
	var kept []string
	for _, reason := range frontier {
		if !informational[reason] {
			kept = append(kept, reason)
		}
	}
	return kept
}

// PlaywrightUnknownOutOfSuiteSource is the selection unknown of a dirty source that reaches no
// Playwright test by a static edge: an end-to-end run may still load it through the application
// under test, so the plan cannot exclude it (TJAA-V0-106).
const PlaywrightUnknownOutOfSuiteSource = "playwright:out-of-suite-source"

// playwrightOutOfSuiteBound caps the paths one out-of-suite detail names; the rest are counted.
const playwrightOutOfSuiteBound = 8

// playwrightOutOfSuiteSources returns one selection unknown for the dirty paths of every changed
// unit from which no Playwright test is reachable (NO_SELECTABLE_TEST), and one for the dirty
// paths outside the TypeScript graph (UNOWNED_DIRTY_PATH), whatever their language, unless the
// repository declares each such path in PlaywrightUnreachedPathsPath and the plan's evidence
// leaves it out of the run, plus the record of that declaration when it matched a dirty path
// (TJAA-V0-106, TJAA-V0-107, TJAA-V0-121). A declared graph path the closure may load, or a
// declared path outside the graph that playwrightUnownedMayLoad cannot rule out, refuses the
// declaration. applied holds the declared paths outside the graph the plan applied, which no
// longer leave the shared graph frontier unresolved.
func playwrightOutOfSuiteSources(root string, closure *playwrightClosure, base affected.Plan, dirty []string, projects []PlaywrightProject, globalTestDir string) (record *PlaywrightPathDeclaration, unknown []PlaywrightUnknown, applied map[string]bool, err error) {
	graph := closure.graph
	var unreached, unowned []string
	for _, entry := range base.Unknown {
		switch entry.Reason {
		case affected.UnknownUnownedDirtyPath:
			unowned = append(unowned, entry.Detail)
		case affected.UnknownNoSelectableTest:
			owned := false
			for _, relative := range dirty {
				if id, found := graph.OwnerOf(relative); found && id == entry.Detail {
					unreached = append(unreached, relative)
					owned = true
				}
			}
			if !owned {
				unreached = append(unreached, entry.Detail)
			}
		}
	}
	if len(unreached)+len(unowned) == 0 {
		return nil, nil, nil, nil
	}
	unreached, unowned = sortedUnique(unreached), sortedUnique(unowned)
	digest, entries, err := readPathDeclaration(affected.DiskSource(root), PlaywrightUnreachedPathsPath, PlaywrightUnreachedPathsProfile)
	if err != nil {
		return nil, nil, nil, err
	}
	var open, openUnowned, declaredUnowned, refused []string
	match := func(relative string) bool {
		if entries == nil || !affected.IgnoredMatch(entries, relative) {
			return false
		}
		if record == nil {
			record = &PlaywrightPathDeclaration{Path: PlaywrightUnreachedPathsPath, SHA256: digest, State: PlaywrightDeclarationApplied}
		}
		record.Matched = append(record.Matched, relative)
		return true
	}
	for _, relative := range unreached {
		if !match(relative) {
			open = append(open, relative)
			continue
		}
		if why, inside := closure.member(relative); inside {
			refused = append(refused, relative+" ("+why+")")
		}
	}
	for _, relative := range unowned {
		if match(relative) {
			declaredUnowned = append(declaredUnowned, relative)
		} else {
			openUnowned = append(openUnowned, relative)
		}
	}
	if len(declaredUnowned) != 0 {
		reasons, err := playwrightUnownedMayLoad(root, closure, declaredUnowned, projects, globalTestDir)
		if err != nil {
			return nil, nil, nil, err
		}
		applied = map[string]bool{}
		for _, relative := range declaredUnowned {
			if why, found := reasons[relative]; found {
				refused = append(refused, relative+" ("+why+")")
			} else {
				applied[relative] = true
			}
		}
	}
	if record != nil {
		slices.Sort(record.Matched)
		if len(refused) != 0 {
			record.State = PlaywrightDeclarationRefused
		}
	}
	if len(open) != 0 {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownOutOfSuiteSource, boundedPathList(open)+
			" reach(es) no Playwright test by a static edge; an end-to-end run may still load it through the application under test, so the plan cannot exclude it; declare it in "+
			PlaywrightUnreachedPathsPath+" when the suite cannot reach it"))
	}
	if len(openUnowned) != 0 {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownOutOfSuiteSource, boundedPathList(openUnowned)+
			" is (are) outside the TypeScript graph, so no static edge shows whether the suite or the application under test loads it; an end-to-end run may still exercise it, so the plan cannot exclude it; declare it in "+
			PlaywrightUnreachedPathsPath+" when the suite cannot reach it"))
	}
	if len(refused) != 0 {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownOutOfSuiteSource, PlaywrightUnreachedPathsPath+" (sha256:"+digest+") declares "+
			boundedPathList(refused)+" unreached by the suite, but the Playwright closure may load it"))
	}
	return record, unknown, applied, nil
}

// playwrightUnownedMayLoad returns, for each declared dirty path outside the TypeScript graph
// that the Playwright run may still load, why (TJAA-V0-121). No graph edge can show such a path
// is out of the run, so the declaration applies only when none of these holds:
//   - the path is the Playwright config or the declaration itself;
//   - Playwright closure membership is bounded or incomplete;
//   - the path lies in a project's test directory, where tests keep snapshots and data;
//   - the path may configure the run (playwrightFrontierFileMayConfigure), such as a
//     package.json or tsconfig.json in a directory holding the config or a closure file;
//   - an observed source in the closure, or one whose membership is unknown, names the path as a
//     string (declaredPathNamed), as a spec reading a fixture or spawning a server script does.
func playwrightUnownedMayLoad(root string, closure *playwrightClosure, declared []string, projects []PlaywrightProject, globalTestDir string) (map[string]string, error) {
	reasons := map[string]string{}
	if !closure.built {
		closure.build()
	}
	directories := map[string]bool{".": true}
	addAncestors := func(relative string) {
		for directory := path.Dir(relative); !directories[directory]; directory = path.Dir(directory) {
			directories[directory] = true
		}
	}
	addAncestors(closure.configPath)
	for id := range closure.units {
		unit, _ := closure.graph.Unit(id)
		for _, relative := range append(slices.Clone(unit.Sources), unit.Tests...) {
			addAncestors(relative)
		}
	}
	var unnamed []string
	for _, relative := range declared {
		testDir := playwrightUnderTestDir(relative, projects, globalTestDir)
		switch {
		case relative == closure.configPath:
			reasons[relative] = "the Playwright config"
		case relative == PlaywrightUnreachedPathsPath:
			reasons[relative] = "the declaration itself"
		case closure.unknown != "":
			reasons[relative] = "Playwright closure membership is unknown: " + closure.unknown
		case testDir != "":
			reasons[relative] = "inside Playwright test directory " + testDir
		case playwrightFrontierFileMayConfigure(root, relative, projects, globalTestDir, directories):
			reasons[relative] = "may configure what the Playwright run loads"
		default:
			unnamed = append(unnamed, relative)
		}
	}
	if len(unnamed) == 0 {
		return reasons, nil
	}
	files, err := affected.SourceFiles(root, playwrightObservedName)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(files, closure.configPath) {
		files = append(files, closure.configPath)
		slices.Sort(files)
	}
	for _, relative := range files {
		if len(reasons) == len(declared) {
			break
		}
		body, err := affected.ReadSource(root, relative)
		if err != nil {
			return nil, err
		}
		text := string(body)
		decoded, rawOffset := decodeStringEscapes(text)
		for _, candidate := range unnamed {
			if _, found := reasons[candidate]; found || relative == candidate {
				continue
			}
			offset, spelling := declaredPathNamed(decoded, candidate)
			if offset < 0 {
				continue
			}
			if why, inside := closure.member(relative); inside {
				reasons[candidate] = lineSite(relative, text, rawOffset(offset)) + " `" + spelling + "` names it (" + why + ")"
			}
		}
	}
	return reasons, nil
}

// playwrightUnderTestDir returns the test directory of the first project, in config order, that
// holds relative, or "" when none does (TJAA-V0-121). Without a resolved project every path is
// treated as inside the global test directory.
func playwrightUnderTestDir(relative string, projects []PlaywrightProject, globalTestDir string) string {
	if globalTestDir == "" {
		globalTestDir = "."
	}
	if len(projects) == 0 {
		return globalTestDir
	}
	for _, project := range projects {
		directory := project.TestDir
		if directory == "" {
			directory = globalTestDir
		}
		if directory == "." || inside(relative, directory) {
			return directory
		}
	}
	return ""
}

// boundedPathList joins at most playwrightOutOfSuiteBound values and counts the rest.
func boundedPathList(values []string) string {
	count := min(len(values), playwrightOutOfSuiteBound)
	listed := strings.Join(values[:count], ", ")
	if extra := len(values) - count; extra > 0 {
		listed += " and " + strconv.Itoa(extra) + " more"
	}
	return listed
}
