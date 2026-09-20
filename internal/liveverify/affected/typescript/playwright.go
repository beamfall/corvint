package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const (
	PlaywrightProfile           = "playwright-affected/0"
	PlaywrightFallbackNone      = "NONE"
	PlaywrightFallbackFullSuite = "FULL_RELEVANT_SUITE"
	PlaywrightAxisSelection     = "SELECTION"
	PlaywrightAxisExecution     = "EXECUTION"
)

const (
	PlaywrightUnknownConfigSyntax        = "playwright:config-syntax-unresolved"
	PlaywrightUnknownProjectSet          = "playwright:project-set-unresolved"
	PlaywrightUnknownProjectMembership   = "playwright:project-membership-unresolved"
	PlaywrightUnknownProjectDependency   = "playwright:project-dependency-unresolved"
	PlaywrightUnknownBrowserIdentity     = "playwright:browser-identity-unresolved"
	PlaywrightUnknownDynamicSource       = "playwright:dynamic-source-reachability"
	PlaywrightUnknownExternalApplication = "playwright:external-application-state"
)

type PlaywrightConfigIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type PlaywrightProject struct {
	Argv           []string `json:"argv"`
	Browser        string   `json:"browser"`
	ConfigFragment string   `json:"configFragmentSha256"`
	Dependencies   []string `json:"dependencies"`
	Device         string   `json:"device"`
	Grep           string   `json:"grep"`
	GrepInvert     string   `json:"grepInvert"`
	Metadata       string   `json:"metadataSha256"`
	Name           string   `json:"name"`
	Teardown       string   `json:"teardown"`
	TestDir        string   `json:"testDir"`
	TestIgnore     []string `json:"testIgnore"`
	TestMatch      []string `json:"testMatch"`

	matchers []playwrightMatcher
	ignores  []playwrightMatcher
}

type PlaywrightWitness struct {
	DirtyPath string   `json:"dirtyPath"`
	Kind      string   `json:"kind"`
	Via       []string `json:"via"`
}

type PlaywrightSelection struct {
	Argv    []string          `json:"argv"`
	Browser string            `json:"browser"`
	Device  string            `json:"device"`
	ID      string            `json:"id"`
	Project string            `json:"project"`
	Test    string            `json:"test"`
	Witness PlaywrightWitness `json:"witness"`
}

type PlaywrightExclusion struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Reason  string `json:"reason"`
	Test    string `json:"test"`
}

type PlaywrightUnknown struct {
	Axis   string `json:"axis"`
	Detail string `json:"detail"`
	Reason string `json:"reason"`
}

type PlaywrightPlan struct {
	Config       PlaywrightConfigIdentity `json:"config"`
	Dirty        []string                 `json:"dirty"`
	Excluded     []PlaywrightExclusion    `json:"excluded"`
	Fallback     string                   `json:"fallback"`
	GraphDigest  string                   `json:"graphDigest"`
	Projects     []PlaywrightProject      `json:"projects"`
	Scope        string                   `json:"scope"`
	Selected     []PlaywrightSelection    `json:"selected"`
	SourceDigest string                   `json:"sourceDigest"`
	Unknown      []PlaywrightUnknown      `json:"unknown"`
}

type playwrightMatcher struct {
	raw string
	re  *regexp.Regexp
}

type observedTypeScript struct {
	result affected.Result
}

func (observedTypeScript) Name() string              { return "typescript" }
func (observedTypeScript) Owns(relative string) bool { return New().Owns(relative) }
func (language observedTypeScript) Units(string) (affected.Result, error) {
	return language.result, nil
}

// SelectPlaywright expands the shared TypeScript path graph into one runnable
// unit per physical Playwright test and statically applicable project. It never
// executes JavaScript or the Playwright package.
func SelectPlaywright(root, configPath string, dirty []string) (PlaywrightPlan, error) {
	if !affected.ValidRelativePath(configPath) || !hasSourceExtension(configPath) {
		return PlaywrightPlan{}, errors.New("playwright config path is not canonical TypeScript/JavaScript source")
	}
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil || !utf8.Valid(configBytes) {
		return PlaywrightPlan{}, errors.New("playwright config is unreadable")
	}
	configSum := sha256.Sum256(configBytes)
	projects, globalTestDir, configUnknown := parsePlaywrightConfig(configPath, string(configBytes))
	result, err := New().units(root, true)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	configUnknown = append(configUnknown, bindPlaywrightGlobalHooks(configPath, string(configBytes), &result)...)
	selectionUnknown, executionUnknown := classifyPlaywrightFrontier(result.Frontier)
	selectionUnknown = append(selectionUnknown, configUnknown...)
	filtered := result
	filtered.Frontier = nil
	for _, reason := range result.Frontier {
		if reason != FrontierConfig && reason != FrontierE2ERuntimeDependency {
			filtered.Frontier = append(filtered.Frontier, reason)
		}
	}
	graph, err := affected.Build(root, observedTypeScript{result: filtered})
	if err != nil {
		return PlaywrightPlan{}, err
	}
	normalizedDirty := affected.NormalizePaths(dirty)
	base := affected.Select(graph, normalizedDirty)
	if base.Scope == affected.ScopeUnknown {
		selectionUnknown = append(selectionUnknown, PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: PlaywrightUnknownDynamicSource, Detail: "the shared TypeScript graph has an unresolved selection frontier"})
	}
	discoveredTests := playwrightDiscoveredTests(result)
	tests := playwrightTests(root, discoveredTests, projects, globalTestDir)
	units := playwrightUnits(root, configPath, projects, globalTestDir, tests)
	selected := selectPlaywrightUnits(configPath, base, projects, units, normalizedDirty)
	selectionUnknown = canonicalPlaywrightUnknowns(selectionUnknown)
	if hasPlaywrightUnknownReason(selectionUnknown, PlaywrightUnknownProjectSet) {
		units = []PlaywrightSelection{}
		selected = []PlaywrightSelection{}
	} else if len(selectionUnknown) != 0 {
		units = allPlaywrightProjectUnits(configPath, projects, discoveredTests)
		selected = allPlaywrightUnits(units)
	}
	selected = expandPlaywrightProjectEdges(selected, projects, units)
	unknown := append(selectionUnknown, executionUnknown...)
	unknown = append(unknown, PlaywrightUnknown{Axis: PlaywrightAxisExecution, Reason: PlaywrightUnknownExternalApplication, Detail: "application state and runtime feature flags are not observed by static affected selection"})
	unknown = canonicalPlaywrightUnknowns(unknown)
	plan := PlaywrightPlan{
		Config: PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(configSum[:])},
		Dirty:  normalizedDirty, Excluded: []PlaywrightExclusion{}, Fallback: PlaywrightFallbackNone,
		Projects: publicPlaywrightProjects(projects), Scope: affected.ScopeBounded,
		Selected: selected, SourceDigest: sourceDigest, Unknown: unknown,
	}
	if len(selectionUnknown) != 0 {
		plan.Scope = affected.ScopeUnknown
		plan.Fallback = PlaywrightFallbackFullSuite
	}
	plan.Excluded = excludedPlaywrightUnits(units, selected)
	plan.GraphDigest, err = playwrightGraphDigest(graph.Digest(), plan.Config, plan.Projects, units)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	after, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return PlaywrightPlan{}, err
	}
	if after != sourceDigest {
		return PlaywrightPlan{}, errors.New("Playwright source changed while the plan was compiled")
	}
	return plan, nil
}

// ObservePlaywrightSources returns a bounded content identity for every input
// the TypeScript observer may use plus the explicitly selected config.
func ObservePlaywrightSources(root, configPath string) (string, error) {
	files, err := affected.SourceFiles(root, observedName)
	if err != nil {
		return "", err
	}
	if !containsString(files, configPath) {
		files = append(files, configPath)
		sort.Strings(files)
	}
	type sourceIdentity struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	identities := make([]sourceIdentity, 0, len(files))
	for _, relative := range files {
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil {
			return "", readErr
		}
		sum := sha256.Sum256(body)
		identities = append(identities, sourceIdentity{Path: relative, SHA256: hex.EncodeToString(sum[:])})
	}
	body, err := json.Marshal(identities, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "playwright-sources:sha256:" + hex.EncodeToString(sum[:]), nil
}

func hasPlaywrightUnknownReason(unknown []PlaywrightUnknown, reason string) bool {
	for _, entry := range unknown {
		if entry.Reason == reason {
			return true
		}
	}
	return false
}

func classifyPlaywrightFrontier(frontier []string) (selection, execution []PlaywrightUnknown) {
	for _, reason := range frontier {
		unknown := PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: reason, Detail: "shared TypeScript graph frontier"}
		switch reason {
		case FrontierE2ERuntimeDependency:
			unknown.Axis = PlaywrightAxisExecution
			execution = append(execution, unknown)
		case FrontierConfig:
			// The selected Playwright config is parsed by the closed profile below.
		default:
			selection = append(selection, unknown)
		}
	}
	return selection, execution
}

func playwrightDiscoveredTests(result affected.Result) []string {
	set := map[string]bool{}
	for _, unit := range result.Units {
		if !strings.HasPrefix(unit.ID, "typescript:"+runnerPlaywright+":") {
			continue
		}
		for _, test := range unit.Tests {
			set[test] = true
		}
	}
	return sortedKeys(set)
}

func playwrightTests(root string, discovered []string, projects []PlaywrightProject, globalTestDir string) []string {
	tests := make([]string, 0, len(discovered))
	for _, test := range discovered {
		if playwrightDefaultTest(test) || explicitPlaywrightTest(root, test) || playwrightAnyProjectOwns(root, projects, globalTestDir, test) {
			tests = append(tests, test)
		}
	}
	return tests
}

func playwrightAnyProjectOwns(root string, projects []PlaywrightProject, globalTestDir, test string) bool {
	for _, project := range projects {
		if playwrightProjectOwns(root, project, globalTestDir, test) {
			return true
		}
	}
	return false
}

func explicitPlaywrightTest(root, relative string) bool {
	body, err := affected.ReadSource(root, relative)
	if err != nil || !utf8.Valid(body) {
		return false
	}
	refs, _, parseErr := scanImports(relative, string(body))
	return parseErr == nil && explicitRunners(string(body), refs)[runnerPlaywright]
}

func playwrightUnits(root, configPath string, projects []PlaywrightProject, globalTestDir string, tests []string) []PlaywrightSelection {
	units := []PlaywrightSelection{}
	for _, project := range projects {
		for _, test := range tests {
			if !playwrightProjectOwns(root, project, globalTestDir, test) {
				continue
			}
			units = append(units, newPlaywrightSelection(configPath, project, test, PlaywrightWitness{}))
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	return units
}

func allPlaywrightProjectUnits(configPath string, projects []PlaywrightProject, tests []string) []PlaywrightSelection {
	units := make([]PlaywrightSelection, 0, len(projects)*len(tests))
	for _, project := range projects {
		for _, test := range tests {
			units = append(units, newPlaywrightSelection(configPath, project, test, PlaywrightWitness{}))
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	return units
}

func newPlaywrightSelection(configPath string, project PlaywrightProject, test string, witness PlaywrightWitness) PlaywrightSelection {
	argv := []string{"npx", "playwright", "test", test, "--config=" + configPath, "--project=" + project.Name}
	return PlaywrightSelection{
		Argv: argv, Browser: project.Browser, Device: project.Device,
		ID:      "typescript:playwright:" + playwrightIDEscape(project.Name) + ":" + test,
		Project: project.Name, Test: test, Witness: witness,
	}
}

func selectPlaywrightUnits(configPath string, base affected.Plan, projects []PlaywrightProject, units []PlaywrightSelection, dirty []string) []PlaywrightSelection {
	all := map[string]PlaywrightSelection{}
	for _, unit := range units {
		all[unit.ID] = unit
	}
	selected := map[string]PlaywrightSelection{}
	if containsString(dirty, configPath) {
		for id, unit := range all {
			unit.Witness = PlaywrightWitness{Kind: "CONFIG_CHANGE", DirtyPath: configPath, Via: []string{configPath}}
			selected[id] = unit
		}
		return sortedPlaywrightSelections(selected)
	}
	for _, physical := range base.Selected {
		for _, test := range physical.Tests {
			for _, project := range projects {
				id := "typescript:playwright:" + playwrightIDEscape(project.Name) + ":" + test
				unit, exists := all[id]
				if !exists {
					continue
				}
				unit.Witness = PlaywrightWitness{Kind: physical.Witness.Kind, DirtyPath: physical.Witness.DirtyPath, Via: append([]string(nil), physical.Witness.Via...)}
				selected[id] = unit
			}
		}
	}
	return sortedPlaywrightSelections(selected)
}

func allPlaywrightUnits(units []PlaywrightSelection) []PlaywrightSelection {
	result := append([]PlaywrightSelection(nil), units...)
	for index := range result {
		result[index].Witness = PlaywrightWitness{Kind: "FULL_SUITE_WIDENING", Via: []string{}}
	}
	return result
}

func expandPlaywrightProjectEdges(selected []PlaywrightSelection, projects []PlaywrightProject, units []PlaywrightSelection) []PlaywrightSelection {
	byProject := map[string][]PlaywrightSelection{}
	for _, unit := range units {
		byProject[unit.Project] = append(byProject[unit.Project], unit)
	}
	projectByName := map[string]PlaywrightProject{}
	dependents := map[string][]string{}
	for _, project := range projects {
		projectByName[project.Name] = project
		for _, dependency := range project.Dependencies {
			dependents[dependency] = append(dependents[dependency], project.Name)
		}
	}
	result := map[string]PlaywrightSelection{}
	type queuedProject struct {
		name             string
		expandDependents bool
	}
	queue := []queuedProject{}
	for _, unit := range selected {
		result[unit.ID] = unit
		queue = append(queue, queuedProject{name: unit.Project, expandDependents: true})
	}
	seenProject := map[queuedProject]bool{}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if seenProject[current] {
			continue
		}
		seenProject[current] = true
		project := projectByName[current.name]
		type relation struct {
			name             string
			expandDependents bool
		}
		related := make([]relation, 0, len(project.Dependencies)+len(dependents[current.name])+1)
		for _, dependency := range project.Dependencies {
			related = append(related, relation{name: dependency})
		}
		if current.expandDependents {
			for _, dependent := range dependents[current.name] {
				related = append(related, relation{name: dependent, expandDependents: true})
			}
		}
		if project.Teardown != "" {
			related = append(related, relation{name: project.Teardown})
		}
		for _, next := range related {
			queue = append(queue, queuedProject{name: next.name, expandDependents: next.expandDependents})
			for _, unit := range byProject[next.name] {
				if _, exists := result[unit.ID]; exists {
					continue
				}
				unit.Witness = PlaywrightWitness{Kind: "PROJECT_RELATION", DirtyPath: current.name, Via: []string{current.name, next.name}}
				result[unit.ID] = unit
			}
		}
	}
	return sortedPlaywrightSelections(result)
}

func excludedPlaywrightUnits(units, selected []PlaywrightSelection) []PlaywrightExclusion {
	chosen := map[string]bool{}
	for _, unit := range selected {
		chosen[unit.ID] = true
	}
	excluded := []PlaywrightExclusion{}
	for _, unit := range units {
		if !chosen[unit.ID] {
			excluded = append(excluded, PlaywrightExclusion{ID: unit.ID, Project: unit.Project, Reason: "NO_DEPENDENCY_PATH_TO_DIRTY_UNIT", Test: unit.Test})
		}
	}
	return excluded
}

func publicPlaywrightProjects(projects []PlaywrightProject) []PlaywrightProject {
	public := append([]PlaywrightProject(nil), projects...)
	for index := range public {
		public[index].matchers = nil
		public[index].ignores = nil
	}
	return public
}

func playwrightGraphDigest(base string, config PlaywrightConfigIdentity, projects []PlaywrightProject, units []PlaywrightSelection) (string, error) {
	body, err := json.Marshal(struct {
		Base     string                   `json:"base"`
		Config   PlaywrightConfigIdentity `json:"config"`
		Projects []PlaywrightProject      `json:"projects"`
		Units    []PlaywrightSelection    `json:"units"`
	}{Base: base, Config: config, Projects: projects, Units: units}, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "playwright-affected-graph:sha256:" + hex.EncodeToString(sum[:]), nil
}

func (plan PlaywrightPlan) Canonical() ([]byte, error) {
	return json.Marshal(plan, json.Deterministic(true))
}

func parsePlaywrightConfig(configPath, source string) ([]PlaywrightProject, string, []PlaywrightUnknown) {
	clean, err := stripComments(source, false)
	if err != nil {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "config tokenization failed")}
	}
	object, ok := playwrightConfigObject(clean)
	if !ok {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "exported config is not one static object")}
	}
	properties, ok := playwrightObjectProperties(object)
	if !ok {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "top-level config object is dynamic")}
	}
	globalTestDir := "."
	unknown := []PlaywrightUnknown{}
	if raw, exists := properties["testDir"]; exists {
		value, literal := playwrightString(raw)
		if !literal || !playwrightRelativeDirectory(value) {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, "global testDir is not one static relative path"))
		} else {
			globalTestDir = path.Clean(path.Join(path.Dir(configPath), value))
		}
	}
	for _, member := range []string{"grep", "grepInvert", "metadata", "testIgnore", "testMatch", "tsconfig"} {
		if _, exists := properties[member]; exists {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, "global "+member+" is outside the static project subset"))
		}
	}
	rawProjects, exists := properties["projects"]
	if !exists {
		return nil, globalTestDir, append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "projects is absent"))
	}
	items, ok := playwrightArrayItems(rawProjects)
	if !ok || len(items) == 0 {
		return nil, globalTestDir, append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "projects is not one nonempty static array"))
	}
	projects := make([]PlaywrightProject, 0, len(items))
	names := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		project, itemUnknown := parsePlaywrightProject(configPath, globalTestDir, item, properties["use"])
		unknown = append(unknown, itemUnknown...)
		if project.Name == "" {
			continue
		}
		if names[project.Name] {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "duplicate project name "+project.Name))
			continue
		}
		names[project.Name] = true
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	for _, project := range projects {
		for _, dependency := range append(append([]string{}, project.Dependencies...), project.Teardown) {
			if dependency != "" && !names[dependency] {
				unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, project.Name+" references unknown project "+dependency))
			}
		}
	}
	if playwrightProjectCycle(projects) {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, "project dependency or teardown graph has a cycle"))
	}
	if len(projects) == 0 {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "no static project identity was recovered"))
	}
	return projects, globalTestDir, canonicalPlaywrightUnknowns(unknown)
}

func parsePlaywrightProject(configPath, globalTestDir, raw, globalUse string) (PlaywrightProject, []PlaywrightUnknown) {
	properties, ok := playwrightObjectProperties(raw)
	if !ok {
		return PlaywrightProject{}, []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownProjectSet, "project entry is not a static object")}
	}
	name, ok := playwrightString(properties["name"])
	if !ok || name == "" {
		return PlaywrightProject{}, []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownProjectSet, "project name is not one nonempty string")}
	}
	project := PlaywrightProject{Name: name, TestDir: globalTestDir, TestIgnore: []string{}, TestMatch: []string{}, Dependencies: []string{}}
	project.Argv = []string{"npx", "playwright", "test", "--config=" + configPath, "--project=" + name}
	fragmentSum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	if globalUse != "" {
		fragmentSum = sha256.Sum256([]byte(strings.TrimSpace(globalUse) + "\n" + strings.TrimSpace(raw)))
	}
	project.ConfigFragment = hex.EncodeToString(fragmentSum[:])
	unknown := []PlaywrightUnknown{}
	if value, exists := properties["testDir"]; exists {
		directory, literal := playwrightString(value)
		if !literal || !playwrightRelativeDirectory(directory) {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, name+" testDir is dynamic"))
		} else {
			project.TestDir = path.Clean(path.Join(path.Dir(configPath), directory))
		}
	}
	project.TestMatch, project.matchers, unknown = parsePlaywrightMatchers(name, "testMatch", properties, unknown)
	project.TestIgnore, project.ignores, unknown = parsePlaywrightMatchers(name, "testIgnore", properties, unknown)
	if value, exists := properties["dependencies"]; exists {
		dependencies, literal := playwrightStringArray(value)
		if !literal {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, name+" dependencies are dynamic"))
		} else {
			project.Dependencies = sortedUnique(dependencies)
		}
	}
	if value, exists := properties["teardown"]; exists {
		teardown, literal := playwrightString(value)
		if !literal || teardown == "" {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, name+" teardown is dynamic"))
		} else {
			project.Teardown = teardown
		}
	}
	project.Grep, unknown = playwrightStaticIdentity(name, "grep", properties, unknown)
	project.GrepInvert, unknown = playwrightStaticIdentity(name, "grepInvert", properties, unknown)
	if metadata, exists := properties["metadata"]; exists {
		if !playwrightStaticValue(metadata) {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, name+" metadata is dynamic"))
		} else {
			sum := sha256.Sum256([]byte(strings.TrimSpace(metadata)))
			project.Metadata = hex.EncodeToString(sum[:])
		}
	}
	project.Browser, project.Device, ok = playwrightInheritedUseIdentity(globalUse, properties["use"])
	if !ok {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownBrowserIdentity, name+" use.browserName/device is dynamic or unsupported"))
	}
	return project, unknown
}

func parsePlaywrightMatchers(project, member string, properties map[string]string, unknown []PlaywrightUnknown) ([]string, []playwrightMatcher, []PlaywrightUnknown) {
	raw, exists := properties[member]
	if !exists {
		return []string{}, nil, unknown
	}
	items, array := playwrightArrayItems(raw)
	if !array {
		items = []string{raw}
	}
	values := []string{}
	matchers := []playwrightMatcher{}
	for _, item := range items {
		matcher, ok := compilePlaywrightMatcher(item)
		if !ok {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, project+" "+member+" contains an unsupported matcher"))
			continue
		}
		values = append(values, matcher.raw)
		matchers = append(matchers, matcher)
	}
	sort.Strings(values)
	sort.Slice(matchers, func(i, j int) bool { return matchers[i].raw < matchers[j].raw })
	return values, matchers, unknown
}

func playwrightStaticIdentity(project, member string, properties map[string]string, unknown []PlaywrightUnknown) (string, []PlaywrightUnknown) {
	raw, exists := properties[member]
	if !exists {
		return "", unknown
	}
	if _, ok := compilePlaywrightMatcher(raw); !ok {
		return "", append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, project+" "+member+" is dynamic"))
	}
	return strings.TrimSpace(raw), unknown
}

func playwrightUseIdentity(raw string) (browser, device string, ok bool) {
	return playwrightInheritedUseIdentity("", raw)
}

func playwrightInheritedUseIdentity(global, raw string) (browser, device string, ok bool) {
	browser, device, ok = playwrightUseLayer(global, "", "")
	if !ok {
		return "", "", false
	}
	browser, device, ok = playwrightUseLayer(raw, browser, device)
	if browser == "" && device != "" {
		browser = playwrightDeviceBrowser(device)
	}
	if browser == "" {
		browser = "chromium"
	}
	return browser, device, ok
}

func playwrightUseLayer(raw, browser, device string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return browser, device, true
	}
	object, exact := playwrightObject(raw)
	if !exact {
		return "", "", false
	}
	items, split := playwrightSplitTopLevel(object[1 : len(object)-1])
	if !split {
		return "", "", false
	}
	seenBrowser := false
	seenDevice := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "...") {
			name, literal := playwrightDeviceSpread(strings.TrimSpace(strings.TrimPrefix(item, "...")))
			if !literal || seenDevice {
				return "", "", false
			}
			device = name
			seenDevice = true
			if playwrightDeviceBrowser(device) == "" {
				return "", device, false
			}
			continue
		}
		colon := playwrightTopLevelColon(item)
		if colon < 0 {
			return "", device, false
		}
		key := strings.TrimSpace(item[:colon])
		if quoted, literal := playwrightString(key); literal {
			key = quoted
		} else {
			for index, character := range key {
				if character != '_' && character != '$' && !unicode.IsLetter(character) && (index == 0 || !unicode.IsDigit(character)) {
					return "", device, false
				}
			}
			if key == "" {
				return "", device, false
			}
		}
		if key == "defaultBrowserType" {
			return "", device, false
		}
		if key != "browserName" {
			if !playwrightStaticValue(strings.TrimSpace(item[colon+1:])) {
				return "", device, false
			}
			continue
		}
		value, literal := playwrightString(strings.TrimSpace(item[colon+1:]))
		if !literal || seenBrowser || value != "chromium" && value != "firefox" && value != "webkit" {
			return "", device, false
		}
		browser, seenBrowser = value, true
	}
	return browser, device, true
}

func playwrightDeviceSpread(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "devices[") || !strings.HasSuffix(raw, "]") {
		return "", false
	}
	return playwrightString(strings.TrimSpace(raw[len("devices[") : len(raw)-1]))
}

func playwrightDeviceBrowser(device string) string {
	switch device {
	case "Desktop Firefox":
		return "firefox"
	case "Desktop Safari":
		return "webkit"
	case "Desktop Chrome":
		return "chromium"
	}
	return ""
}

func playwrightProjectOwns(root string, project PlaywrightProject, globalTestDir, test string) bool {
	directory := project.TestDir
	if directory == "" {
		directory = globalTestDir
	}
	if directory != "." && !inside(test, directory) {
		return false
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(test, directory), "/")
	absolute := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(test)))
	if len(project.matchers) != 0 {
		if !playwrightAnyMatcher(project.matchers, absolute) {
			return false
		}
	} else if !playwrightDefaultTest(relative) {
		return false
	}
	return !playwrightAnyMatcher(project.ignores, absolute)
}

func playwrightDefaultTest(relative string) bool {
	base := path.Base(relative)
	for _, marker := range []string{".spec.", ".test."} {
		if !strings.Contains(base, marker) {
			continue
		}
		for _, extension := range sourceExtensions {
			if strings.HasSuffix(base, extension) {
				return true
			}
		}
	}
	return false
}

func playwrightAnyMatcher(matchers []playwrightMatcher, values ...string) bool {
	for _, matcher := range matchers {
		for _, value := range values {
			if matcher.re.MatchString(value) {
				return true
			}
		}
	}
	return false
}

func compilePlaywrightMatcher(raw string) (playwrightMatcher, bool) {
	raw = strings.TrimSpace(raw)
	if value, ok := playwrightString(raw); ok {
		pattern, valid := playwrightGlobPattern(value)
		if !valid {
			return playwrightMatcher{}, false
		}
		re, err := regexp.Compile("^(?:" + pattern + ")$")
		return playwrightMatcher{raw: raw, re: re}, err == nil
	}
	pattern, flags, ok := playwrightRegexLiteral(raw)
	if !ok || strings.Contains(pattern, "(?") || strings.Contains(pattern, "\\k<") {
		return playwrightMatcher{}, false
	}
	prefix := ""
	for _, flag := range flags {
		switch flag {
		case 'i':
			prefix += "(?i)"
		case 'm':
			prefix += "(?m)"
		case 's':
			prefix += "(?s)"
		case 'g', 'u', 'y':
		default:
			return playwrightMatcher{}, false
		}
	}
	re, err := regexp.Compile(prefix + pattern)
	return playwrightMatcher{raw: raw, re: re}, err == nil
}

func playwrightGlobPattern(glob string) (string, bool) {
	var pattern strings.Builder
	for index := 0; index < len(glob); {
		switch glob[index] {
		case '*':
			if index+1 < len(glob) && glob[index+1] == '*' {
				if index+2 < len(glob) && glob[index+2] == '/' {
					pattern.WriteString("(?:.*/)?")
					index += 3
				} else {
					pattern.WriteString(".*")
					index += 2
				}
			} else {
				pattern.WriteString("[^/]*")
				index++
			}
		case '?':
			pattern.WriteString("[^/]")
			index++
		case '{':
			end := strings.IndexByte(glob[index+1:], '}')
			if end < 0 {
				return "", false
			}
			parts := strings.Split(glob[index+1:index+1+end], ",")
			pattern.WriteString("(?:")
			for partIndex, part := range parts {
				if part == "" || strings.ContainsAny(part, "*?{}[]") {
					return "", false
				}
				if partIndex != 0 {
					pattern.WriteByte('|')
				}
				pattern.WriteString(regexp.QuoteMeta(part))
			}
			pattern.WriteByte(')')
			index += end + 2
		case '[', ']':
			return "", false
		default:
			start := index
			for index < len(glob) && !strings.ContainsRune("*?{[]", rune(glob[index])) {
				index++
			}
			pattern.WriteString(regexp.QuoteMeta(glob[start:index]))
		}
	}
	return pattern.String(), true
}

func playwrightRegexLiteral(raw string) (pattern, flags string, ok bool) {
	if len(raw) < 2 || raw[0] != '/' {
		return "", "", false
	}
	escaped := false
	class := false
	for index := 1; index < len(raw); index++ {
		switch raw[index] {
		case '\\':
			escaped = !escaped
		case '[':
			if !escaped {
				class = true
			}
			escaped = false
		case ']':
			if !escaped {
				class = false
			}
			escaped = false
		case '/':
			if !escaped && !class {
				return raw[1:index], raw[index+1:], true
			}
			escaped = false
		default:
			escaped = false
		}
	}
	return "", "", false
}

func playwrightConfigObject(source string) (string, bool) {
	export := regexp.MustCompile(`(?m)^[\t ]*export[\t ]+default\b`).FindStringIndex(source)
	if export == nil {
		return "", false
	}
	rest := strings.TrimSpace(source[export[1]:])
	if strings.HasPrefix(rest, "defineConfig") && len(rest) > len("defineConfig") && (rest[len("defineConfig")] == '(' || unicode.IsSpace(rune(rest[len("defineConfig")]))) {
		rest = rest[len("defineConfig"):]
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			return "", false
		}
		value, next, ok := playwrightBalancedValue(rest, open+1, ')')
		if !ok {
			return "", false
		}
		if trailing := strings.TrimSpace(rest[next:]); trailing != "" && trailing != ";" {
			return "", false
		}
		return playwrightObject(value)
	}
	return playwrightObject(rest)
}

func playwrightObject(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] != '{' {
		return "", false
	}
	value, next, ok := playwrightBalancedValue(raw, 1, '}')
	if !ok {
		return "", false
	}
	if trailing := strings.TrimSpace(raw[next:]); trailing != "" && trailing != ";" {
		return "", false
	}
	return "{" + value + "}", true
}

func playwrightObjectProperties(raw string) (map[string]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, false
	}
	body := raw[1 : len(raw)-1]
	items, ok := playwrightSplitTopLevel(body)
	if !ok {
		return nil, false
	}
	properties := map[string]string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "...") {
			return nil, false
		}
		colon := playwrightTopLevelColon(item)
		if colon < 0 {
			return nil, false
		}
		keyRaw := strings.TrimSpace(item[:colon])
		key, literal := playwrightString(keyRaw)
		if !literal {
			key = keyRaw
			for _, character := range key {
				if character != '_' && character != '$' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
					return nil, false
				}
			}
		}
		if key == "" || strings.TrimSpace(item[colon+1:]) == "" {
			return nil, false
		}
		if _, duplicate := properties[key]; duplicate {
			return nil, false
		}
		properties[key] = strings.TrimSpace(item[colon+1:])
	}
	return properties, true
}

func playwrightArrayItems(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, false
	}
	return playwrightSplitTopLevel(raw[1 : len(raw)-1])
}

func playwrightSplitTopLevel(raw string) ([]string, bool) {
	items := []string{}
	start := 0
	stack := []byte{}
	quote := byte(0)
	escaped := false
	regex := false
	class := false
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if regex {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '[' {
				class = true
			} else if character == ']' {
				class = false
			} else if character == '/' && !class {
				regex = false
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
		case '/':
			regex = playwrightSlashStartsRegex(raw, index)
		case '{', '[', '(':
			stack = append(stack, character)
		case '}', ']', ')':
			if len(stack) == 0 || !playwrightPair(stack[len(stack)-1], character) {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				items = append(items, strings.TrimSpace(raw[start:index]))
				start = index + 1
			}
		}
	}
	if quote != 0 || regex || len(stack) != 0 {
		return nil, false
	}
	items = append(items, strings.TrimSpace(raw[start:]))
	return items, true
}

func playwrightTopLevelColon(raw string) int {
	depth := 0
	quote := byte(0)
	escaped := false
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case ':':
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func playwrightBalancedValue(raw string, start int, closer byte) (string, int, bool) {
	depth := 0
	quote := byte(0)
	escaped := false
	for index := start; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth == 0 && character == closer {
				return strings.TrimSpace(raw[start:index]), index + 1, true
			}
			depth--
		}
	}
	return "", len(raw), false
}

func playwrightPair(open, close byte) bool {
	return open == '{' && close == '}' || open == '[' && close == ']' || open == '(' && close == ')'
}

func playwrightSlashStartsRegex(raw string, index int) bool {
	for previous := index - 1; previous >= 0; previous-- {
		if raw[previous] == ' ' || raw[previous] == '\t' || raw[previous] == '\n' || raw[previous] == '\r' {
			continue
		}
		return strings.ContainsRune("([{:;,=!?&|", rune(raw[previous]))
	}
	return true
}

func playwrightString(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != raw[len(raw)-1] || raw[0] != '\'' && raw[0] != '"' {
		return "", false
	}
	if raw[0] == '"' {
		value, err := strconv.Unquote(raw)
		return value, err == nil
	}
	body := raw[1 : len(raw)-1]
	escaped := false
	for _, character := range body {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '\'' {
			return "", false
		}
	}
	if escaped {
		return "", false
	}
	body = strings.ReplaceAll(body, `\'`, `'`)
	if strings.Contains(body, "\\") {
		quoted := `"` + strings.ReplaceAll(body, `"`, `\"`) + `"`
		value, err := strconv.Unquote(quoted)
		return value, err == nil
	}
	return body, true
}

func playwrightStringArray(raw string) ([]string, bool) {
	items, ok := playwrightArrayItems(raw)
	if !ok {
		return nil, false
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, literal := playwrightString(item)
		if !literal || value == "" {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func playwrightStaticValue(raw string) bool {
	raw = strings.TrimSpace(raw)
	if _, ok := playwrightString(raw); ok {
		return true
	}
	if raw == "true" || raw == "false" || raw == "null" {
		return true
	}
	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return true
	}
	if items, ok := playwrightArrayItems(raw); ok {
		for _, item := range items {
			if !playwrightStaticValue(item) {
				return false
			}
		}
		return true
	}
	if properties, ok := playwrightObjectProperties(raw); ok {
		for _, value := range properties {
			if !playwrightStaticValue(value) {
				return false
			}
		}
		return true
	}
	return false
}

func playwrightProjectCycle(projects []PlaywrightProject) bool {
	edges := map[string][]string{}
	for _, project := range projects {
		edges[project.Name] = append(append([]string{}, project.Dependencies...), project.Teardown)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(name string) bool {
		if visiting[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visiting[name] = true
		for _, next := range edges[name] {
			if visit(next) {
				return true
			}
		}
		visiting[name] = false
		visited[name] = true
		return false
	}
	for name := range edges {
		if visit(name) {
			return true
		}
	}
	return false
}

func selectionUnknown(reason, detail string) PlaywrightUnknown {
	return PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: reason, Detail: detail}
}

func canonicalPlaywrightUnknowns(values []PlaywrightUnknown) []PlaywrightUnknown {
	seen := map[string]bool{}
	result := []PlaywrightUnknown{}
	for _, value := range values {
		key := value.Axis + "\x00" + value.Reason + "\x00" + value.Detail
		if !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Axis != result[j].Axis {
			return result[i].Axis < result[j].Axis
		}
		if result[i].Reason != result[j].Reason {
			return result[i].Reason < result[j].Reason
		}
		return result[i].Detail < result[j].Detail
	})
	return result
}

func sortedPlaywrightSelections(values map[string]PlaywrightSelection) []PlaywrightSelection {
	result := make([]PlaywrightSelection, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func playwrightIDEscape(value string) string {
	const hexDigits = "0123456789ABCDEF"
	var result strings.Builder
	for _, raw := range []byte(value) {
		if raw >= 'a' && raw <= 'z' || raw >= 'A' && raw <= 'Z' || raw >= '0' && raw <= '9' || strings.ContainsRune("-_.", rune(raw)) {
			result.WriteByte(raw)
			continue
		}
		result.WriteByte('%')
		result.WriteByte(hexDigits[raw>>4])
		result.WriteByte(hexDigits[raw&15])
	}
	return result.String()
}

func playwrightRelativeDirectory(value string) bool {
	clean := path.Clean(value)
	return value != "" && !strings.HasPrefix(clean, "../") && clean != ".." && !strings.HasPrefix(clean, "/")
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return sortedKeys(set)
}

func containsString(values []string, want string) bool {
	index := sort.SearchStrings(values, want)
	return index < len(values) && values[index] == want
}
